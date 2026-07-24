// Package repository holds cross-cutting repository integration tests. This is a
// test-only package: it verifies DB-level guarantees that sit beneath the sqlc
// wrappers, notably that the deferred balance trigger rejects an unbalanced
// entry even when the Go domain check is bypassed (defense in depth, IT3).
package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thee5176/reckonna/internal/testsupport"
)

func accountID(t *testing.T, pool *pgxpool.Pool, code int) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		"SELECT id FROM account WHERE code=$1", code).Scan(&id))
	return id
}

// TestBalanceTrigger_RejectsUnbalanced inserts an unbalanced entry with RAW SQL
// (no domain.NewEntry), then commits. The deferred CONSTRAINT TRIGGER must fire
// at COMMIT with SQLSTATE 23514 and leave no rows (IT3).
func TestBalanceTrigger_RejectsUnbalanced(t *testing.T) {
	pool := testsupport.NewPostgres(t)
	ctx := context.Background()

	cashID := accountID(t, pool, 10000)
	revID := accountID(t, pool, 40000)

	var bookID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, "SELECT id FROM book WHERE code='base'").Scan(&bookID))

	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()

	entryID, _ := uuid.NewV7()
	_, err = tx.Exec(ctx,
		"INSERT INTO journal_entry (id, entry_date, owner_sub, book_id) VALUES ($1, current_date, 'raw', $2)",
		entryID, bookID)
	require.NoError(t, err)

	l1, _ := uuid.NewV7()
	l2, _ := uuid.NewV7()
	_, err = tx.Exec(ctx,
		"INSERT INTO journal_line (id, journal_entry_id, account_id, side, amount, line_no) VALUES ($1,$2,$3,'debit',1000,1)",
		l1, entryID, cashID)
	require.NoError(t, err)
	_, err = tx.Exec(ctx,
		"INSERT INTO journal_line (id, journal_entry_id, account_id, side, amount, line_no) VALUES ($1,$2,$3,'credit',500,2)",
		l2, entryID, revID)
	require.NoError(t, err, "inserts succeed mid-tx; the check is deferred to COMMIT")

	// COMMIT is where the deferred trigger runs.
	err = tx.Commit(ctx)
	require.Error(t, err, "unbalanced entry must fail at COMMIT")

	var pgErr *pgconn.PgError
	require.True(t, errors.As(err, &pgErr), "want a PgError, got %T", err)
	assert.Equal(t, "23514", pgErr.Code, "check_violation from the balance trigger")

	// No rows should remain.
	var n int
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM journal_entry WHERE owner_sub='raw'").Scan(&n))
	assert.Equal(t, 0, n)
}

// TestBalanceTrigger_AllowsBalanced confirms the trigger is not overzealous: a
// balanced raw insert commits.
func TestBalanceTrigger_AllowsBalanced(t *testing.T) {
	pool := testsupport.NewPostgres(t)
	ctx := context.Background()

	cashID := accountID(t, pool, 10000)
	revID := accountID(t, pool, 40000)
	var bookID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, "SELECT id FROM book WHERE code='base'").Scan(&bookID))

	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()

	entryID, _ := uuid.NewV7()
	_, err = tx.Exec(ctx, "INSERT INTO journal_entry (id, entry_date, owner_sub, book_id) VALUES ($1, current_date, 'rawok', $2)", entryID, bookID)
	require.NoError(t, err)
	l1, _ := uuid.NewV7()
	l2, _ := uuid.NewV7()
	_, err = tx.Exec(ctx, "INSERT INTO journal_line (id, journal_entry_id, account_id, side, amount, line_no) VALUES ($1,$2,$3,'debit',1000,1)", l1, entryID, cashID)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, "INSERT INTO journal_line (id, journal_entry_id, account_id, side, amount, line_no) VALUES ($1,$2,$3,'credit',1000,2)", l2, entryID, revID)
	require.NoError(t, err)

	require.NoError(t, tx.Commit(ctx), "balanced entry commits")
}
