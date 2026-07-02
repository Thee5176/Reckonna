// Package testsupport spins a Postgres for integration tests. It prefers a
// container (testcontainers-go) so CI is hermetic, but reuses an existing DB
// when RECKONNA_TEST_DATABASE_URL is set (fast local iteration). Each call gets
// its OWN uniquely-named schema (via search_path) that is migrated fresh and
// dropped on cleanup — so tests, including parallel packages sharing one DB,
// never collide.
package testsupport

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/thee5176/reckonna/internal/config"
)

// NewPostgres returns a migrated pgx pool bound to a fresh, isolated schema.
func NewPostgres(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()

	baseDSN := os.Getenv("RECKONNA_TEST_DATABASE_URL")
	if baseDSN == "" {
		baseDSN = startContainer(t, ctx)
	}

	schema := "t_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	dsn, err := withSearchPath(baseDSN, schema)
	require.NoError(t, err)

	pool, err := config.NewPool(ctx, dsn)
	require.NoError(t, err, "open pool")
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	})

	require.NoError(t, migrateInto(ctx, pool, schema), "migrate")
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

// withSearchPath returns dsn with a libpq `options=-c search_path=<schema>` so
// every pooled connection resolves unqualified objects to the isolated schema.
func withSearchPath(dsn, schema string) (string, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return "", fmt.Errorf("parse dsn: %w", err)
	}
	q := u.Query()
	q.Set("options", "-c search_path="+schema)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// migrateInto creates the schema and applies every up migration in order.
// Multi-statement files run via the simple protocol (no args).
func migrateInto(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	if _, err := pool.Exec(ctx, "CREATE SCHEMA IF NOT EXISTS "+schema); err != nil {
		return fmt.Errorf("create schema: %w", err)
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
