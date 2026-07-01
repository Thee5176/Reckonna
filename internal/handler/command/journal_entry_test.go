package command_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thee5176/reckonna/internal/config"
	"github.com/thee5176/reckonna/internal/handler/command"
	"github.com/thee5176/reckonna/internal/handler/middleware"
	"github.com/thee5176/reckonna/internal/handler/problem"
	"github.com/thee5176/reckonna/internal/service"
	"github.com/thee5176/reckonna/internal/testsupport"
)

const balancedBody = `{"date":"2025-01-01","description":"sale","lines":[
  {"account_code":10000,"side":"debit","amount":"1000.0000","dimensions":{"currency":"JPY"}},
  {"account_code":40000,"side":"credit","amount":"1000.0000","dimensions":{"currency":"JPY"}}]}`

const unbalancedBody = `{"date":"2025-01-01","description":"bad","lines":[
  {"account_code":10000,"side":"debit","amount":"1000.0000","dimensions":{"currency":"JPY"}},
  {"account_code":40000,"side":"credit","amount":"500.0000","dimensions":{"currency":"JPY"}}]}`

func newRouter(t *testing.T) (*gin.Engine, *pgxpool.Pool) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	pool := testsupport.NewPostgres(t)
	bundle, err := config.LoadBundle(config.LocalesDir())
	require.NoError(t, err)
	pw := problem.NewWriter(bundle)
	svc := service.NewLedgerCommandService(pool)
	h := command.NewHandler(svc, pw, pool)

	r := gin.New()
	r.Use(middleware.Locale())
	r.Use(middleware.RequireJSON(pw))
	// test auth stub: owner id from a header (real auth is S10)
	r.Use(func(c *gin.Context) { c.Set(problem.SubKey, c.GetHeader("X-Test-Sub")); c.Next() })
	r.POST("/command/journal-entries", h.Idempotency(), h.Post)
	r.PUT("/command/journal-entries/:id", h.Put)
	r.DELETE("/command/journal-entries/:id", h.Delete)
	return r, pool
}

func do(t *testing.T, r *gin.Engine, method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("X-Test-Sub", "ownerA")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestPost_Balanced(t *testing.T) {
	r, pool := newRouter(t)
	w := do(t, r, http.MethodPost, "/command/journal-entries", balancedBody, nil)

	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")
	assert.Equal(t, `"1"`, w.Header().Get("ETag"))

	var n int
	require.NoError(t, pool.QueryRow(context.Background(),
		"SELECT count(*) FROM journal_entry WHERE owner_sub='ownerA'").Scan(&n))
	assert.Equal(t, 1, n)
}

func TestPost_Unbalanced_Problem(t *testing.T) {
	r, _ := newRouter(t)
	w := do(t, r, http.MethodPost, "/command/journal-entries", unbalancedBody, nil)

	require.Equal(t, http.StatusUnprocessableEntity, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "application/problem+json")

	var p problem.Problem
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &p))
	assert.Equal(t, "unbalanced_entry", p.Code) // assert on code, not localized text
	assert.Equal(t, 422, p.Status)
	require.Len(t, p.Errors, 1)
	require.NotNil(t, p.Errors[0].LineIndex)
	assert.Equal(t, 0, *p.Errors[0].LineIndex)
	assert.Equal(t, "amount", p.Errors[0].Field)
	assert.Equal(t, "debit_credit_mismatch", p.Errors[0].Issue)
}

func TestPost_UnknownAccount(t *testing.T) {
	r, _ := newRouter(t)
	body := strings.Replace(balancedBody, "10000", "99999", 1)
	w := do(t, r, http.MethodPost, "/command/journal-entries", body, nil)
	require.Equal(t, http.StatusUnprocessableEntity, w.Code)
	var p problem.Problem
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &p))
	assert.Equal(t, "unknown_account_code", p.Code)
}

func TestPost_WrongContentType_415(t *testing.T) {
	r, _ := newRouter(t)
	req := httptest.NewRequest(http.MethodPost, "/command/journal-entries", strings.NewReader(balancedBody))
	req.Header.Set("Content-Type", "text/plain")
	req.Header.Set("X-Test-Sub", "ownerA")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusUnsupportedMediaType, w.Code)
	var p problem.Problem
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &p))
	assert.Equal(t, "unsupported_media_type", p.Code)
}

func TestPut_PreconditionAndConflict(t *testing.T) {
	r, _ := newRouter(t)
	// create first
	w := do(t, r, http.MethodPost, "/command/journal-entries", balancedBody, nil)
	require.Equal(t, http.StatusCreated, w.Code)
	var created struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	path := "/command/journal-entries/" + created.ID

	t.Run("missing If-Match -> 428 (AT16b)", func(t *testing.T) {
		w := do(t, r, http.MethodPut, path, balancedBody, nil)
		assert.Equal(t, http.StatusPreconditionRequired, w.Code)
	})

	t.Run("stale If-Match -> 409 with current version (AT16)", func(t *testing.T) {
		w := do(t, r, http.MethodPut, path, balancedBody, map[string]string{"If-Match": `"99"`})
		require.Equal(t, http.StatusConflict, w.Code)
		var p problem.Problem
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &p))
		assert.Equal(t, "concurrency_conflict", p.Code)
		require.NotNil(t, p.CurrentVersion)
		assert.Equal(t, int32(1), *p.CurrentVersion)
	})

	t.Run("correct If-Match -> 200 new version", func(t *testing.T) {
		w := do(t, r, http.MethodPut, path, balancedBody, map[string]string{"If-Match": `"1"`})
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.Equal(t, `"2"`, w.Header().Get("ETag"))
	})
}

func TestPost_IdempotencyReplayAndConflict(t *testing.T) {
	r, pool := newRouter(t)
	key := map[string]string{"Idempotency-Key": "11111111-1111-1111-1111-111111111111"}

	first := do(t, r, http.MethodPost, "/command/journal-entries", balancedBody, key)
	require.Equal(t, http.StatusCreated, first.Code)

	t.Run("same key + same body -> cached 201, no second row (AT15)", func(t *testing.T) {
		second := do(t, r, http.MethodPost, "/command/journal-entries", balancedBody, key)
		require.Equal(t, http.StatusCreated, second.Code)
		assert.JSONEq(t, first.Body.String(), second.Body.String())
		var n int
		require.NoError(t, pool.QueryRow(context.Background(),
			"SELECT count(*) FROM journal_entry WHERE owner_sub='ownerA'").Scan(&n))
		assert.Equal(t, 1, n)
	})

	t.Run("same key + different body -> 422 duplicate (AT15b)", func(t *testing.T) {
		w := do(t, r, http.MethodPost, "/command/journal-entries", unbalancedBody, key)
		require.Equal(t, http.StatusUnprocessableEntity, w.Code)
		var p problem.Problem
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &p))
		assert.Equal(t, "duplicate_idempotency_key", p.Code)
	})
}

func TestPost_LocalizedError_JA(t *testing.T) {
	r, _ := newRouter(t)
	w := do(t, r, http.MethodPost, "/command/journal-entries", unbalancedBody,
		map[string]string{"Accept-Language": "ja"})
	require.Equal(t, http.StatusUnprocessableEntity, w.Code)

	var p problem.Problem
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &p))
	assert.Equal(t, "unbalanced_entry", p.Code) // code is locale-neutral

	bundle, err := config.LoadBundle(config.LocalesDir())
	require.NoError(t, err)
	ja := bundle.Error("ja", "unbalanced_entry")
	assert.Equal(t, ja.Title, p.Title, "title must be Japanese (AT18)")
	assert.Equal(t, ja.Detail, p.Detail)
}
