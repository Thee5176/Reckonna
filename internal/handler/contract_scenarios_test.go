package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thee5176/reckonna/internal/handler/problem"
	qsvc "github.com/thee5176/reckonna/internal/query"
)

// decodeID extracts the id from a create-journal-entry 201 response body.
func decodeID(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var created struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	require.NotEmpty(t, created.ID)
	return created.ID
}

// TestContract_CreateJournalEntry_201 checks the 201 shape: id+version body,
// ETag + Location headers, all matching the spec.
func TestContract_CreateJournalEntry_201(t *testing.T) {
	r := newContractRouter(t)
	w := call(t, r, http.MethodPost, "/command/journal-entries", "ownerA", cBalanced, nil, true)

	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	assert.Regexp(t, `^"\d+"$`, w.Header().Get("ETag"))
	assert.Contains(t, w.Header().Get("Location"), "/command/journal-entries/")

	var created struct {
		ID      string `json:"id"`
		Version int32  `json:"version"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	assert.NotEmpty(t, created.ID)
	assert.Equal(t, int32(1), created.Version)
}

// TestContract_CreateJournalEntry_Unbalanced_422 checks 借方≠貸方 -> 422
// unbalanced_entry, response validated against the Problem schema.
func TestContract_CreateJournalEntry_Unbalanced_422(t *testing.T) {
	r := newContractRouter(t)
	w := call(t, r, http.MethodPost, "/command/journal-entries", "ownerA", cUnbalanced, nil, true)

	require.Equal(t, http.StatusUnprocessableEntity, w.Code)
	var p problem.Problem
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &p))
	assert.Equal(t, "unbalanced_entry", p.Code)
}

// TestContract_CreateJournalEntry_WrongContentType_415 checks a non-JSON
// Content-Type is rejected before body parsing. The request itself is
// deliberately spec-invalid (Content-Type not declared for the operation), so
// we only assert the response, not the request.
func TestContract_CreateJournalEntry_WrongContentType_415(t *testing.T) {
	r := newContractRouter(t)
	w := call(t, r, http.MethodPost, "/command/journal-entries", "ownerA", cBalanced,
		map[string]string{"Content-Type": "text/plain"}, false)

	require.Equal(t, http.StatusUnsupportedMediaType, w.Code)
	var p problem.Problem
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &p))
	assert.Equal(t, "unsupported_media_type", p.Code)
}

// TestContract_UpdateJournalEntry_MissingIfMatch_428 checks the
// force-opt-in-to-concurrency-control rule.
func TestContract_UpdateJournalEntry_MissingIfMatch_428(t *testing.T) {
	r := newContractRouter(t)
	created := call(t, r, http.MethodPost, "/command/journal-entries", "ownerA", cBalanced, nil, true)
	id := decodeID(t, created)

	w := call(t, r, http.MethodPut, "/command/journal-entries/"+id, "ownerA", cBalanced, nil, true)
	require.Equal(t, http.StatusPreconditionRequired, w.Code)
}

// TestContract_UpdateJournalEntry_StaleIfMatch_409 checks the conflict body
// carries current_version, matching the spec's Problem.current_version field.
func TestContract_UpdateJournalEntry_StaleIfMatch_409(t *testing.T) {
	r := newContractRouter(t)
	created := call(t, r, http.MethodPost, "/command/journal-entries", "ownerA", cBalanced, nil, true)
	id := decodeID(t, created)

	w := call(t, r, http.MethodPut, "/command/journal-entries/"+id, "ownerA", cBalanced,
		map[string]string{"If-Match": `"99"`}, true)
	require.Equal(t, http.StatusConflict, w.Code)

	var p problem.Problem
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &p))
	assert.Equal(t, "concurrency_conflict", p.Code)
	require.NotNil(t, p.CurrentVersion)
	assert.Equal(t, int32(1), *p.CurrentVersion)
}

// TestContract_GetJournalEntry_CrossOwner_404 checks the enumeration defense:
// a read for someone else's entry is 404, not 403, and validates against the
// spec's response schema for GET /query/journal-entries/{id}.
func TestContract_GetJournalEntry_CrossOwner_404(t *testing.T) {
	r := newContractRouter(t)
	created := call(t, r, http.MethodPost, "/command/journal-entries", "ownerA", cBalanced, nil, true)
	id := decodeID(t, created)

	w := call(t, r, http.MethodGet, "/query/journal-entries/"+id, "ownerB", "", nil, true)
	require.Equal(t, http.StatusNotFound, w.Code)
	var p problem.Problem
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &p))
	assert.Equal(t, "not_found", p.Code)
}

// TestContract_GetJournalEntry_OwnRoundTrip checks the happy-path GET,
// including the ETag header, against the spec.
func TestContract_GetJournalEntry_OwnRoundTrip(t *testing.T) {
	r := newContractRouter(t)
	created := call(t, r, http.MethodPost, "/command/journal-entries", "ownerA", cBalanced, nil, true)
	id := decodeID(t, created)

	w := call(t, r, http.MethodGet, "/query/journal-entries/"+id, "ownerA", "", nil, true)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Regexp(t, `^"\d+"$`, w.Header().Get("ETag"))

	var v qsvc.EntryView
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &v))
	assert.Equal(t, id, v.ID)
	assert.Len(t, v.Lines, 2)
}

// TestContract_GetJournalLines checks GET /query/journal-lines/{id}.
func TestContract_GetJournalLines(t *testing.T) {
	r := newContractRouter(t)
	created := call(t, r, http.MethodPost, "/command/journal-entries", "ownerA", cBalanced, nil, true)
	id := decodeID(t, created)

	w := call(t, r, http.MethodGet, "/query/journal-lines/"+id, "ownerA", "", nil, true)
	require.Equal(t, http.StatusOK, w.Code)
}

// TestContract_DeleteJournalEntry_204 checks the 204-no-body success shape.
func TestContract_DeleteJournalEntry_204(t *testing.T) {
	r := newContractRouter(t)
	created := call(t, r, http.MethodPost, "/command/journal-entries", "ownerA", cBalanced, nil, true)
	id := decodeID(t, created)

	w := call(t, r, http.MethodDelete, "/command/journal-entries/"+id, "ownerA", "", nil, true)
	require.Equal(t, http.StatusNoContent, w.Code)
	assert.Empty(t, w.Body.Bytes())
}

// TestContract_ListJournalEntries_InvalidCursor_400 checks a syntactically
// wrong cursor -> 400 invalid_cursor.
func TestContract_ListJournalEntries_InvalidCursor_400(t *testing.T) {
	r := newContractRouter(t)
	w := call(t, r, http.MethodGet, "/query/journal-entries?cursor=short", "ownerA", "", nil, true)
	require.Equal(t, http.StatusBadRequest, w.Code)
	var p problem.Problem
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &p))
	assert.Equal(t, "invalid_cursor", p.Code)
}

// TestContract_ListJournalEntries_Page checks the cursor-page envelope
// {items, next_cursor, has_more} against the spec.
func TestContract_ListJournalEntries_Page(t *testing.T) {
	r := newContractRouter(t)
	call(t, r, http.MethodPost, "/command/journal-entries", "ownerPage", cBalanced, nil, true)
	call(t, r, http.MethodPost, "/command/journal-entries", "ownerPage", cBalanced, nil, true)

	w := call(t, r, http.MethodGet, "/query/journal-entries?limit=1", "ownerPage", "", nil, true)
	require.Equal(t, http.StatusOK, w.Code)
	var page qsvc.PageView
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &page))
	require.Len(t, page.Items, 1)
	assert.True(t, page.HasMore)
	require.NotNil(t, page.NextCursor)
}

// TestContract_ListAccounts checks GET /query/accounts against the spec.
func TestContract_ListAccounts(t *testing.T) {
	r := newContractRouter(t)
	w := call(t, r, http.MethodGet, "/query/accounts", "ownerA", "", nil, true)
	require.Equal(t, http.StatusOK, w.Code)
}

// TestContract_Balances checks the repeated ?account= param, both the happy
// path and the "empty -> 400" rule.
func TestContract_Balances(t *testing.T) {
	r := newContractRouter(t)
	call(t, r, http.MethodPost, "/command/journal-entries", "ownerBal", cBalanced, nil, true)

	t.Run("populated", func(t *testing.T) {
		w := call(t, r, http.MethodGet, "/query/balances?account=10000&account=40000", "ownerBal", "", nil, true)
		require.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("empty -> 400", func(t *testing.T) {
		w := call(t, r, http.MethodGet, "/query/balances", "ownerBal", "", nil, false)
		require.Equal(t, http.StatusBadRequest, w.Code)
	})
}

// TestContract_Statements checks both statement endpoints against the spec.
func TestContract_Statements(t *testing.T) {
	r := newContractRouter(t)
	call(t, r, http.MethodPost, "/command/journal-entries", "ownerStmt", cBalanced, nil, true)

	t.Run("balance-sheet", func(t *testing.T) {
		w := call(t, r, http.MethodGet, "/query/statements/balance-sheet", "ownerStmt", "", nil, true)
		require.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("profit-loss", func(t *testing.T) {
		w := call(t, r, http.MethodGet, "/query/statements/profit-loss", "ownerStmt", "", nil, true)
		require.Equal(t, http.StatusOK, w.Code)
	})
}

// TestContract_Health checks both liveness endpoints against the spec.
func TestContract_Health(t *testing.T) {
	r := newContractRouter(t)
	t.Run("command", func(t *testing.T) {
		w := call(t, r, http.MethodGet, "/command/health", "", "", nil, true)
		require.Equal(t, http.StatusOK, w.Code)
	})
	t.Run("query", func(t *testing.T) {
		w := call(t, r, http.MethodGet, "/query/health", "", "", nil, true)
		require.Equal(t, http.StatusOK, w.Code)
	})
}
