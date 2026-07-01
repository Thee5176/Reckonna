package middleware_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thee5176/reckonna/internal/config"
	"github.com/thee5176/reckonna/internal/handler/middleware"
	"github.com/thee5176/reckonna/internal/handler/problem"
)

const testAudience = "reckonna-api"
const testKID = "test-key"

// mockIssuer stands up a minimal OIDC provider: discovery + JWKS signed by a
// known RSA key. Returns the issuer URL and the signing key.
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
		fmt.Fprint(w, jwksJSON(&priv.PublicKey, testKID))
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	issuer = srv.URL
	return issuer, priv
}

func jwksJSON(pub *rsa.PublicKey, kid string) string {
	n := base64.RawURLEncoding.EncodeToString(pub.N.Bytes())
	e := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes())
	return fmt.Sprintf(`{"keys":[{"kty":"RSA","use":"sig","alg":"RS256","kid":%q,"n":%q,"e":%q}]}`, kid, n, e)
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
	iss, priv := mockIssuer(t)
	r := protectedRouter(t, iss)

	tok := mint(t, priv, testKID, iss, testAudience, "user-1", time.Now().Add(time.Hour))
	w := call(r, tok)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "user-1", w.Body.String(), "sub is extracted onto context")
}

func TestAuth_Rejects(t *testing.T) {
	iss, priv := mockIssuer(t)
	r := protectedRouter(t, iss)
	_, otherKey := mockIssuer(t) // an unrelated key for the bad-signature case

	tests := []struct {
		name  string
		token string
	}{
		{"missing header", ""},
		{"wrong audience", mint(t, priv, testKID, iss, "someone-else", "u", time.Now().Add(time.Hour))},
		{"expired", mint(t, priv, testKID, iss, testAudience, "u", time.Now().Add(-time.Hour))},
		{"bad signature", mint(t, otherKey, testKID, iss, testAudience, "u", time.Now().Add(time.Hour))},
		{"wrong issuer", mint(t, priv, testKID, "https://evil.example", testAudience, "u", time.Now().Add(time.Hour))},
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
