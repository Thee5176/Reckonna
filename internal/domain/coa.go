package domain

// AccountType classifies the nature of an account (standard §6). It drives
// statement placement: asset/liability/equity → balance sheet; income/expense
// → profit & loss.
type AccountType string

const (
	AccountAsset     AccountType = "asset"
	AccountLiability AccountType = "liability"
	AccountEquity    AccountType = "equity"
	AccountIncome    AccountType = "income"
	AccountExpense   AccountType = "expense"
)

// Valid reports whether t is a known account type.
func (t AccountType) Valid() bool {
	switch t {
	case AccountAsset, AccountLiability, AccountEquity, AccountIncome, AccountExpense:
		return true
	default:
		return false
	}
}

// NormalBalance is the side on which an account's balance normally sits.
type NormalBalance string

const (
	NormalDebit  NormalBalance = "debit"
	NormalCredit NormalBalance = "credit"
)

// Account is the value-object view of a chart-of-accounts entry as needed by
// the domain to validate a posting. The full master (name, description,
// mappings) lives in config/coa.yaml and the DB; the domain only needs the
// posting-relevant attributes.
type Account struct {
	Code               int
	Type               AccountType
	NormalBalance      NormalBalance
	Postable           bool
	RequiredDimensions []DimensionType
}
