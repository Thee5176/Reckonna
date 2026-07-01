package command

// ledger_repo.go — hand-written thin wrappers over the sqlc-generated Queries
// that persist a validated domain.JournalEntry. It is mechanical: no business
// rules live here (the domain already validated balance/currency/dimensions;
// the deferred DB trigger re-checks balance at COMMIT). The service owns the tx
// and passes a tx-bound *Queries, so a mid-step error rolls the whole entry back.

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/thee5176/reckonna/internal/domain"
)

// ErrUnknownDimension is returned when a line references a dimension value that
// does not exist. The service maps it to a 422 (unknown/invalid dimension).
var ErrUnknownDimension = errors.New("command: unknown dimension value")

// sideToDB maps a domain Side to the DB enum.
func sideToDB(s domain.Side) EntrySide {
	if s == domain.SideCredit {
		return EntrySideCredit
	}
	return EntrySideDebit
}

// PersistEntry writes the entry header then its lines + dimensions. accByCode
// maps every referenced account code to its id (resolved by the service before
// the tx). Returns the new entry version (1 on create). Balance is enforced at
// COMMIT by the deferred trigger.
func PersistEntry(ctx context.Context, q *Queries, e *domain.JournalEntry, bookID uuid.UUID, accByCode map[int]uuid.UUID) (int32, error) {
	row, err := q.InsertJournalEntry(ctx, InsertJournalEntryParams{
		ID:          e.ID,
		EntryDate:   e.Date,
		Description: e.Description,
		OwnerSub:    e.Owner,
		BookID:      bookID,
	})
	if err != nil {
		return 0, fmt.Errorf("insert journal_entry: %w", err)
	}
	if err := PersistLines(ctx, q, e, accByCode); err != nil {
		return 0, err
	}
	return row.Version, nil
}

// PersistLines writes an entry's lines + dimension values against an already
// existing entry header (used by both create and the replace-on-update path).
func PersistLines(ctx context.Context, q *Queries, e *domain.JournalEntry, accByCode map[int]uuid.UUID) error {
	for i, l := range e.Lines {
		lineID, err := uuid.NewV7()
		if err != nil {
			return fmt.Errorf("new line id: %w", err)
		}
		if err := q.InsertJournalLine(ctx, InsertJournalLineParams{
			ID:             lineID,
			JournalEntryID: e.ID,
			AccountID:      accByCode[l.Account.Code],
			Side:           sideToDB(l.Side),
			Amount:         l.Amount.Decimal(),
			LineNo:         int32(i + 1),
		}); err != nil {
			return fmt.Errorf("insert journal_line %d: %w", i, err)
		}

		for dt, dv := range l.Dimensions {
			res, err := q.GetDimensionValue(ctx, GetDimensionValueParams{
				TypeCode:  string(dt),
				ValueCode: dv,
			})
			if err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return fmt.Errorf("%w: %s=%s", ErrUnknownDimension, dt, dv)
				}
				return fmt.Errorf("resolve dimension %s=%s: %w", dt, dv, err)
			}
			if err := q.InsertJournalLineDimension(ctx, InsertJournalLineDimensionParams{
				JournalLineID:    lineID,
				DimensionTypeID:  res.TypeID,
				DimensionValueID: res.ValueID,
			}); err != nil {
				return fmt.Errorf("insert journal_line_dimension: %w", err)
			}
		}
	}
	return nil
}
