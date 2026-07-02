// Package domain holds the accounting core: entities, value objects, and
// invariants. It has no dependencies on infrastructure (DB, HTTP, OTel) so the
// rules can be exercised in plain unit tests.
//
// Naming follows the CoA governance standard §7: a JournalEntry is the header
// (date, description, owner, book); a JournalLine is one posting (account,
// amount, side, dimensions incl. currency). These replace the earlier
// Ledger / LedgerItem names.
package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// Side names which column of a double-entry posting a JournalLine hits.
type Side uint8

const (
	SideDebit Side = iota + 1
	SideCredit
)

// Domain invariant errors. Callers use errors.Is; handlers map each to an
// RFC 7807 code (see internal/handler/errors.go).
var (
	// ErrUnbalanced is returned when 借方 (debit) != 貸方 (credit).
	ErrUnbalanced = errors.New("domain: entry unbalanced (借方 != 貸方)")
	// ErrMixedCurrency is returned when lines disagree on their currency
	// dimension. A v1 entry must be single-currency.
	ErrMixedCurrency = errors.New("domain: entry mixes currencies (v1 entries are single-currency)")
	// ErrRequiredDimension is returned when a line omits a dimension its
	// account declares as required (standard §7 R7.4).
	ErrRequiredDimension = errors.New("domain: line missing a required dimension")
	// ErrNoLines is returned when an entry has no postings.
	ErrNoLines = errors.New("domain: entry has no lines")
)

// JournalLine is one posting: an Amount on a Side, against an Account, carrying
// dimension values (including currency).
type JournalLine struct {
	Account    Account
	Side       Side
	Amount     Money
	Dimensions map[DimensionType]string
}

// Currency returns the line's currency dimension value (empty if unset).
func (l JournalLine) Currency() string { return l.Dimensions[DimCurrency] }

// JournalEntry is a balanced aggregate of postings within a single book. The
// only way to construct a valid entry is NewEntry, which enforces every
// invariant.
type JournalEntry struct {
	ID          uuid.UUID
	Date        time.Time
	Description string
	Owner       string
	Book        string
	Lines       []JournalLine
}

// NewEntry constructs a JournalEntry after verifying, in order:
//   - at least one line (ErrNoLines),
//   - every account's required dimensions are present (ErrRequiredDimension),
//   - all lines share one currency (ErrMixedCurrency),
//   - 借方 == 貸方 (ErrUnbalanced).
//
// Sums use domain.Money (decimal) — no float arithmetic — so sub-cent
// imbalances are detected exactly.
func NewEntry(id uuid.UUID, date time.Time, description, owner, book string, lines []JournalLine) (*JournalEntry, error) {
	if len(lines) == 0 {
		return nil, ErrNoLines
	}

	for _, l := range lines {
		for _, req := range l.Account.RequiredDimensions {
			if v, ok := l.Dimensions[req]; !ok || v == "" {
				return nil, ErrRequiredDimension
			}
		}
	}

	currency := lines[0].Currency()
	for _, l := range lines {
		if l.Currency() != currency {
			return nil, ErrMixedCurrency
		}
	}

	var debit, credit Money
	for _, l := range lines {
		switch l.Side {
		case SideDebit:
			debit = debit.Add(l.Amount)
		case SideCredit:
			credit = credit.Add(l.Amount)
		}
	}
	if !debit.Equal(credit) {
		return nil, ErrUnbalanced
	}

	return &JournalEntry{
		ID:          id,
		Date:        date,
		Description: description,
		Owner:       owner,
		Book:        book,
		Lines:       lines,
	}, nil
}
