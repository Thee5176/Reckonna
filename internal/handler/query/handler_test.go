package query_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thee5176/reckonna/internal/config"
	"github.com/thee5176/reckonna/internal/domain"
	"github.com/thee5176/reckonna/internal/handler/problem"
	qhttp "github.com/thee5176/reckonna/internal/handler/query"
	qsvc "github.com/thee5176/reckonna/internal/query"
	"github.com/thee5176/reckonna/internal/service"
	"github.com/thee5176/reckonna/internal/testsupport"
)

var ctx = context.Background()

func money(s string) domain.Money { return domain.NewMoney(decimal.RequireFromString(s)) }

// import alias for domain via a thin re-export to keep line() terse.
func line(code int, side domain.Side, amt, cur string) service.LineInput {
	return service.LineInput{AccountCode: code, Side: side, Amount: money(amt),
		Dimensions: map[domain.DimensionType]string{domain.DimCurrency: cur}}
}

func setup(t *testing.T) (*gin.Engine, *service.LedgerCommandService) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	pool := testsupport.NewPostgres(t)
	bundle, err := config.LoadBundle(config.LocalesDir())
	require.NoError(t, err)
	pw := problem.NewWriter(bundle)

	cmd := service.NewLedgerCommandService(pool)
	h := qhttp.NewHandler(qsvc.NewService(pool), pw)

	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set(problem.SubKey, c.GetHeader("X-Test-Sub")); c.Next() })
	h.Register(r)
	return r, cmd
}

func post(t *testing.T, cmd *service.LedgerCommandService, owner string, lines ...service.LineInput) uuid.UUID {
	t.Helper()
	id, _, err := cmd.PostLedger(ctx, service.EntryInput{
		Date: time.Now(), Description: "seed", Owner: owner, Book: domain.BookBase, Lines: lines,
	})
	require.NoError(t, err)
	return id
}

func get(t *testing.T, r *gin.Engine, path, sub string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("X-Test-Sub", sub)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestGet_OwnerScopedRoundTrip(t *testing.T) {
	r, cmd := setup(t)
	id := post(t, cmd, "ownerA",
		line(10000, domain.SideDebit, "1000.0000", "JPY"),
		line(40000, domain.SideCredit, "1000.0000", "JPY"))

	t.Run("owner reads own entry with balanced lines (IT1)", func(t *testing.T) {
		w := get(t, r, "/query/journal-entries/"+id.String(), "ownerA")
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		var v qsvc.EntryView
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &v))
		assert.Equal(t, id.String(), v.ID)
		require.Len(t, v.Lines, 2)
		debit, credit := decimal.Zero, decimal.Zero
		for _, l := range v.Lines {
			if l.Side == "debit" {
				debit = debit.Add(decimal.RequireFromString(l.Amount))
			} else {
				credit = credit.Add(decimal.RequireFromString(l.Amount))
			}
		}
		assert.True(t, debit.Equal(credit), "debit==credit")
	})

	t.Run("cross-owner GET returns 404 not 403 (T16)", func(t *testing.T) {
		w := get(t, r, "/query/journal-entries/"+id.String(), "ownerB")
		require.Equal(t, http.StatusNotFound, w.Code)
		var p problem.Problem
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &p))
		assert.Equal(t, "not_found", p.Code)
	})
}

func TestList_OwnerScopeAndPagination(t *testing.T) {
	r, cmd := setup(t)
	for i := 0; i < 3; i++ {
		post(t, cmd, "ownerA", line(10000, domain.SideDebit, "10.0000", "JPY"), line(40000, domain.SideCredit, "10.0000", "JPY"))
	}
	post(t, cmd, "ownerB", line(10000, domain.SideDebit, "5.0000", "JPY"), line(40000, domain.SideCredit, "5.0000", "JPY"))

	t.Run("list is owner-scoped (AT3, IT5)", func(t *testing.T) {
		w := get(t, r, "/query/journal-entries", "ownerA")
		require.Equal(t, http.StatusOK, w.Code)
		var page qsvc.PageView
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &page))
		assert.Len(t, page.Items, 3, "only ownerA's three entries")
	})

	t.Run("cursor pagination (AT3a)", func(t *testing.T) {
		w := get(t, r, "/query/journal-entries?limit=2", "ownerA")
		var p1 qsvc.PageView
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &p1))
		require.Len(t, p1.Items, 2)
		require.True(t, p1.HasMore)
		require.NotNil(t, p1.NextCursor)

		w2 := get(t, r, "/query/journal-entries?limit=2&cursor="+*p1.NextCursor, "ownerA")
		var p2 qsvc.PageView
		require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &p2))
		assert.Len(t, p2.Items, 1)
		assert.False(t, p2.HasMore)
		assert.Nil(t, p2.NextCursor)
	})

	t.Run("invalid cursor -> 400 invalid_cursor", func(t *testing.T) {
		w := get(t, r, "/query/journal-entries?cursor=short", "ownerA") // valid chars, wrong length
		require.Equal(t, http.StatusBadRequest, w.Code)
		var p problem.Problem
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &p))
		assert.Equal(t, "invalid_cursor", p.Code)
	})
}

func TestListAccounts(t *testing.T) {
	r, _ := setup(t)
	w := get(t, r, "/query/accounts", "ownerA")
	require.Equal(t, http.StatusOK, w.Code)
	var body struct {
		Accounts []qsvc.AccountView `json:"accounts"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Len(t, body.Accounts, 20, "full chart returned (AT9)")
}

func TestBalanceSheet_Identity(t *testing.T) {
	r, cmd := setup(t)
	// Dr cash 1000 / Cr contributed capital 1000  → assets == equity
	post(t, cmd, "ownerBS",
		line(10000, domain.SideDebit, "1000.0000", "JPY"),
		line(30000, domain.SideCredit, "1000.0000", "JPY"))

	w := get(t, r, "/query/statements/balance-sheet", "ownerBS")
	require.Equal(t, http.StatusOK, w.Code)
	var bs qsvc.BalanceSheetView
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &bs))
	assert.True(t, bs.Balanced, "assets == liabilities + equity (AT6)")
	assert.Equal(t, bs.TotalAssets, bs.TotalLiabAndEquity)
	assert.Equal(t, "1000.0000", bs.TotalAssets)
}

func TestProfitLoss_NetIncome(t *testing.T) {
	r, cmd := setup(t)
	post(t, cmd, "ownerPL", line(10000, domain.SideDebit, "1000.0000", "JPY"), line(40000, domain.SideCredit, "1000.0000", "JPY")) // revenue 1000
	post(t, cmd, "ownerPL", line(60000, domain.SideDebit, "300.0000", "JPY"), line(10000, domain.SideCredit, "300.0000", "JPY"))   // expense 300

	w := get(t, r, "/query/statements/profit-loss", "ownerPL")
	require.Equal(t, http.StatusOK, w.Code)
	var pl qsvc.ProfitLossView
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &pl))
	assert.Equal(t, "1000.0000", pl.Revenue)
	assert.Equal(t, "300.0000", pl.Expenses)
	assert.Equal(t, "700.0000", pl.NetIncome, "netIncome == revenue - expenses (AT7)")
}

func TestBalances_RepeatedParam(t *testing.T) {
	r, cmd := setup(t)
	post(t, cmd, "ownerBAL", line(10000, domain.SideDebit, "1000.0000", "JPY"), line(40000, domain.SideCredit, "1000.0000", "JPY"))

	t.Run("balances for requested accounts", func(t *testing.T) {
		w := get(t, r, "/query/balances?account=10000&account=40000", "ownerBAL")
		require.Equal(t, http.StatusOK, w.Code)
		var body struct {
			Balances []qsvc.BalanceView `json:"balances"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		got := map[int]string{}
		for _, b := range body.Balances {
			got[b.AccountCode] = b.Balance
		}
		assert.Equal(t, "1000.0000", got[10000]) // cash, debit-normal
		assert.Equal(t, "1000.0000", got[40000]) // revenue, credit-normal
	})

	t.Run("empty account list -> 400", func(t *testing.T) {
		w := get(t, r, "/query/balances", "ownerBAL")
		require.Equal(t, http.StatusBadRequest, w.Code)
	})
}
