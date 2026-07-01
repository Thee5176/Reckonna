// Package testsupport spins a Postgres for integration tests. It prefers a
// container (testcontainers-go) so CI is hermetic, but reuses an existing DB
// when RECKONNA_TEST_DATABASE_URL is set (fast local iteration). Each call
// returns a freshly-migrated pool: the public schema is dropped and every
// db/migration/*.up.sql is applied in order, so tests never share state.
package testsupport

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/thee5176/reckonna/internal/config"
)

// NewPostgres returns a migrated pgx pool for a test, cleaning up on completion.
func NewPostgres(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()

	dsn := os.Getenv("RECKONNA_TEST_DATABASE_URL")
	if dsn == "" {
		dsn = startContainer(t, ctx)
	}

	pool, err := config.NewPool(ctx, dsn)
	require.NoError(t, err, "open pool")
	t.Cleanup(pool.Close)

	require.NoError(t, resetAndMigrate(ctx, pool), "migrate")
	return pool
}

func startContainer(t *testing.T, ctx context.Context) string {
	t.Helper()
	pgC, err := postgres.Run(ctx, "docker.io/library/postgres:17-alpine",
		postgres.WithDatabase("acct"),
		postgres.WithUsername("postgres"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForListeningPort("5432/tcp").WithStartupTimeout(90*time.Second)),
	)
	require.NoError(t, err, "start postgres container")
	t.Cleanup(func() { _ = pgC.Terminate(ctx) })

	dsn, err := pgC.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	return dsn
}

// resetAndMigrate drops the schema and re-applies all up migrations so each test
// starts clean. Multi-statement files run via the simple protocol (no args).
func resetAndMigrate(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, "DROP SCHEMA public CASCADE; CREATE SCHEMA public;"); err != nil {
		return fmt.Errorf("reset schema: %w", err)
	}
	dir := filepath.Join(repoRoot(), "db", "migration")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read migrations: %w", err)
	}
	var ups []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".up.sql") {
			ups = append(ups, e.Name())
		}
	}
	sort.Strings(ups)
	for _, name := range ups {
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return fmt.Errorf("read %s: %w", name, err)
		}
		if _, err := pool.Exec(ctx, string(b)); err != nil {
			return fmt.Errorf("apply %s: %w", name, err)
		}
	}
	return nil
}

// repoRoot walks up to the module root (dir with go.mod).
func repoRoot() string {
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			panic("go.mod not found from CWD")
		}
		dir = parent
	}
}
