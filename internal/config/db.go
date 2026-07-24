// Package config wires runtime configuration and infrastructure clients (DB
// pool, OIDC, OTel). Values come from Vault-rendered env at runtime — never
// hardcoded (see .claude/rules/secrets-vault.md).
package config

import (
	"context"
	"fmt"

	pgxdecimal "github.com/jackc/pgx-shopspring-decimal"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPool opens a pgx connection pool and registers the shopspring/decimal codec
// on every connection, so NUMERIC(20,4) columns scan into decimal.Decimal and
// no money value ever passes through float64. The DSN is supplied by the caller
// from Vault-rendered config; this package never embeds credentials.
func NewPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse dsn: %w", err)
	}
	cfg.AfterConnect = func(_ context.Context, conn *pgx.Conn) error {
		pgxdecimal.Register(conn.TypeMap())
		return nil
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("connect pool: %w", err)
	}
	return pool, nil
}
