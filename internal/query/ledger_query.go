// Package query is the read model: owner-scoped reads over the query-side sqlc
// package. It has NO write path — cmd/query imports only this and
// internal/repository/query, giving compile-time CQRS purity (IT9). Not-found
// and not-visible are indistinguishable (ErrNotFound) to prevent enumeration
// (T16).
package query

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	qdb "github.com/thee5176/reckonna/internal/repository/query"
)

// Read-model errors. The handler maps these to HTTP status codes.
var (
	ErrNotFound      = errors.New("query: not found")
	ErrInvalidCursor = errors.New("query: invalid cursor")
	ErrTooManyItems  = errors.New("query: too many items requested")
)

// Pagination + query limits (API Conventions).
const (
	DefaultLimit = 50
	MaxLimit     = 200
	MaxAccounts  = 50
)

// Service serves read-model queries.
type Service struct {
	q *qdb.Queries
}

// NewService builds the read model over a pgx pool.
func NewService(pool *pgxpool.Pool) *Service { return &Service{q: qdb.New(pool)} }

// LineView is one posting in a read response.
type LineView struct {
	AccountCode int    `json:"account_code"`
	Side        string `json:"side"`
	Amount      string `json:"amount"`
	LineNo      int    `json:"line_no"`
}

// EntryView is a journal entry for read responses. Lines are populated by
// GetLedger, omitted by list.
type EntryView struct {
	ID          string     `json:"id"`
	Date        string     `json:"date"`
	Description string     `json:"description"`
	Version     int32      `json:"version"`
	Lines       []LineView `json:"lines,omitempty"`
}

// PageView is a cursor-paginated list response.
type PageView struct {
	Items      []EntryView `json:"items"`
	NextCursor *string     `json:"next_cursor"`
	HasMore    bool        `json:"has_more"`
}

// GetLedger returns an owner-scoped entry with its lines. Version is returned
// separately for the ETag header. Not owned / not existing → ErrNotFound (404).
func (s *Service) GetLedger(ctx context.Context, id uuid.UUID, owner string) (*EntryView, error) {
	row, err := s.q.GetJournalEntry(ctx, qdb.GetJournalEntryParams{ID: id, OwnerSub: owner})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get entry: %w", err)
	}
	lines, err := s.q.GetJournalLines(ctx, qdb.GetJournalLinesParams{EntryID: id, OwnerSub: owner})
	if err != nil {
		return nil, fmt.Errorf("get lines: %w", err)
	}
	view := &EntryView{
		ID:          row.ID.String(),
		Date:        row.EntryDate.Format("2006-01-02"),
		Description: row.Description,
		Version:     row.Version,
		Lines:       make([]LineView, len(lines)),
	}
	for i, l := range lines {
		view.Lines[i] = LineView{
			AccountCode: int(l.AccountCode),
			Side:        string(l.Side),
			Amount:      l.Amount.String(),
			LineNo:      int(l.LineNo),
		}
	}
	return view, nil
}

// ListLedgers returns an owner-scoped page of entries ordered by UUIDv7 id.
// limit is clamped to [1, MaxLimit] (default DefaultLimit). An opaque cursor
// (base64 UUIDv7) resumes after the last returned id.
func (s *Service) ListLedgers(ctx context.Context, owner string, limit int, cursor string) (*PageView, error) {
	if limit <= 0 {
		limit = DefaultLimit
	}
	if limit > MaxLimit {
		limit = MaxLimit
	}
	cur, err := decodeCursor(cursor)
	if err != nil {
		return nil, err
	}

	rows, err := s.q.ListJournalEntries(ctx, qdb.ListJournalEntriesParams{
		OwnerSub:  owner,
		Cursor:    cur,
		PageLimit: int32(limit + 1), // fetch one extra to detect has_more
	})
	if err != nil {
		return nil, fmt.Errorf("list entries: %w", err)
	}

	page := &PageView{Items: []EntryView{}}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	for _, r := range rows {
		page.Items = append(page.Items, EntryView{
			ID:          r.ID.String(),
			Date:        r.EntryDate.Format("2006-01-02"),
			Description: r.Description,
			Version:     r.Version,
		})
	}
	page.HasMore = hasMore
	if hasMore && len(rows) > 0 {
		next := encodeCursor(rows[len(rows)-1].ID)
		page.NextCursor = &next
	}
	return page, nil
}

func encodeCursor(id uuid.UUID) string {
	return base64.RawURLEncoding.EncodeToString(id[:])
}

func decodeCursor(s string) (pgtype.UUID, error) {
	if s == "" {
		return pgtype.UUID{Valid: false}, nil
	}
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil || len(b) != 16 {
		return pgtype.UUID{}, ErrInvalidCursor
	}
	var arr [16]byte
	copy(arr[:], b)
	return pgtype.UUID{Bytes: arr, Valid: true}, nil
}
