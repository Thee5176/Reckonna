// Package middleware holds neutral HTTP middleware shared by both the command
// and query routers: content negotiation, i18n locale resolution, and OIDC auth.
// It imports neither the command nor the query side, so importing it from
// cmd/query does not break compile-time CQRS purity (IT9).
package middleware

import (
	"mime"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/thee5176/reckonna/internal/handler/problem"
)

// RequireJSON rejects POST/PUT/PATCH requests whose Content-Type is not
// application/json with 415, before any body parsing (AT17, IT15). Other
// methods (GET/DELETE/OPTIONS) pass through.
func RequireJSON(pw *problem.Writer) gin.HandlerFunc {
	return func(c *gin.Context) {
		switch c.Request.Method {
		case http.MethodPost, http.MethodPut, http.MethodPatch:
			ct := c.GetHeader("Content-Type")
			mediaType, _, err := mime.ParseMediaType(ct)
			if err != nil || !strings.EqualFold(mediaType, "application/json") {
				pw.Write(c, http.StatusUnsupportedMediaType, "unsupported_media_type", nil, nil)
				return
			}
		}
		c.Next()
	}
}
