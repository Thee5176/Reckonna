package domain_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thee5176/reckonna/internal/domain"
)

// dec is a test helper that parses a decimal string and fails the test on
// malformed input. Centralizes the parse-error handling so each case row stays
// a one-liner.
func dec(t *testing.T, s string) decimal.Decimal {
	t.Helper()
	d, err := decimal.NewFromString(s)
	require.NoErrorf(t, err, "bad decimal literal %q", s)
	return d
}

func TestNewLedger_BalanceInvariant(t *testing.T) {
	cash := uuid.New()
	revenue := uuid.New()
	receivable := uuid.New()

	tests := []struct {
		name    string
		items   func(t *testing.T) []domain.LedgerItem
		wantErr error
	}{
		{
			name: "balanced single pair",
			items: func(t *testing.T) []domain.LedgerItem {
				return []domain.LedgerItem{
					{Account: cash, Side: domain.SideDebit, Amount: dec(t, "100.00")},
					{Account: revenue, Side: domain.SideCredit, Amount: dec(t, "100.00")},
				}
			},
			wantErr: nil,
		},
		{
			name: "balanced split debits",
			items: func(t *testing.T) []domain.LedgerItem {
				return []domain.LedgerItem{
					{Account: cash, Side: domain.SideDebit, Amount: dec(t, "30.00")},
					{Account: receivable, Side: domain.SideDebit, Amount: dec(t, "70.00")},
					{Account: revenue, Side: domain.SideCredit, Amount: dec(t, "100.00")},
				}
			},
			wantErr: nil,
		},
		{
			name: "unbalanced debit exceeds credit",
			items: func(t *testing.T) []domain.LedgerItem {
				return []domain.LedgerItem{
					{Account: cash, Side: domain.SideDebit, Amount: dec(t, "150.00")},
					{Account: revenue, Side: domain.SideCredit, Amount: dec(t, "100.00")},
				}
			},
			wantErr: domain.ErrUnbalanced,
		},
		{
			// Sub-cent imbalance — guards against any future float64 regression:
			// float64 cannot represent 0.01 exactly, so a float-based sum would
			// round this case to zero and the assertion would silently pass.
			name: "unbalanced by sub-cent rejects",
			items: func(t *testing.T) []domain.LedgerItem {
				return []domain.LedgerItem{
					{Account: cash, Side: domain.SideDebit, Amount: dec(t, "100.01")},
					{Account: revenue, Side: domain.SideCredit, Amount: dec(t, "100.00")},
				}
			},
			wantErr: domain.ErrUnbalanced,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			items := tt.items(t)

			// Act
			ledger, err := domain.NewLedger(items)

			// Assert
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.Nil(t, ledger)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, ledger)
		})
	}
}

// TestLedgerItem_Amount_IsDecimal_NotFloat asserts the money path uses
// arbitrary-precision decimal, not float64. A precision of 20 fractional
// digits exceeds float64's ~15-17 significant-digit limit, so a future
// refactor that swaps decimal.Decimal for float64 would lose the trailing
// digit and fail this round-trip.
func TestLedgerItem_Amount_IsDecimal_NotFloat(t *testing.T) {
	// Arrange
	const literal = "0.10000000000000000001"
	want := dec(t, literal)
	item := domain.LedgerItem{
		Account: uuid.New(),
		Side:    domain.SideDebit,
		Amount:  want,
	}

	// Act
	got := item.Amount

	// Assert — exact string round-trip; float64 cannot preserve this.
	assert.Equal(t, literal, got.String(),
		"Amount must preserve full decimal precision (no float64 truncation)")
	assert.True(t, got.Equal(want), "Amount must equal the constructed decimal")
}

func TestSide_DebitAndCreditAreDistinct(t *testing.T) {
	// Distinct enum values — guards against a future refactor that aliases
	// both sides to the same constant (which would let any ledger "balance").
	assert.NotEqual(t, domain.SideDebit, domain.SideCredit)
}
