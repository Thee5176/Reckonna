package query

import (
	"context"
	"fmt"

	"github.com/shopspring/decimal"

	qdb "github.com/thee5176/reckonna/internal/repository/query"
)

// LineBalance is one account's presented (positive) balance in a statement.
type LineBalance struct {
	Code    int    `json:"code"`
	Name    string `json:"name"`
	Balance string `json:"balance"`
}

// BalanceSheetView groups balances by element and asserts the accounting
// identity Assets == Liabilities + Equity (AT6).
type BalanceSheetView struct {
	Assets             []LineBalance `json:"assets"`
	Liabilities        []LineBalance `json:"liabilities"`
	Equity             []LineBalance `json:"equity"`
	TotalAssets        string        `json:"total_assets"`
	TotalLiabAndEquity string        `json:"total_liabilities_and_equity"`
	Balanced           bool          `json:"balanced"`
}

// BalanceSheet builds the owner-scoped balance sheet (AT6, IT7). Asset balance =
// net_debit; liability/equity balance = −net_debit (i.e. net credit).
func (s *Service) BalanceSheet(ctx context.Context, owner string) (*BalanceSheetView, error) {
	rows, err := s.q.BalanceSheet(ctx, owner)
	if err != nil {
		return nil, fmt.Errorf("balance sheet: %w", err)
	}
	view := &BalanceSheetView{Assets: []LineBalance{}, Liabilities: []LineBalance{}, Equity: []LineBalance{}}
	totalAssets := decimal.Zero
	totalLiabEquity := decimal.Zero

	for _, r := range rows {
		switch r.Type {
		case qdb.AccountTypeAsset:
			bal := r.NetDebit
			totalAssets = totalAssets.Add(bal)
			view.Assets = append(view.Assets, lineBalance(int(r.Code), r.Name, bal))
		case qdb.AccountTypeLiability:
			bal := r.NetDebit.Neg()
			totalLiabEquity = totalLiabEquity.Add(bal)
			view.Liabilities = append(view.Liabilities, lineBalance(int(r.Code), r.Name, bal))
		case qdb.AccountTypeEquity:
			bal := r.NetDebit.Neg()
			totalLiabEquity = totalLiabEquity.Add(bal)
			view.Equity = append(view.Equity, lineBalance(int(r.Code), r.Name, bal))
		}
	}
	// Current-period earnings (revenue − expenses) are unclosed and belong to
	// equity. Without a closing entry the naive Assets == Liab + Equity identity
	// would be off by exactly net income, so we fold it into equity here (a
	// synthetic "Current period earnings" line, code 0). Closing entries are a
	// later phase; until then this keeps the balance sheet balanced (AT6).
	netIncome, err := s.netIncome(ctx, owner)
	if err != nil {
		return nil, err
	}
	if !netIncome.IsZero() {
		view.Equity = append(view.Equity, lineBalance(0, "Current period earnings", netIncome))
		totalLiabEquity = totalLiabEquity.Add(netIncome)
	}

	view.TotalAssets = totalAssets.StringFixed(4)
	view.TotalLiabAndEquity = totalLiabEquity.StringFixed(4)
	view.Balanced = totalAssets.Equal(totalLiabEquity)
	return view, nil
}

// netIncome returns owner-scoped revenue − expenses as an exact decimal. It sums
// net_credit over all P&L accounts (income positive, expense negative).
func (s *Service) netIncome(ctx context.Context, owner string) (decimal.Decimal, error) {
	rows, err := s.q.ProfitLoss(ctx, owner)
	if err != nil {
		return decimal.Zero, fmt.Errorf("net income: %w", err)
	}
	total := decimal.Zero
	for _, r := range rows {
		total = total.Add(r.NetCredit)
	}
	return total, nil
}

// ProfitLossView reports revenue, expenses, and net income (AT7).
type ProfitLossView struct {
	Income    []LineBalance `json:"income"`
	Expense   []LineBalance `json:"expense"`
	Revenue   string        `json:"revenue"`
	Expenses  string        `json:"expenses"`
	NetIncome string        `json:"net_income"`
}

// ProfitLoss builds the owner-scoped P&L (AT7, IT7). Income balance = net_credit;
// expense presented as −net_credit. netIncome = revenue − expenses.
func (s *Service) ProfitLoss(ctx context.Context, owner string) (*ProfitLossView, error) {
	rows, err := s.q.ProfitLoss(ctx, owner)
	if err != nil {
		return nil, fmt.Errorf("profit and loss: %w", err)
	}
	view := &ProfitLossView{Income: []LineBalance{}, Expense: []LineBalance{}}
	revenue := decimal.Zero
	expenses := decimal.Zero

	for _, r := range rows {
		switch r.Type {
		case qdb.AccountTypeIncome:
			revenue = revenue.Add(r.NetCredit)
			view.Income = append(view.Income, lineBalance(int(r.Code), r.Name, r.NetCredit))
		case qdb.AccountTypeExpense:
			exp := r.NetCredit.Neg()
			expenses = expenses.Add(exp)
			view.Expense = append(view.Expense, lineBalance(int(r.Code), r.Name, exp))
		}
	}
	view.Revenue = revenue.StringFixed(4)
	view.Expenses = expenses.StringFixed(4)
	view.NetIncome = revenue.Sub(expenses).StringFixed(4)
	return view, nil
}

func lineBalance(code int, name string, bal decimal.Decimal) LineBalance {
	return LineBalance{Code: code, Name: name, Balance: bal.StringFixed(4)}
}
