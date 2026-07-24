package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thee5176/reckonna/internal/domain"
	"github.com/thee5176/reckonna/internal/service"
	"github.com/thee5176/reckonna/internal/testsupport"
)

// money parses a decimal literal into domain.Money, panicking on malformed
// input. Test literals are compile-time known-good, so RequireFromString (not
// t.Fatal) keeps callers one-liners.
func money(s string) domain.Money {
	return domain.NewMoney(decimal.RequireFromString(s))
}

func jpy(v string) map[domain.DimensionType]string {
	return map[domain.DimensionType]string{domain.DimCurrency: v}
}

func countEntries(t *testing.T, pool *pgxpool.Pool, owner string) int {
	t.Helper()
	var n int
	require.NoError(t, pool.QueryRow(context.Background(),
		"SELECT count(*) FROM journal_entry WHERE owner_sub=$1", owner).Scan(&n))
	return n
}

// balanced returns a valid single-currency entry: debit 10000 / credit 40000.
func balanced(owner string) service.EntryInput {
	return service.EntryInput{
		Date: time.Now(), Description: "sale", Owner: owner, Book: domain.BookBase,
		Lines: []service.LineInput{
			{AccountCode: 10000, Side: domain.SideDebit, Amount: money("1000.0000"), Dimensions: jpy("JPY")},
			{AccountCode: 40000, Side: domain.SideCredit, Amount: money("1000.0000"), Dimensions: jpy("JPY")},
		},
	}
}

// escrowEntry returns an entry crediting the escrow account (21500, requires
// counterparty) with the given counterparty value.
func escrowEntry(owner, counterparty string) service.EntryInput {
	return service.EntryInput{
		Date: time.Now(), Owner: owner, Book: domain.BookBase,
		Lines: []service.LineInput{
			{AccountCode: 10000, Side: domain.SideDebit, Amount: money("100"), Dimensions: jpy("JPY")},
			{AccountCode: 21500, Side: domain.SideCredit, Amount: money("100"),
				Dimensions: map[domain.DimensionType]string{domain.DimCurrency: "JPY", domain.DimCounterparty: counterparty}},
		},
	}
}

func TestPostLedger_BalanceAndValidation(t *testing.T) {
	pool := testsupport.NewPostgres(t)
	svc := service.NewLedgerCommandService(pool)
	ctx := context.Background()

	t.Run("balanced persists with version 1 (AT1)", func(t *testing.T) {
		id, ver, err := svc.PostLedger(ctx, balanced("ownerA"))
		require.NoError(t, err)
		assert.NotEqual(t, "00000000-0000-0000-0000-000000000000", id.String())
		assert.Equal(t, int32(1), ver)
		assert.Equal(t, 1, countEntries(t, pool, "ownerA"))
	})

	t.Run("unbalanced rejected, no rows (AT2)", func(t *testing.T) {
		in := balanced("ownerUB")
		in.Lines[1].Amount = money("500.0000")
		_, _, err := svc.PostLedger(ctx, in)
		require.ErrorIs(t, err, domain.ErrUnbalanced)
		assert.Equal(t, 0, countEntries(t, pool, "ownerUB"))
	})

	t.Run("unknown account code rejected (AT10)", func(t *testing.T) {
		in := balanced("ownerX")
		in.Lines[0].AccountCode = 99999
		_, _, err := svc.PostLedger(ctx, in)
		require.ErrorIs(t, err, service.ErrUnknownAccountCode)
	})

	t.Run("mixed currency rejected (AT14)", func(t *testing.T) {
		in := balanced("ownerMC")
		in.Lines[1].Dimensions = jpy("USD")
		_, _, err := svc.PostLedger(ctx, in)
		require.ErrorIs(t, err, domain.ErrMixedCurrency)
	})

	t.Run("missing required dimension rejected (IT12)", func(t *testing.T) {
		in := service.EntryInput{
			Date: time.Now(), Owner: "ownerRD", Book: domain.BookBase,
			Lines: []service.LineInput{
				{AccountCode: 10000, Side: domain.SideDebit, Amount: money("100"), Dimensions: jpy("JPY")},
				{AccountCode: 21500, Side: domain.SideCredit, Amount: money("100"), Dimensions: jpy("JPY")}, // escrow needs counterparty
			},
		}
		_, _, err := svc.PostLedger(ctx, in)
		require.ErrorIs(t, err, domain.ErrRequiredDimension)
	})

	t.Run("escrow with counterparty accepted", func(t *testing.T) {
		_, _, err := svc.PostLedger(ctx, escrowEntry("ownerESC", "external"))
		require.NoError(t, err)
	})
}

// TestPostLedger_AtomicRollback proves a mid-write failure leaves NO partial
// rows: an entry whose 2nd line references a non-existent counterparty value
// passes domain validation but fails inside the tx (IT2).
func TestPostLedger_AtomicRollback(t *testing.T) {
	pool := testsupport.NewPostgres(t)
	svc := service.NewLedgerCommandService(pool)
	ctx := context.Background()

	_, _, err := svc.PostLedger(ctx, escrowEntry("ownerATOM", "ghost"))
	require.Error(t, err)
	assert.Equal(t, 0, countEntries(t, pool, "ownerATOM"), "no partial rows after rollback")
}

func TestUpdateAndDeleteLedger(t *testing.T) {
	pool := testsupport.NewPostgres(t)
	svc := service.NewLedgerCommandService(pool)
	ctx := context.Background()

	id, ver, err := svc.PostLedger(ctx, balanced("ownerU"))
	require.NoError(t, err)
	require.Equal(t, int32(1), ver)

	t.Run("update bumps version (AT16 happy path, IT17)", func(t *testing.T) {
		in := balanced("ownerU")
		in.Description = "corrected"
		newVer, err := svc.UpdateLedger(ctx, id, 1, in)
		require.NoError(t, err)
		assert.Equal(t, int32(2), newVer)
	})

	t.Run("stale version conflict (AT16)", func(t *testing.T) {
		in := balanced("ownerU")
		_, err := svc.UpdateLedger(ctx, id, 1, in) // version is now 2
		var vc *service.VersionConflictError
		require.ErrorAs(t, err, &vc)
		assert.Equal(t, int32(2), vc.Current)
	})

	t.Run("cross-owner update forbidden (AT4)", func(t *testing.T) {
		in := balanced("intruder")
		_, err := svc.UpdateLedger(ctx, id, 2, in)
		require.ErrorIs(t, err, service.ErrForbidden)
	})

	t.Run("cross-owner delete forbidden (AT4)", func(t *testing.T) {
		require.ErrorIs(t, svc.DeleteLedger(ctx, id, "intruder"), service.ErrForbidden)
	})

	t.Run("owner delete cascades (AT5)", func(t *testing.T) {
		require.NoError(t, svc.DeleteLedger(ctx, id, "ownerU"))
		assert.Equal(t, 0, countEntries(t, pool, "ownerU"))
		var lines int
		require.NoError(t, pool.QueryRow(ctx,
			"SELECT count(*) FROM journal_line WHERE journal_entry_id=$1", id).Scan(&lines))
		assert.Equal(t, 0, lines, "lines cascade-deleted")
	})
}
