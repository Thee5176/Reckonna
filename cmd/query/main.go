// Query service (read side) of the accounting CQRS system. It exposes the
// read-only /query/* endpoints plus a public health check, guarded by OIDC auth
// and i18n. It imports ONLY the read model + neutral middleware — never the
// command side — which readonly_test verifies at the package-import level (IT9).
package main

import (
	"context"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"

	"github.com/thee5176/reckonna/internal/config"
	"github.com/thee5176/reckonna/internal/handler/middleware"
	"github.com/thee5176/reckonna/internal/handler/problem"
	qhttp "github.com/thee5176/reckonna/internal/handler/query"
	query "github.com/thee5176/reckonna/internal/query"
)

func main() {
	ctx := context.Background()
	cfg := config.Load("query")

	shutdown, err := config.SetupTracing(ctx, cfg)
	must(err, "setup tracing")
	defer func() { _ = shutdown(context.Background()) }()

	pool, err := config.NewPool(ctx, cfg.DatabaseURL)
	must(err, "connect db")
	defer pool.Close()

	bundle, err := config.LoadBundle(config.LocalesDir())
	must(err, "load locales")
	pw := problem.NewWriter(bundle)

	auth, err := config.NewAuthenticator(ctx, config.OIDCConfig{IssuerURL: cfg.OIDCIssuer, Audience: cfg.OIDCAudience})
	must(err, "oidc discovery")

	h := qhttp.NewHandler(query.NewService(pool), pw)

	r := gin.New()
	r.Use(gin.Recovery(), otelgin.Middleware("reckonna-query"))
	r.GET("/query/health", health)

	api := r.Group("")
	api.Use(middleware.Locale(), middleware.Auth(auth, pw))
	h.Register(api)

	log.Printf("query service listening on :%s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("query service stopped: %v", err)
	}
}

func health(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "query"}) }

func must(err error, what string) {
	if err != nil {
		log.Fatalf("%s: %v", what, err)
	}
}
