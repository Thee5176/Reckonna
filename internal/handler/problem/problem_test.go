package problem_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thee5176/reckonna/internal/config"
	"github.com/thee5176/reckonna/internal/handler/problem"
)

func testWriter(t *testing.T) *problem.Writer {
	t.Helper()
	bundle, err := config.LoadBundle(config.LocalesDir())
	require.NoError(t, err)
	return problem.NewWriter(bundle)
}

// TestWriter_Write_Envelope asserts the RFC 7807 shape: problem+json content
// type, Vary header, kebab-case type URL (dashed), instance path, locale-neutral
// code, per-line errors, and that the gin chain is aborted.
func TestWriter_Write_Envelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := testWriter(t)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/command/journal-entries", nil)

	idx := 0
	w.Write(c, http.StatusUnprocessableEntity, "unbalanced_entry",
		[]problem.FieldError{{LineIndex: &idx, Field: "amount", Issue: "debit_credit_mismatch"}}, nil)

	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "application/problem+json")
	assert.Equal(t, "Accept-Language", rec.Header().Get("Vary"))
	assert.True(t, c.IsAborted())

	var p problem.Problem
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &p))
	assert.Equal(t, "unbalanced_entry", p.Code)
	assert.Equal(t, 422, p.Status)
	assert.Equal(t, "https://reckonna.dev/errors/unbalanced-entry", p.Type) // dashed()
	assert.Equal(t, "/command/journal-entries", p.Instance)
	require.Len(t, p.Errors, 1)
	assert.Equal(t, "amount", p.Errors[0].Field)
	require.NotNil(t, p.Errors[0].LineIndex)
	assert.Equal(t, 0, *p.Errors[0].LineIndex)
}

// TestWriter_Write_CurrentVersion asserts the optional current_version field is
// emitted for concurrency conflicts.
func TestWriter_Write_CurrentVersion(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := testWriter(t)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPut, "/command/journal-entries/x", nil)

	var cur int32 = 3
	w.Write(c, http.StatusConflict, "concurrency_conflict", nil, &cur)

	var p problem.Problem
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &p))
	assert.Equal(t, "concurrency_conflict", p.Code)
	require.NotNil(t, p.CurrentVersion)
	assert.Equal(t, int32(3), *p.CurrentVersion)
}

// TestLocale_And_Sub asserts the context accessors: defaults when unset, the
// stored value when the middleware has set it.
func TestLocale_And_Sub(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	assert.Equal(t, config.DefaultLocale, problem.Locale(c), "unset locale defaults")
	assert.Equal(t, "", problem.Sub(c), "unset sub is empty")

	c.Set(problem.LocaleKey, "ja")
	c.Set(problem.SubKey, "ownerA")
	assert.Equal(t, "ja", problem.Locale(c))
	assert.Equal(t, "ownerA", problem.Sub(c))
}
