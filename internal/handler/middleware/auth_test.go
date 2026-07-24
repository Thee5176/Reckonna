package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thee5176/reckonna/internal/config"
	"github.com/thee5176/reckonna/internal/handler/middleware"
	"github.com/thee5176/reckonna/internal/handler/problem"
	"github.com/thee5176/reckonna/internal/testsupport"
)

const testAudience = "reckonna-api"
const testKID = "test-key"

func protectedRouter(t *testing.T, iss string) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	auth, err := config.NewAuthenticator(context.Background(), config.OIDCConfig{IssuerURL: iss, Audience: testAudience})
	require.NoError(t, err)
	bundle, err := config.LoadBundle(config.LocalesDir())
	require.NoError(t, err)
	pw := problem.NewWriter(bundle)

	r := gin.New()
	r.Use(middleware.Auth(auth, pw))
	r.GET("/protected", func(c *gin.Context) { c.String(http.StatusOK, problem.Sub(c)) })
	return r
}

func call(r *gin.Engine, bearer string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestAuth_ValidToken(t *testing.T) {
	iss, priv := testsupport.MockOIDCIssuer(t, testKID)
	r := protectedRouter(t, iss)

	tok := testsupport.MintJWT(t, priv, testKID, iss, testAudience, "user-1", time.Now().Add(time.Hour))
	w := call(r, tok)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "user-1", w.Body.String(), "sub is extracted onto context")
}

func TestAuth_Rejects(t *testing.T) {
	iss, priv := testsupport.MockOIDCIssuer(t, testKID)
	r := protectedRouter(t, iss)
	_, otherKey := testsupport.MockOIDCIssuer(t, testKID) // an unrelated key for the bad-signature case

	tests := []struct {
		name  string
		token string
	}{
		{"missing header", ""},
		{"wrong audience", testsupport.MintJWT(t, priv, testKID, iss, "someone-else", "u", time.Now().Add(time.Hour))},
		{"expired", testsupport.MintJWT(t, priv, testKID, iss, testAudience, "u", time.Now().Add(-time.Hour))},
		{"bad signature", testsupport.MintJWT(t, otherKey, testKID, iss, testAudience, "u", time.Now().Add(time.Hour))},
		{"wrong issuer", testsupport.MintJWT(t, priv, testKID, "https://evil.example", testAudience, "u", time.Now().Add(time.Hour))},
		{"garbage", "not-a-jwt"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := call(r, tt.token)
			require.Equal(t, http.StatusUnauthorized, w.Code)
			assert.Contains(t, w.Header().Get("Content-Type"), "application/problem+json")
			assert.Contains(t, w.Body.String(), "unauthorized")
		})
	}
}
