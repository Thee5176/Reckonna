package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/thee5176/reckonna/internal/handler/problem"
)

// TokenVerifier validates a bearer token and returns its owner id (sub).
// *config.Authenticator implements it; tests can inject a fake. Keeping the
// middleware behind this interface (rather than importing config directly) also
// keeps it neutral for CQRS purity.
type TokenVerifier interface {
	Verify(ctx context.Context, rawToken string) (string, error)
}

// Auth validates the Authorization: Bearer <jwt> header against the OIDC
// provider (JWKS signature + issuer + audience + expiry) and stores the sub on
// the context for owner scoping. Any failure → 401 (IT4, AT8). Health endpoints
// are not wrapped with this middleware, so they stay public.
//
// 404-vs-403 policy (enumeration defense, T16): once authenticated, a *read* for
// a resource owned by someone else returns 404 (not 403) so existence is not
// leaked. Write ops (PUT/DELETE) on a known-but-unowned resource return 403 per
// AT4. This split lives in the query vs command handlers; auth only supplies sub.
func Auth(v TokenVerifier, pw *problem.Writer) gin.HandlerFunc {
	return func(c *gin.Context) {
		authz := c.GetHeader("Authorization")
		const prefix = "Bearer "
		if len(authz) <= len(prefix) || !strings.EqualFold(authz[:len(prefix)], prefix) {
			pw.Write(c, http.StatusUnauthorized, "unauthorized", nil, nil)
			return
		}
		raw := strings.TrimSpace(authz[len(prefix):])
		sub, err := v.Verify(c.Request.Context(), raw)
		if err != nil || sub == "" {
			pw.Write(c, http.StatusUnauthorized, "unauthorized", nil, nil)
			return
		}
		c.Set(problem.SubKey, sub)
		c.Next()
	}
}
