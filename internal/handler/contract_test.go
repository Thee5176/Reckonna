// Package handler_test validates the real command + query handlers against
// api/openapi.yaml using kin-openapi (openapi3 + openapi3filter +
// routers/gorillamux). It drives the actual Gin handlers via
// net/http/httptest (no mocks) so the spec is checked against genuine
// request/response pairs, not hand-built fixtures. Complements
// internal/handler/command/journal_entry_test.go and
// internal/handler/query/handler_test.go, which cover business-rule detail
// this file does not re-assert.
package handler_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thee5176/reckonna/internal/config"
	"github.com/thee5176/reckonna/internal/handler/command"
	"github.com/thee5176/reckonna/internal/handler/middleware"
	"github.com/thee5176/reckonna/internal/handler/problem"
	qhttp "github.com/thee5176/reckonna/internal/handler/query"
	qsvc "github.com/thee5176/reckonna/internal/query"
	"github.com/thee5176/reckonna/internal/service"
	"github.com/thee5176/reckonna/internal/testsupport"
)

const cBalanced = `{"date":"2025-01-01","description":"sale","lines":[
  {"account_code":10000,"side":"debit","amount":"1000.0000","dimensions":{"currency":"JPY"}},
  {"account_code":40000,"side":"credit","amount":"1000.0000","dimensions":{"currency":"JPY"}}]}`

const cUnbalanced = `{"date":"2025-01-01","description":"bad","lines":[
  {"account_code":10000,"side":"debit","amount":"1000.0000","dimensions":{"currency":"JPY"}},
  {"account_code":40000,"side":"credit","amount":"500.0000","dimensions":{"currency":"JPY"}}]}`

var (
	specOnce   sync.Once
	specDoc    *openapi3.T
	specRouter routers.Router
)

// loadSpec parses + validates api/openapi.yaml once and builds the
// gorillamux router used to match requests to operations. moduleRoot walks up
// from the working directory to the nearest go.mod, same trick as
// config.LocalesDir, so this works regardless of which package `go test` runs
// from.
func loadSpec(t *testing.T) (*openapi3.T, routers.Router) {
	t.Helper()
	specOnce.Do(func() {
		loader := openapi3.NewLoader()
		doc, err := loader.LoadFromFile(filepath.Join(moduleRoot(), "api", "openapi.yaml"))
		require.NoError(t, err, "load api/openapi.yaml")
		require.NoError(t, doc.Validate(context.Background()), "api/openapi.yaml is not a valid OpenAPI document")
		r, err := gorillamux.NewRouter(doc)
		require.NoError(t, err, "build spec router")
		specDoc, specRouter = doc, r
	})
	require.NotNil(t, specDoc, "spec failed to load in an earlier test")
	return specDoc, specRouter
}

func moduleRoot() string {
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "."
		}
		dir = parent
	}
}

// newContractRouter mirrors the route registration in cmd/command/main.go +
// cmd/query/main.go (health public, everything else behind Locale +
// RequireJSON + owner scoping) on a single engine over a shared pool, so a
// test can create via /command and read via /query. Auth is a header stub
// (X-Test-Sub) exactly like journal_entry_test.go and query/handler_test.go —
// the real OIDC verifier is covered by middleware/auth_test.go, not here.
func newContractRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	pool := testsupport.NewPostgres(t)
	bundle, err := config.LoadBundle(config.LocalesDir())
	require.NoError(t, err)
	pw := problem.NewWriter(bundle)

	cmdH := command.NewHandler(service.NewLedgerCommandService(pool), pw, pool)
	qH := qhttp.NewHandler(qsvc.NewService(pool), pw)

	r := gin.New()
	r.GET("/command/health", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "command"}) })
	r.GET("/query/health", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "query"}) })

	api := r.Group("")
	api.Use(middleware.Locale(), middleware.RequireJSON(pw))
	api.Use(func(c *gin.Context) { c.Set(problem.SubKey, c.GetHeader("X-Test-Sub")); c.Next() })
	api.POST("/command/journal-entries", cmdH.Idempotency(), cmdH.Post)
	api.PUT("/command/journal-entries/:id", cmdH.Put)
	api.DELETE("/command/journal-entries/:id", cmdH.Delete)
	qH.Register(api)
	return r
}

// call drives path through r as sub, then checks the response against the
// spec operation matched for method+path. When checkRequest is true it also
// asserts the outgoing request itself is spec-valid (skip this for
// deliberately protocol-malformed requests, e.g. the 415 case, where a
// request-validation error is the expected outcome, not a test bug).
func call(t *testing.T, r http.Handler, method, path, sub, body string, headers map[string]string, checkRequest bool) *httptest.ResponseRecorder {
	t.Helper()
	_, oaRouter := loadSpec(t)

	findReq := httptest.NewRequest(method, path, nil)
	route, params, err := oaRouter.FindRoute(findReq)
	require.NoErrorf(t, err, "no api/openapi.yaml route matches %s %s", method, path)

	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("X-Test-Sub", sub)
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	if checkRequest {
		valReq := req.Clone(req.Context())
		valReq.Body = http.NoBody
		if body != "" {
			valReq.Body = io.NopCloser(strings.NewReader(body))
		}
		reqErr := openapi3filter.ValidateRequest(context.Background(), &openapi3filter.RequestValidationInput{
			Request: valReq, PathParams: params, Route: route,
			Options: &openapi3filter.Options{AuthenticationFunc: openapi3filter.NoopAuthenticationFunc},
		})
		assert.NoErrorf(t, reqErr, "request %s %s does not match api/openapi.yaml", method, path)
	}

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	respInput := &openapi3filter.ResponseValidationInput{
		RequestValidationInput: &openapi3filter.RequestValidationInput{Request: req, PathParams: params, Route: route},
		Status:                 w.Code,
		Header:                 w.Header(),
	}
	respInput.SetBodyBytes(w.Body.Bytes())
	respErr := openapi3filter.ValidateResponse(context.Background(), respInput)
	assert.NoErrorf(t, respErr, "response %s %s -> %d does not match api/openapi.yaml:\n%s", method, path, w.Code, w.Body.String())

	return w
}
