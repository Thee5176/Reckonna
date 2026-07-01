package e2e

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thee5176/reckonna/internal/handler/problem"
	qsvc "github.com/thee5176/reckonna/internal/query"
)

func TestE2E_HealthIsPublic(t *testing.T) {
	h := newHarness(t)
	for _, path := range []string{"/command/health", "/query/health"} {
		w := h.req(t, http.MethodGet, path, "", "", nil)
		require.Equal(t, http.StatusOK, w.Code, path)
	}
}

// AT8 — any non-health endpoint without a valid JWT → 401.
func TestE2E_Unauthorized(t *testing.T) {
	h := newHarness(t)
	w := h.req(t, http.MethodGet, "/query/journal-entries", "", "", nil)
	require.Equal(t, http.StatusUnauthorized, w.Code)

	// invalid/garbage bearer likewise 401
	w = h.req(t, http.MethodGet, "/query/accounts", "", "",
		map[string]string{"Authorization": "Bearer not-a-jwt"})
	require.Equal(t, http.StatusUnauthorized, w.Code)
}

// AT1 — authenticated POST of a balanced entry persists and is retrievable, 借方=貸方.
func TestE2E_CreateAndRead(t *testing.T) {
	h := newHarness(t)
	w := h.req(t, http.MethodPost, "/command/journal-entries", "alice",
		entryJSON(10000, 40000, "1000.0000", "JPY"), nil)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	var created struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))

	g := h.req(t, http.MethodGet, "/query/journal-entries/"+created.ID, "alice", "", nil)
	require.Equal(t, http.StatusOK, g.Code)
	var view qsvc.EntryView
	require.NoError(t, json.Unmarshal(g.Body.Bytes(), &view))
	require.Len(t, view.Lines, 2)
	assert.Equal(t, `"1"`, g.Header().Get("ETag"))
}

// AT2/AT10 — unbalanced and unknown-account posts are rejected with problem+json.
func TestE2E_SemanticRejections(t *testing.T) {
	h := newHarness(t)

	t.Run("unbalanced -> 422 unbalanced_entry (AT2)", func(t *testing.T) {
		body := `{"date":"2025-01-01","lines":[
		  {"account_code":10000,"side":"debit","amount":"1000.0000","dimensions":{"currency":"JPY"}},
		  {"account_code":40000,"side":"credit","amount":"500.0000","dimensions":{"currency":"JPY"}}]}`
		w := h.req(t, http.MethodPost, "/command/journal-entries", "alice", body, nil)
		require.Equal(t, http.StatusUnprocessableEntity, w.Code)
		assert.Contains(t, w.Header().Get("Content-Type"), "application/problem+json")
		var p problem.Problem
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &p))
		assert.Equal(t, "unbalanced_entry", p.Code)
	})

	t.Run("unknown account -> 422 (AT10)", func(t *testing.T) {
		w := h.req(t, http.MethodPost, "/command/journal-entries", "alice",
			entryJSON(99999, 40000, "10.0000", "JPY"), nil)
		require.Equal(t, http.StatusUnprocessableEntity, w.Code)
		var p problem.Problem
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &p))
		assert.Equal(t, "unknown_account_code", p.Code)
	})
}

// AT13/AT14 — currency dimension: single-currency USD accepted; mixed rejected.
func TestE2E_CurrencyDimension(t *testing.T) {
	h := newHarness(t)

	t.Run("single-currency USD accepted (AT13)", func(t *testing.T) {
		w := h.req(t, http.MethodPost, "/command/journal-entries", "alice",
			entryJSON(10000, 40000, "1000.0000", "USD"), nil)
		require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	})

	t.Run("mixed currency rejected (AT14)", func(t *testing.T) {
		body := `{"date":"2025-01-01","lines":[
		  {"account_code":10000,"side":"debit","amount":"1000.0000","dimensions":{"currency":"USD"}},
		  {"account_code":40000,"side":"credit","amount":"1000.0000","dimensions":{"currency":"JPY"}}]}`
		w := h.req(t, http.MethodPost, "/command/journal-entries", "alice", body, nil)
		require.Equal(t, http.StatusUnprocessableEntity, w.Code)
		var p problem.Problem
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &p))
		assert.Equal(t, "mixed_currency", p.Code)
	})
}

// AT3/AT4 — owner scoping across the full stack.
func TestE2E_OwnerScope(t *testing.T) {
	h := newHarness(t)
	w := h.req(t, http.MethodPost, "/command/journal-entries", "alice",
		entryJSON(10000, 40000, "1000.0000", "JPY"), nil)
	require.Equal(t, http.StatusCreated, w.Code)
	var created struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))

	t.Run("bob cannot see alice's entry -> 404 (AT3/T16)", func(t *testing.T) {
		g := h.req(t, http.MethodGet, "/query/journal-entries/"+created.ID, "bob", "", nil)
		assert.Equal(t, http.StatusNotFound, g.Code)
	})

	t.Run("bob cannot delete alice's entry -> 403 (AT4)", func(t *testing.T) {
		d := h.req(t, http.MethodDelete, "/command/journal-entries/"+created.ID, "bob", "", nil)
		assert.Equal(t, http.StatusForbidden, d.Code)
	})

	t.Run("bob's list is empty", func(t *testing.T) {
		l := h.req(t, http.MethodGet, "/query/journal-entries", "bob", "", nil)
		require.Equal(t, http.StatusOK, l.Code)
		var page qsvc.PageView
		require.NoError(t, json.Unmarshal(l.Body.Bytes(), &page))
		assert.Empty(t, page.Items)
	})
}

// AT6/AT7 — statements over the full stack.
func TestE2E_Statements(t *testing.T) {
	h := newHarness(t)
	// Dr cash 1000 / Cr capital 1000 (asset vs equity)
	require.Equal(t, http.StatusCreated,
		h.req(t, http.MethodPost, "/command/journal-entries", "carol", entryJSON(10000, 30000, "1000.0000", "JPY"), nil).Code)
	// Dr cash 500 / Cr revenue 500 (income)
	require.Equal(t, http.StatusCreated,
		h.req(t, http.MethodPost, "/command/journal-entries", "carol", entryJSON(10000, 40000, "500.0000", "JPY"), nil).Code)
	// Dr staff 200 / Cr cash 200 (expense)
	require.Equal(t, http.StatusCreated,
		h.req(t, http.MethodPost, "/command/journal-entries", "carol", entryJSON(60000, 10000, "200.0000", "JPY"), nil).Code)

	t.Run("balance sheet balances (AT6)", func(t *testing.T) {
		w := h.req(t, http.MethodGet, "/query/statements/balance-sheet", "carol", "", nil)
		require.Equal(t, http.StatusOK, w.Code)
		var bs qsvc.BalanceSheetView
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &bs))
		assert.True(t, bs.Balanced, "assets == liabilities + equity")
		assert.Equal(t, bs.TotalAssets, bs.TotalLiabAndEquity)
	})

	t.Run("profit-loss net income (AT7)", func(t *testing.T) {
		w := h.req(t, http.MethodGet, "/query/statements/profit-loss", "carol", "", nil)
		require.Equal(t, http.StatusOK, w.Code)
		var pl qsvc.ProfitLossView
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &pl))
		assert.Equal(t, "500.0000", pl.Revenue)
		assert.Equal(t, "200.0000", pl.Expenses)
		assert.Equal(t, "300.0000", pl.NetIncome)
	})
}

// AT11 — NUMERIC(20,4) rounding policy is explicit and stable across round-trip.
func TestE2E_MoneyPrecision(t *testing.T) {
	h := newHarness(t)
	cases := []struct {
		input      string
		wantStored string
	}{
		{"1000.3333", "1000.3333"},  // exact-fit
		{"1000.33335", "1000.3334"}, // 5th-decimal boundary → round half away from zero
		{"0.12345", "0.1235"},       // sub-cent → 4 places
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			w := h.req(t, http.MethodPost, "/command/journal-entries", "dave",
				entryJSON(10000, 40000, tc.input, "JPY"), nil)
			require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
			var created struct {
				ID string `json:"id"`
			}
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))

			g := h.req(t, http.MethodGet, "/query/journal-entries/"+created.ID, "dave", "", nil)
			require.Equal(t, http.StatusOK, g.Code)
			var v qsvc.EntryView
			require.NoError(t, json.Unmarshal(g.Body.Bytes(), &v))
			require.Len(t, v.Lines, 2)
			for _, l := range v.Lines {
				assert.Equalf(t, tc.wantStored, l.Amount, "stable NUMERIC(20,4) rounding for %s", tc.input)
			}
		})
	}
}
