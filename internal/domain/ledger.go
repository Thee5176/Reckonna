// Package domain holds the accounting core: entities, value objects, and
// invariants. It has no dependencies on infrastructure (DB, HTTP, OTel) so
// the rules can be exercised in plain unit tests.
package domain

import (
	"errors"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Side names which column of a double-entry ledger a LedgerItem posts to.
type Side uint8

const (
	SideDebit Side = iota + 1
	SideCredit
)

// ErrUnbalanced is returned by NewLedger when 借方 (debit) != 貸方 (credit).
// Double-entry: every ledger must balance.
var ErrUnbalanced = errors.New("domain: ledger unbalanced (借方 != 貸方)")

// LedgerItem is one posting line: an Amount on a Side, against an Account.
// Amount uses shopspring/decimal so the money path never touches float64.
type LedgerItem struct {
	Account uuid.UUID
	Side    Side
	Amount  decimal.Decimal
}

// Ledger is an aggregate of postings whose debit and credit sums are equal.
// The only way to construct a valid Ledger is through NewLedger, which
// enforces the balance invariant.
type Ledger struct {
	Items []LedgerItem
}

// NewLedger constructs a Ledger after verifying 借方 == 貸方.
// Sums are computed with decimal.Decimal — no float arithmetic — so sub-cent
// imbalances are detected exactly.
func NewLedger(items []LedgerItem) (*Ledger, error) {
	var debit, credit decimal.Decimal
	for _, it := range items {
		switch it.Side {
		case SideDebit:
			debit = debit.Add(it.Amount)
		case SideCredit:
			credit = credit.Add(it.Amount)
		}
	}
	if !debit.Equal(credit) {
		return nil, ErrUnbalanced
	}
	return &Ledger{Items: items}, nil
}
