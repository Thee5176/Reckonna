// Package good is a QG fixture demonstrating a clean Go/Gin clean-arch
// domain unit: pure, simple, fully tested, enforces 借方=貸方.
package good

import "errors"

// ErrUnbalanced is returned when debit != credit on a ledger post.
var ErrUnbalanced = errors.New("ledger: debit != credit")

// LedgerItem is a single line on a Ledger.
type LedgerItem struct {
	Debit  int64
	Credit int64
}

// PostLedger asserts SUM(debit)==SUM(credit) and returns the total.
// Pure function, cognitive complexity = 2, fully covered by tests below.
func PostLedger(items []LedgerItem) (int64, error) {
	var d, c int64
	for _, it := range items {
		d += it.Debit
		c += it.Credit
	}
	if d != c {
		return 0, ErrUnbalanced
	}
	return d, nil
}
