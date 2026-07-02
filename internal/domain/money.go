package domain

import "github.com/shopspring/decimal"

// Money is an exact monetary amount. It wraps shopspring/decimal.Decimal so no
// money path ever touches float64. Persistence normalizes to the DB policy
// NUMERIC(20,4) at the repository boundary; the domain keeps full precision so
// sub-cent imbalances are detected exactly.
type Money struct {
	amount decimal.Decimal
}

// NewMoney wraps a decimal.Decimal as Money.
func NewMoney(d decimal.Decimal) Money { return Money{amount: d} }

// MoneyFromString parses a decimal literal into Money.
func MoneyFromString(s string) (Money, error) {
	d, err := decimal.NewFromString(s)
	if err != nil {
		return Money{}, err
	}
	return Money{amount: d}, nil
}

// Decimal returns the underlying exact value.
func (m Money) Decimal() decimal.Decimal { return m.amount }

// Add returns the sum of two Money values. Exact — no rounding.
func (m Money) Add(o Money) Money { return Money{amount: m.amount.Add(o.amount)} }

// Equal reports exact equality (scale-insensitive: 100 == 100.00).
func (m Money) Equal(o Money) bool { return m.amount.Equal(o.amount) }

// IsNegative reports whether the amount is below zero.
func (m Money) IsNegative() bool { return m.amount.IsNegative() }

// IsZero reports whether the amount is exactly zero.
func (m Money) IsZero() bool { return m.amount.IsZero() }

// String renders the exact decimal representation.
func (m Money) String() string { return m.amount.String() }
