package testsupport

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thee5176/reckonna/internal/config"
)

// TestMigrations_UpDownUp_Idempotent applies every up migration, rolls them all
// back via the matching downs in reverse order, then re-applies the ups. It
// asserts each step succeeds (reversible + idempotent) and that the deferred
// balance CONSTRAINT TRIGGER is present after the second up (plan IT8). A broken
// or missing down migration fails here rather than silently on a later deploy.
func TestMigrations_UpDownUp_Idempotent(t *testing.T) {
	ctx := context.Background()

	baseDSN := os.Getenv("RECKONNA_TEST_DATABASE_URL")
	if baseDSN == "" {
		baseDSN = startContainer(t, ctx)
	}

	schema := "mig_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	dsn, err := withSearchPath(baseDSN, schema)
	require.NoError(t, err)

	pool, err := config.NewPool(ctx, dsn)
	require.NoError(t, err, "open pool")
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	})

	require.NoError(t, migrateInto(ctx, pool, schema), "first up")
	require.NoError(t, applyDowns(ctx, pool), "down (reverse)")
	require.NoError(t, migrateInto(ctx, pool, schema), "second up must be idempotent")

	// The deferred balance trigger must exist after the re-up, scoped to THIS
	// schema (every isolated test schema creates one, so filter by namespace).
	var n int
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT count(*) FROM pg_trigger tg
		  JOIN pg_class c ON c.oid = tg.tgrelid
		  JOIN pg_namespace ns ON ns.oid = c.relnamespace
		 WHERE tg.tgname = 'trg_entry_balanced' AND ns.nspname = $1`, schema).Scan(&n))
	assert.Equal(t, 1, n, "balance trigger present after up->down->up")
}

// applyDowns runs every *.down.sql in reverse filename order, mirroring the
// monotonic up sequence owned by the backend HEAD (006 down first, 001 last).
func applyDowns(ctx context.Context, pool *pgxpool.Pool) error {
	dir := filepath.Join(repoRoot(), "db", "migration")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read migrations: %w", err)
	}
	var downs []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".down.sql") {
			downs = append(downs, e.Name())
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(downs)))
	for _, name := range downs {
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
