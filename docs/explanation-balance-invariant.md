# Explanation — The Balance Invariant Across Two Layers

Why a journal entry's 借方=貸方 (debit=credit) rule is enforced in **both** the Go
domain and the PostgreSQL trigger, and why amounts finer than 4 decimal places
are rejected rather than rounded.

Reference for the concrete errors/tests: [reference-backend-validation.md](reference-backend-validation.md).

## The problem

Double-entry accounting has one non-negotiable rule: within a journal entry, the
sum of debits equals the sum of credits. Break it and the books do not balance.

The system enforces this in two places (defense in depth):

1. **Domain** — `domain.NewEntry` sums `Money` (arbitrary-precision decimal) and
   compares exactly (`requireBalanced`).
2. **Database** — a deferred `CONSTRAINT TRIGGER` (`trg_entry_balanced`) sums
   `journal_line.amount` at COMMIT and raises SQLSTATE `23514` if debit ≠ credit.

The trap: the two layers do arithmetic at **different precision**. The domain is
full precision; the column is `NUMERIC(20,4)`, so Postgres rounds each amount to
4 decimal places on INSERT. When those policies disagree, the layers disagree.

### The divergence, concretely

```
lines            debit: 0.12344  +  0.12341        credit: 0.24685

domain (full precision)   0.24685 == 0.24685        → BALANCED  (NewEntry accepts)
DB numeric(20,4) stored   0.1234 + 0.1234 = 0.2468  vs  0.2469  → UNBALANCED (trigger rejects, 23514)
```

The domain accepts an entry the database then rejects at COMMIT. No bad data
persists (the trigger backstops it into a 422), but the two guardians of the same
invariant give different verdicts on the same input. The reverse also exists: an
entry unbalanced at full precision but balanced after 4dp rounding would be
rejected by the domain yet accepted by a bypassed DB.

The acceptance suite did not catch this: `TestE2E_MoneyPrecision` (AT11)
originally used **symmetric** entries — the same amount on both sides — which
round identically, so they never expose the asymmetry.

## The approach — reject, don't reconcile (Option B)

Two ways to make the layers agree:

- **A — Quantize:** have the domain round every amount to 4dp before comparing,
  matching the DB. Silently accepts sub-cent input and rounds it.
- **B — Reject (chosen):** reject any amount whose 4dp rounding would change its
  value, up front, as a `422 validation_failed`. No sub-4dp amount ever reaches
  either balance check, so both always agree.

Option B is implemented by `Money.TooPrecise()` (`internal/domain/money.go`):

```go
func (m Money) TooPrecise() bool { return !m.amount.Equal(m.amount.Round(4)) }
```

`validateLines` calls it before the balance check and returns
`ErrExcessivePrecision`; the command handler maps that to `422 validation_failed`.
Trailing zeros beyond 4dp (`100.30000`) carry no significance and pass.

```
POST amount 1000.33335  ──► DTO → domain.NewEntry
                              validateLines: TooPrecise? yes
                              → ErrExcessivePrecision
                              → 422 validation_failed   (never persisted)
POST amount 1000.3333   ──► accepted, stored exactly, stable round-trip
```

`TestE2E_MoneyPrecision` now asserts this: `1000.3333` → 201 (stored
`1000.3333`), `1000.33335` → 422, `0.12345` → 422.

## Trade-offs

- **Chosen (B) gives up silent convenience.** A client sending 5+ decimal places
  gets a 422 and must round to 4dp itself. In exchange, money is never silently
  altered and the two invariant layers can never disagree — the property IT18
  asserts.
- **Rejected (A) would have hidden precision loss.** Rounding `1000.33335` to
  `1000.3334` without telling the caller is exactly the kind of quiet money
  mutation an accounting system must not do.
- **Defense in depth is kept, not replaced.** The DB trigger still fires: if the
  domain guard were ever bypassed (raw SQL, a new code path), the deferred
  trigger rejects an unbalanced COMMIT. IT3 proves this with raw inserts.

## Why the DB trigger is DEFERRABLE INITIALLY DEFERRED

A multi-line entry is transiently unbalanced mid-transaction (the first line is
in, the balancing line is not yet). A per-row check would reject it. The
constraint trigger is deferred to COMMIT, so the whole entry is validated once,
as a unit. See `db/migration/002_balance_check.up.sql`.

## Verification (IT18)

- Domain: `TestNewEntry_ExcessivePrecision` — the divergence case
  `0.12344 + 0.12341 == 0.24685` is rejected with `ErrExcessivePrecision`;
  exact-4dp and trailing-zero amounts are accepted.
- Handler: `TestPost_ExcessivePrecision_422` — a 5dp POST returns 422 and
  persists no row.
- DB backstop: `TestBalanceTrigger_RejectsUnbalanced` (IT3).

## Related
- [reference-backend-validation.md](reference-backend-validation.md) — the error/code tables
- [howto-run-backend-validation.md](howto-run-backend-validation.md) — running the proofs
