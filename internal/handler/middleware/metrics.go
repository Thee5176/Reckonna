package middleware

import (
	"github.com/gin-gonic/gin"

	"github.com/thee5176/reckonna/internal/metrics"
)

// Metrics records one reckonna.http.server.requests count per request, labelled
// with method, matched route, and status — the RED signal for the dashboard.
// Uses the matched route (c.FullPath) rather than the raw path so cardinality
// stays bounded. Neutral: both routers use it without breaking IT9.
func Metrics() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}
		metrics.RecordHTTPRequest(c.Request.Context(), c.Request.Method, route, c.Writer.Status())
	}
}
