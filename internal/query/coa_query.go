package query

import (
	"context"
	"fmt"

	qdb "github.com/thee5176/reckonna/internal/repository/query"
)

// AccountView is one chart-of-accounts entry in a read response.
type AccountView struct {
	Code          int    `json:"code"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	Type          string `json:"type"`
	NormalBalance string `json:"normal_balance"`
	Postable      bool   `json:"postable"`
	IFRSLineItem  string `json:"ifrs_line_item"`
	Status        string `json:"status"`
}

// ListAccounts returns the active chart of accounts ordered by code (AT9).
func (s *Service) ListAccounts(ctx context.Context) ([]AccountView, error) {
	rows, err := s.q.ListAccounts(ctx)
	if err != nil {
		return nil, fmt.Errorf("list accounts: %w", err)
	}
	out := make([]AccountView, len(rows))
	for i, r := range rows {
		out[i] = AccountView{
			Code:          int(r.Code),
			Name:          r.Name,
			Description:   r.Description,
			Type:          string(r.Type),
			NormalBalance: string(r.NormalBalance),
			Postable:      r.Postable,
			IFRSLineItem:  r.IfrsLineItem,
			Status:        string(r.Status),
		}
	}
	return out, nil
}

// BalanceView is an account's outstanding balance, signed to its normal side.
type BalanceView struct {
	AccountCode int    `json:"account_code"`
	AccountName string `json:"account_name"`
	Balance     string `json:"balance"`
}

// Balances returns owner-scoped outstanding balances for the requested account
// codes. The caller must pass 1..MaxAccounts codes (empty → handler 400,
// >MaxAccounts → ErrTooManyItems). Balance is signed to the account's normal
// side: debit-normal = debit−credit; credit-normal = credit−debit.
func (s *Service) Balances(ctx context.Context, owner string, codes []int) ([]BalanceView, error) {
	if len(codes) > MaxAccounts {
		return nil, ErrTooManyItems
	}
	arg := make([]int32, 0, len(codes))
	for _, c := range codes {
		// Only 5-digit CoA codes can exist; bound the value before narrowing to
		// int32 so an out-of-range input can never wrap (defense in depth — the
		// handler already validates the range).
		if c < 10000 || c > 99999 {
			continue
		}
		arg = append(arg, int32(c))
	}
	rows, err := s.q.GetOutstandingBalances(ctx, qdb.GetOutstandingBalancesParams{OwnerSub: owner, Codes: arg})
	if err != nil {
		return nil, fmt.Errorf("outstanding balances: %w", err)
	}
	out := make([]BalanceView, len(rows))
	for i, r := range rows {
		bal := r.TotalDebit.Sub(r.TotalCredit)
		if r.NormalBalance == qdb.NormalBalanceCredit {
			bal = r.TotalCredit.Sub(r.TotalDebit)
		}
		out[i] = BalanceView{
			AccountCode: int(r.AccountCode),
			AccountName: r.AccountName,
			Balance:     bal.StringFixed(4),
		}
	}
	return out, nil
}
