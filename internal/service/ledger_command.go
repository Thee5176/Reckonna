package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/thee5176/reckonna/internal/domain"
	"github.com/thee5176/reckonna/internal/repository/command"
)

// Command-side use-case errors. The handler maps each to an RFC 7807 status.
var (
	ErrUnknownAccountCode = errors.New("service: unknown account code")
	ErrBookNotFound       = errors.New("service: book not found")
	ErrEntryNotFound      = errors.New("service: journal entry not found")
	ErrForbidden          = errors.New("service: not the owner")
)

// VersionConflictError signals an optimistic-concurrency failure and carries the
// current persisted version so the handler can return it in the 409 body.
type VersionConflictError struct{ Current int32 }

func (e *VersionConflictError) Error() string {
	return fmt.Sprintf("service: version conflict (current=%d)", e.Current)
}

// LineInput is one posting as parsed from the request DTO (business codes, not ids).
type LineInput struct {
	AccountCode int
	Side        domain.Side
	Amount      domain.Money
	Dimensions  map[domain.DimensionType]string
}

// EntryInput is the create/update payload after DTO parsing. Owner comes from
// the authenticated JWT sub, never from the body.
type EntryInput struct {
	Date        time.Time
	Description string
	Owner       string
	Book        string
	Lines       []LineInput
}

// LedgerCommandService creates, updates, and deletes journal entries. It owns
// the write transaction; repositories operate on the tx-bound Queries.
type LedgerCommandService struct {
	pool *pgxpool.Pool
	q    *command.Queries // non-tx queries for pre-tx reference reads
}

// NewLedgerCommandService builds the service over a pgx pool.
func NewLedgerCommandService(pool *pgxpool.Pool) *LedgerCommandService {
	return &LedgerCommandService{pool: pool, q: command.New(pool)}
}

// buildEntry resolves accounts (validating each code exists), constructs the
// domain entry (which enforces balance/currency/required-dimension), and returns
// it plus the account-id map for persistence.
func (s *LedgerCommandService) buildEntry(ctx context.Context, id uuid.UUID, in EntryInput) (*domain.JournalEntry, map[int]uuid.UUID, error) {
	accByCode := make(map[int]uuid.UUID, len(in.Lines))
	accMeta := make(map[int]command.GetAccountByCodeRow, len(in.Lines))
	lines := make([]domain.JournalLine, 0, len(in.Lines))

	for _, li := range in.Lines {
		meta, ok := accMeta[li.AccountCode]
		if !ok {
			row, err := s.q.GetAccountByCode(ctx, int32(li.AccountCode))
			if err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return nil, nil, fmt.Errorf("%w: %d", ErrUnknownAccountCode, li.AccountCode)
				}
				return nil, nil, fmt.Errorf("resolve account %d: %w", li.AccountCode, err)
			}
			meta = row
			accMeta[li.AccountCode] = row
			accByCode[li.AccountCode] = row.ID
		}

		req := make([]domain.DimensionType, len(meta.RequiredDimensions))
		for i, d := range meta.RequiredDimensions {
			req[i] = domain.DimensionType(d)
		}
		lines = append(lines, domain.JournalLine{
			Account: domain.Account{
				Code:               int(meta.Code),
				Type:               domain.AccountType(meta.Type),
				NormalBalance:      domain.NormalBalance(meta.NormalBalance),
				Postable:           meta.Postable,
				RequiredDimensions: req,
			},
			Side:       li.Side,
			Amount:     li.Amount,
			Dimensions: li.Dimensions,
		})
	}

	entry, err := domain.NewEntry(id, in.Date, in.Description, in.Owner, in.Book, lines)
	if err != nil {
		return nil, nil, err
	}
	return entry, accByCode, nil
}

// PostLedger validates and atomically persists a new journal entry, returning
// its id and version (1 on create). (S7, AT1/AT2/AT10/AT13/AT14, IT1)
func (s *LedgerCommandService) PostLedger(ctx context.Context, in EntryInput) (uuid.UUID, int32, error) {
	book, err := s.q.GetBookByCode(ctx, in.Book)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, 0, fmt.Errorf("%w: %s", ErrBookNotFound, in.Book)
		}
		return uuid.Nil, 0, fmt.Errorf("resolve book: %w", err)
	}

	id, err := uuid.NewV7()
	if err != nil {
		return uuid.Nil, 0, fmt.Errorf("new entry id: %w", err)
	}
	entry, accByCode, err := s.buildEntry(ctx, id, in)
	if err != nil {
		return uuid.Nil, 0, err
	}

	var version int32
	err = WithinTx(ctx, s.pool, func(q *command.Queries) error {
		v, perr := command.PersistEntry(ctx, q, entry, book.ID, accByCode)
		version = v
		return perr
	})
	if err != nil {
		return uuid.Nil, 0, err
	}
	return id, version, nil
}

// UpdateLedger replaces an entry's header + lines under optimistic concurrency.
// Ownership is enforced (cross-owner → ErrForbidden / 403 per AT4). (S9, AT16)
func (s *LedgerCommandService) UpdateLedger(ctx context.Context, id uuid.UUID, expectedVersion int32, in EntryInput) (int32, error) {
	cur, err := s.q.GetJournalEntryForUpdate(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, ErrEntryNotFound
		}
		return 0, fmt.Errorf("load entry: %w", err)
	}
	if cur.OwnerSub != in.Owner {
		return 0, ErrForbidden
	}
	if cur.Version != expectedVersion {
		return 0, &VersionConflictError{Current: cur.Version}
	}

	entry, accByCode, err := s.buildEntry(ctx, id, in)
	if err != nil {
		return 0, err
	}

	var newVersion int32
	err = WithinTx(ctx, s.pool, func(q *command.Queries) error {
		v, uerr := q.UpdateJournalEntryHeader(ctx, command.UpdateJournalEntryHeaderParams{
			ID:          id,
			EntryDate:   entry.Date,
			Description: entry.Description,
			Version:     expectedVersion,
		})
		if uerr != nil {
			if errors.Is(uerr, pgx.ErrNoRows) {
				// version changed between our read and the UPDATE
				return &VersionConflictError{Current: cur.Version}
			}
			return fmt.Errorf("update header: %w", uerr)
		}
		newVersion = v
		if derr := q.DeleteJournalLinesByEntry(ctx, id); derr != nil {
			return fmt.Errorf("clear lines: %w", derr)
		}
		return command.PersistLines(ctx, q, entry, accByCode)
	})
	if err != nil {
		return 0, err
	}
	return newVersion, nil
}

// DeleteLedger removes an entry and all its lines (cascade). Ownership enforced.
// (S9, AT5)
func (s *LedgerCommandService) DeleteLedger(ctx context.Context, id uuid.UUID, owner string) error {
	cur, err := s.q.GetJournalEntryForUpdate(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrEntryNotFound
		}
		return fmt.Errorf("load entry: %w", err)
	}
	if cur.OwnerSub != owner {
		return ErrForbidden
	}
	return WithinTx(ctx, s.pool, func(q *command.Queries) error {
		return q.DeleteJournalEntry(ctx, command.DeleteJournalEntryParams{ID: id, OwnerSub: owner})
	})
}
