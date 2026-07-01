// Package e2e exercises the fully-wired command + query routers end to end:
// real OIDC auth (against a mock JWKS issuer), real middleware chain, real
// services, and a real Postgres (testcontainers-go, or RECKONNA_TEST_DATABASE_URL
// for local reuse) with the actual migrations + balance trigger. These are the
// AT-level acceptance tests; the DB-level invariant is exercised, not mocked.
package e2e

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"

	"github.com/thee5176/reckonna/internal/config"
	cmdhttp "github.com/thee5176/reckonna/internal/handler/command"
	"github.com/thee5176/reckonna/internal/handler/middleware"
	"github.com/thee5176/reckonna/internal/handler/problem"
	qhttp "github.com/thee5176/reckonna/internal/handler/query"
	qsvc "github.com/thee5176/reckonna/internal/query"
	"github.com/thee5176/reckonna/internal/service"
	"github.com/thee5176/reckonna/internal/testsupport"
)

const (
	e2eAudience = "reckonna-api"
	e2eKID      = "e2e-key"
)

// harness is a fully-wired stack under test.
type harness struct {
	router *gin.Engine
	priv   *rsa.PrivateKey
	issuer string
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	gin.SetMode(gin.TestMode)

	issuer, priv := mockIssuer(t)
	pool := testsupport.NewPostgres(t)

	bundle, err := config.LoadBundle(config.LocalesDir())
	require.NoError(t, err)
	pw := problem.NewWriter(bundle)
	auth, err := config.NewAuthenticator(context.Background(),
		config.OIDCConfig{IssuerURL: issuer, Audience: e2eAudience})
	require.NoError(t, err)

	ch := cmdhttp.NewHandler(service.NewLedgerCommandService(pool), pw, pool)
	qh := qhttp.NewHandler(qsvc.NewService(pool), pw)

	r := gin.New()
	r.Use(gin.Recovery())
	r.GET("/command/health", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok"}) })
	r.GET("/query/health", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok"}) })

	api := r.Group("")
	api.Use(middleware.Locale(), middleware.RequireJSON(pw), middleware.Auth(auth, pw))
	api.POST("/command/journal-entries", ch.Idempotency(), ch.Post)
	api.PUT("/command/journal-entries/:id", ch.Put)
	api.DELETE("/command/journal-entries/:id", ch.Delete)
	qh.Register(api)

	return &harness{router: r, priv: priv, issuer: issuer}
}

// token mints a valid access token for the given sub.
func (h *harness) token(t *testing.T, sub string) string {
	return mint(t, h.priv, e2eKID, h.issuer, e2eAudience, sub, time.Now().Add(time.Hour))
}

// req performs an authenticated request and returns the recorder.
func (h *harness) req(t *testing.T, method, path, sub, body string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if sub != "" {
		req.Header.Set("Authorization", "Bearer "+h.token(t, sub))
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	h.router.ServeHTTP(w, req)
	return w
}

// ---- mock OIDC issuer (RSA + discovery + JWKS) ----

func mockIssuer(t *testing.T) (string, *rsa.PrivateKey) {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	mux := http.NewServeMux()
	var issuer string
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"issuer":%q,"jwks_uri":%q,"id_token_signing_alg_values_supported":["RS256"]}`,
			issuer, issuer+"/jwks")
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		n := base64.RawURLEncoding.EncodeToString(priv.PublicKey.N.Bytes())
		e := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(priv.PublicKey.E)).Bytes())
		fmt.Fprintf(w, `{"keys":[{"kty":"RSA","use":"sig","alg":"RS256","kid":%q,"n":%q,"e":%q}]}`, e2eKID, n, e)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	issuer = srv.URL
	return issuer, priv
}

func mint(t *testing.T, priv *rsa.PrivateKey, kid, iss, aud, sub string, exp time.Time) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"iss": iss, "aud": aud, "sub": sub, "exp": exp.Unix(), "iat": time.Now().Unix(),
	})
	tok.Header["kid"] = kid
	s, err := tok.SignedString(priv)
	require.NoError(t, err)
	return s
}

// entryJSON builds a single-currency entry body (debit/credit pair).
func entryJSON(debitCode, creditCode int, amount, currency string) string {
	return fmt.Sprintf(`{"date":"2025-01-01","description":"e2e","lines":[
	  {"account_code":%d,"side":"debit","amount":%q,"dimensions":{"currency":%q}},
	  {"account_code":%d,"side":"credit","amount":%q,"dimensions":{"currency":%q}}]}`,
		debitCode, amount, currency, creditCode, amount, currency)
}
