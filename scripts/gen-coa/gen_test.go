package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestValidate_RealCoAIsClean is the CI gate: the checked-in config/coa.yaml
// must satisfy the governance standard. A hand-edit that breaks a rule fails here.
func TestValidate_RealCoAIsClean(t *testing.T) {
	coa, err := LoadCoA("../../config/coa.yaml")
	require.NoError(t, err)
	require.NotEmpty(t, coa.Accounts)

	errs := coa.Validate()
	assert.Emptyf(t, errs, "config/coa.yaml must be clean, got: %v", errs)

	// The escrow account must keep its required counterparty dimension (proves §7 R7.4 is seeded).
	var escrow *Account
	for i := range coa.Accounts {
		if coa.Accounts[i].Code == 21500 {
			escrow = &coa.Accounts[i]
		}
	}
	require.NotNil(t, escrow, "21500 Customer escrow payable must exist")
	assert.Contains(t, escrow.RequiredDimensions, "counterparty")
}

// TestValidate_CatchesViolations asserts each governance rule is actually enforced.
func TestValidate_CatchesViolations(t *testing.T) {
	base := Account{
		Code: 10000, Name: "Cash", Description: "d", Type: "asset", NormalBalance: "debit",
		Postable: true, CurrentNoncurrent: "current", IFRSLineItem: "Cash",
		AllowedBooks: []string{"base"}, Status: "active", Owner: "steward",
	}

	tests := []struct {
		name    string
		mutate  func(a *Account)
		wantSub string
	}{
		{"type/balance mismatch", func(a *Account) { a.NormalBalance = "credit" }, "requires normal_balance"},
		{"code out of range for type", func(a *Account) { a.Code = 40000 }, "out of range"},
		{"unknown dimension", func(a *Account) { a.RequiredDimensions = []string{"nope"} }, "unknown dimension"},
		{"missing description", func(a *Account) { a.Description = "" }, "missing description"},
		{"missing ifrs_line_item", func(a *Account) { a.IFRSLineItem = "" }, "missing ifrs_line_item"},
		{"missing owner", func(a *Account) { a.Owner = "" }, "missing owner"},
		{"asset without current_noncurrent", func(a *Account) { a.CurrentNoncurrent = "" }, "current_noncurrent required"},
		{"unknown book", func(a *Account) { a.AllowedBooks = []string{"ifrs"} }, "unknown book"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := base
			tt.mutate(&a)
			c := &CoA{Accounts: []Account{a}}
			errs := c.Validate()
			require.NotEmpty(t, errs, "expected a violation")
			joined := ""
			for _, e := range errs {
				joined += e.Error() + "\n"
			}
			assert.Contains(t, joined, tt.wantSub)
		})
	}
}

// TestValidate_DuplicateCodeAndName catches collisions across accounts.
func TestValidate_DuplicateCodeAndName(t *testing.T) {
	a := Account{Code: 10000, Name: "Cash", Description: "d", Type: "asset", NormalBalance: "debit",
		Postable: true, CurrentNoncurrent: "current", IFRSLineItem: "Cash", AllowedBooks: []string{"base"}, Status: "active", Owner: "s"}
	c := &CoA{Accounts: []Account{a, a}}
	errs := c.Validate()
	joined := ""
	for _, e := range errs {
		joined += e.Error() + "\n"
	}
	assert.Contains(t, joined, "duplicate code")
	assert.Contains(t, joined, "duplicate name")
}

// TestSeedSQL_Shape asserts the generated seed is well-formed and round-trips
// the escrow account's required-dimension array literal.
func TestSeedSQL_Shape(t *testing.T) {
	coa, err := LoadCoA("../../config/coa.yaml")
	require.NoError(t, err)

	up, down := coa.SeedSQL()
	assert.Contains(t, up, "INSERT INTO account")
	assert.Contains(t, up, "(10000, 'Cash and cash equivalents'")
	assert.Contains(t, up, "'{\"counterparty\"}'") // escrow required dimension array
	assert.Contains(t, up, "NULL")                 // equity/income/expense current_noncurrent
	assert.Contains(t, down, "DELETE FROM account WHERE code IN (")
	assert.Contains(t, down, "21500")
}

func TestPgArray(t *testing.T) {
	assert.Equal(t, "'{}'", pgArray(nil))
	assert.Equal(t, "'{\"base\"}'", pgArray([]string{"base"}))
	assert.Equal(t, "'{\"counterparty\"}'", pgArray([]string{"counterparty"}))
}
