// Command service (write side) of the accounting CQRS system. It exposes
// POST/PUT/DELETE /command/journal-entries plus a public health check, guarded
// by OIDC auth, content-type, i18n, and Idempotency-Key middleware. Config is
// Vault-rendered env; nothing here embeds secrets.
package main

import (
	"context"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"

	"github.com/thee5176/reckonna/internal/config"
	cmdhttp "github.com/thee5176/reckonna/internal/handler/command"
	"github.com/thee5176/reckonna/internal/handler/middleware"
	"github.com/thee5176/reckonna/internal/handler/problem"
	"github.com/thee5176/reckonna/internal/metrics"
	"github.com/thee5176/reckonna/internal/service"
)

func main() {
	ctx := context.Background()
	cfg := config.Load("command")

	shutdown, err := config.SetupTelemetry(ctx, cfg)
	must(err, "setup telemetry")
	defer func() { _ = shutdown(context.Background()) }()
	must(metrics.Init(), "init metrics")

	pool, err := config.NewPool(ctx, cfg.DatabaseURL)
	must(err, "connect db")
	defer pool.Close()

	bundle, err := config.LoadBundle(config.LocalesDir())
	must(err, "load locales")
	pw := problem.NewWriter(bundle)

	auth, err := config.NewAuthenticator(ctx, config.OIDCConfig{IssuerURL: cfg.OIDCIssuer, Audience: cfg.OIDCAudience})
	must(err, "oidc discovery")

	h := cmdhttp.NewHandler(service.NewLedgerCommandService(pool), pw, pool)

	r := gin.New()
	r.Use(gin.Recovery(), otelgin.Middleware("reckonna-command"), middleware.Metrics())
	r.GET("/command/health", health)

	api := r.Group("")
	api.Use(middleware.Locale(), middleware.RequireJSON(pw), middleware.Auth(auth, pw))
	api.POST("/command/journal-entries", h.Idempotency(), h.Post)
	api.PUT("/command/journal-entries/:id", h.Put)
	api.DELETE("/command/journal-entries/:id", h.Delete)

	log.Printf("command service listening on :%s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("command service stopped: %v", err)
	}
}

func health(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "command"}) }

func must(err error, what string) {
	if err != nil {
		log.Fatalf("%s: %v", what, err)
	}
}
