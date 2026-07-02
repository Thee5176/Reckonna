package domain_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thee5176/reckonna/internal/domain"
)

// money parses a decimal literal into domain.Money, panicking on malformed
// input. Test literals are compile-time known-good, so RequireFromString (not
// t.Fatal) keeps case rows plain values instead of per-row closures.
func money(s string) domain.Money {
	return domain.NewMoney(decimal.RequireFromString(s))
}

// acct builds a minimal postable account with the given code, normal balance,
// and optional required dimensions.
func acct(code int, nb domain.NormalBalance, req ...domain.DimensionType) domain.Account {
	return domain.Account{
		Code:               code,
		Type:               domain.AccountAsset, // type is irrelevant to balance/currency invariants
		NormalBalance:      nb,
		Postable:           true,
		RequiredDimensions: req,
	}
}

// dims is a convenience constructor for a line's dimension map.
func dims(currency string, extra ...[2]string) map[domain.DimensionType]string {
	m := map[domain.DimensionType]string{domain.DimCurrency: currency}
	for _, kv := range extra {
		m[domain.DimensionType(kv[0])] = kv[1]
	}
	return m
}

func TestNewEntry_BalanceInvariant(t *testing.T) {
	cash := acct(10000, domain.NormalDebit)
	revenue := acct(40000, domain.NormalCredit)
	receivable := acct(11000, domain.NormalDebit)

	tests := []struct {
		name    string
		lines   []domain.JournalLine
		wantErr error
	}{
		{
			name: "balanced single pair",
			lines: []domain.JournalLine{
				{Account: cash, Side: domain.SideDebit, Amount: money("100.00"), Dimensions: dims("JPY")},
				{Account: revenue, Side: domain.SideCredit, Amount: money("100.00"), Dimensions: dims("JPY")},
			},
			wantErr: nil,
		},
		{
			name: "balanced split debits",
			lines: []domain.JournalLine{
				{Account: cash, Side: domain.SideDebit, Amount: money("30.00"), Dimensions: dims("JPY")},
				{Account: receivable, Side: domain.SideDebit, Amount: money("70.00"), Dimensions: dims("JPY")},
				{Account: revenue, Side: domain.SideCredit, Amount: money("100.00"), Dimensions: dims("JPY")},
			},
			wantErr: nil,
		},
		{
			// 借方 (debit) exceeds 貸方 (credit): the double-entry invariant is broken.
			name: "unbalanced debit exceeds credit",
			lines: []domain.JournalLine{
				{Account: cash, Side: domain.SideDebit, Amount: money("150.00"), Dimensions: dims("JPY")},
				{Account: revenue, Side: domain.SideCredit, Amount: money("100.00"), Dimensions: dims("JPY")},
			},
			wantErr: domain.ErrUnbalanced,
		},
		{
			// Sub-cent imbalance — guards against any future float64 regression:
			// float64 cannot represent 0.01 exactly, so a float-based sum would
			// round this case to zero and the assertion would silently pass.
			name: "unbalanced by sub-cent rejects",
			lines: []domain.JournalLine{
				{Account: cash, Side: domain.SideDebit, Amount: money("100.01"), Dimensions: dims("JPY")},
				{Account: revenue, Side: domain.SideCredit, Amount: money("100.00"), Dimensions: dims("JPY")},
			},
			wantErr: domain.ErrUnbalanced,
		},
		{
			name:    "no lines rejected",
			lines:   nil,
			wantErr: domain.ErrNoLines,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			entry, err := domain.NewEntry(uuid.New(), time.Now(), "test", "owner-sub", domain.BookBase, tt.lines)

			// Assert
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.Nil(t, entry)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, entry)
			assert.Equal(t, domain.BookBase, entry.Book)
		})
	}
}

// TestNewEntry_MixedCurrency asserts a v1 entry must be single-currency: lines
// that disagree on their currency dimension are rejected. (AT14)
func TestNewEntry_MixedCurrency(t *testing.T) {
	cash := acct(10000, domain.NormalDebit)
	revenue := acct(40000, domain.NormalCredit)

	lines := []domain.JournalLine{
		{Account: cash, Side: domain.SideDebit, Amount: money("100.00"), Dimensions: dims("USD")},
		{Account: revenue, Side: domain.SideCredit, Amount: money("100.00"), Dimensions: dims("JPY")},
	}

	entry, err := domain.NewEntry(uuid.New(), time.Now(), "mixed", "owner", domain.BookBase, lines)
	require.ErrorIs(t, err, domain.ErrMixedCurrency)
	assert.Nil(t, entry)
}

// TestNewEntry_RequiredDimension asserts an account declaring a required
// dimension (e.g. 21500 Customer escrow payable requires counterparty) rejects
// a line that omits it. (IT12, proves §7 R7.4)
func TestNewEntry_RequiredDimension(t *testing.T) {
	cash := acct(10000, domain.NormalDebit)
	escrow := acct(21500, domain.NormalCredit, domain.DimCounterparty)

	t.Run("missing required counterparty rejected", func(t *testing.T) {
		lines := []domain.JournalLine{
			{Account: cash, Side: domain.SideDebit, Amount: money("100.00"), Dimensions: dims("JPY")},
			{Account: escrow, Side: domain.SideCredit, Amount: money("100.00"), Dimensions: dims("JPY")},
		}
		entry, err := domain.NewEntry(uuid.New(), time.Now(), "escrow", "owner", domain.BookBase, lines)
		require.ErrorIs(t, err, domain.ErrRequiredDimension)
		assert.Nil(t, entry)
	})

	t.Run("present required counterparty accepted", func(t *testing.T) {
		lines := []domain.JournalLine{
			{Account: cash, Side: domain.SideDebit, Amount: money("100.00"), Dimensions: dims("JPY")},
			{Account: escrow, Side: domain.SideCredit, Amount: money("100.00"), Dimensions: dims("JPY", [2]string{"counterparty", "cust-1"})},
		}
		entry, err := domain.NewEntry(uuid.New(), time.Now(), "escrow", "owner", domain.BookBase, lines)
		require.NoError(t, err)
		require.NotNil(t, entry)
	})
}

// TestMoney_IsDecimal_NotFloat asserts the money path uses arbitrary-precision
// decimal, not float64. A precision of 20 fractional digits exceeds float64's
// ~15-17 significant-digit limit, so a refactor to float64 would lose the
// trailing digit and fail this round-trip.
func TestMoney_IsDecimal_NotFloat(t *testing.T) {
	const literal = "0.10000000000000000001"
	m := money(literal)
	assert.Equal(t, literal, m.String(),
		"Money must preserve full decimal precision (no float64 truncation)")
}

func TestSide_DebitAndCreditAreDistinct(t *testing.T) {
	// Distinct enum values — guards against a future refactor that aliases both
	// sides to the same constant (which would let any entry "balance").
	assert.NotEqual(t, domain.SideDebit, domain.SideCredit)
}
