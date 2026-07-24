// Package service holds use cases and transaction orchestration. It owns the
// unit-of-work boundary (the service begins/commits the tx); repositories
// operate on the tx-bound sqlc Queries. No HTTP or DTO types leak in here.
package service

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/thee5176/reckonna/internal/repository/command"
)

// TxBeginner is the subset of *pgxpool.Pool the service needs. Narrowing to an
// interface lets tx orchestration be unit-tested with a fake.
type TxBeginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

// WithinTx runs fn inside a single transaction: commit on success, rollback on
// any error (or panic). The tx-bound *command.Queries is what fn writes through,
// so a mid-step failure leaves NO partial rows — the deferred balance trigger
// then validates the whole entry at COMMIT (IT2, IT3).
func WithinTx(ctx context.Context, pool TxBeginner, fn func(q *command.Queries) error) (err error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx)
			panic(p)
		}
	}()

	if err = fn(command.New(tx)); err != nil {
		if rbErr := tx.Rollback(ctx); rbErr != nil {
			return fmt.Errorf("%w (rollback also failed: %v)", err, rbErr)
		}
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}
