# Reference — Backend Validation (Plan 03 CQRS Core)

Complete, factual description of the plan-03 backend validation surface: the
acceptance/integration criteria, the domain invariant errors, how each maps to
an HTTP status, and the commands that prove them. Derived from code on
`feat/03-backend-cqrs-core`.

- Criteria ledger: [`qa-plan-03.md`](qa-plan-03.md) · Run status: [`validation-status.md`](validation-status.md)
- Why the balance invariant works this way: [explanation-balance-invariant.md](explanation-balance-invariant.md)
- How to run the suites: [howto-run-backend-validation.md](howto-run-backend-validation.md)

## Domain invariant errors

Defined in `internal/domain/journal_entry.go`. `NewEntry` is the only
constructor of a valid `JournalEntry`; it runs guards in order and returns the
first failure. Callers match with `errors.Is`.

| Error | Guard (func) | Meaning |
|-------|--------------|---------|
| `ErrNoLines` | `NewEntry` | entry has zero postings |
| `ErrExcessivePrecision` | `validateLines` | a line amount is finer than `NUMERIC(20,4)` (would be rounded on INSERT) |
| `ErrRequiredDimension` | `validateLines` | a line omits a dimension its account declares required (§7 R7.4) |
| `ErrMixedCurrency` | `requireSingleCurrency` | lines disagree on their `currency` dimension (v1 entries are single-currency) |
| `ErrUnbalanced` | `requireBalanced` | Σ debit ≠ Σ credit (借方 ≠ 貸方) |

Guard order (from `NewEntry`): `len==0` → `validateLines` (precision, then
required-dimension) → `requireSingleCurrency` → `requireBalanced`. Sums use
`domain.Money` (arbitrary-precision decimal, never `float64`).

## Error → HTTP mapping

`internal/handler/command/errors.go` (`writeError`) classifies each error into an
RFC 7807 `application/problem+json` response. DB-layer defenses map to the same
codes as their domain equivalents.

| Error / SQLSTATE | HTTP | `code` |
|------------------|------|--------|
| `ErrUnbalanced` **or** trigger `23514` | 422 | `unbalanced_entry` (with `errors[0]{line_index,field:"amount",issue:"debit_credit_mismatch"}`) |
| `ErrMixedCurrency` | 422 | `mixed_currency` |
| `ErrRequiredDimension` | 422 | `missing_required_dimension` |
| `ErrUnknownAccountCode` **or** FK `23503` | 422 | `unknown_account_code` |
| `ErrNoLines`, `ErrBookNotFound`, `ErrUnknownDimension`, **`ErrExcessivePrecision`** | 422 | `validation_failed` |
| `ErrEntryNotFound` | 404 | `not_found` |
| `ErrForbidden` | 403 | `forbidden` |
| `VersionConflictError` | 409 | `concurrency_conflict` (+ `current_version`) |
| syntactic (`errBadRequest`) | 400 | `validation_failed` |

Tests assert on `code` + `status`, never on localized `title`/`detail`.

## Money policy (NUMERIC(20,4))

- Storage: `journal_line.amount` is `numeric(20, 4)`, `CHECK (amount >= 0)`.
- Domain: `Money` wraps `shopspring/decimal.Decimal` at full precision.
  `Money.TooPrecise()` reports whether rounding to 4dp changes the value.
- Policy (Option B, IT18): amounts finer than 4dp are **rejected** (422
  `validation_failed`), never silently rounded. Trailing zeros beyond 4dp are
  accepted (no significance lost).

## Acceptance criteria (E2E) — plan §1

Committed under `e2e/` (`//go:build e2e`). AT5/AT9 proven at service/handler
layer (stronger row-level assertions than e2e).

| AT | Requirement | Test |
|----|-------------|------|
| AT1 | balanced create → persist+read, 借方=貸方 | `TestE2E_CreateAndRead` |
| AT2 | unbalanced → 422 `unbalanced_entry` | `TestE2E_SemanticRejections` |
| AT3/AT4 | owner scope: cross-owner GET→404, PUT/DELETE→403 | `TestE2E_OwnerScope` |
| AT5 | DELETE cascades lines (rows==0) | `internal/service/ledger_command_test.go` |
| AT6/AT7 | balance-sheet / profit-loss | `TestE2E_Statements` |
| AT8 | no/invalid JWT → 401 | `TestE2E_Unauthorized` |
| AT9 | CoA list (20 accounts) | `internal/handler/query/handler_test.go` |
| AT10 | unknown account → 422 | `TestE2E_SemanticRejections` |
| AT11 | 4dp exact accepted + stable; >4dp rejected (Option B) | `TestE2E_MoneyPrecision` |
| AT13/AT14 | single-currency USD ok; mixed → 422 `mixed_currency` | `TestE2E_CurrencyDimension` |
| AT15/AT15b, AT16/AT16b, AT17, AT18 | idempotency, concurrency, 415, i18n | handler/contract tests |

## Integration criteria — plan §2

| IT | Condition | Location |
|----|-----------|----------|
| IT1 | command.Post → query.Get same id | `internal/query` |
| IT2 | tx fails mid-write → atomic rollback | `internal/service` |
| IT3 | DB trigger rejects unbalanced (domain bypassed) | `internal/repository/balance_trigger_test.go` |
| IT4 | OIDC JWKS bad sig/iss/aud/exp → 401 | `internal/handler/middleware/auth_test.go` |
| IT5 | reads owner-filtered | `internal/query` |
| IT7 | statement aggregates group by CoA | `internal/handler/query` |
| IT8 | migration up→down→up idempotent + trigger present | `internal/testsupport/migrate_test.go` |
| IT9 | cmd/query read-only (compile-time, go/parser) | `internal/query/readonly_test.go` |
| IT12 | required dimension (counterparty on 21500) → 422 | handler test |
| IT13 | per-(entry,book) balance + mixed-currency reject | `internal/repository` |
| IT14 | every CoA code + error code has en+ja | `internal/config/i18n_coverage_test.go` |
| IT15 | Content-Type middleware pre-handler | `internal/handler/command` |
| IT16 | idempotency UNIQUE(key,owner_sub) + replay + TTL | handler test |
| IT17 | version trigger bumps on UPDATE → ETag | service test |
| IT18 | domain ∧ DB trigger agree at 4dp (reject >4dp) | `internal/domain/journal_entry_test.go` + handler 422 test |

## Run commands

```bash
make test                                      # unit + IT (go test ./... -race)
go test -tags e2e ./e2e/... -count=1           # acceptance (needs Docker/podman)
```

Test DB resolves in order: `RECKONNA_TEST_DATABASE_URL` (caller) →
`scripts/render-test-db-url.sh` (Vault shared PG, schema-isolated) →
testcontainers-go (needs `DOCKER_HOST`). See the how-to for setup.

## Related
- [explanation-balance-invariant.md](explanation-balance-invariant.md) — why the two-layer 4dp check
- [howto-run-backend-validation.md](howto-run-backend-validation.md) — running the suites
- [qa-plan-03.md](qa-plan-03.md) · [validation-status.md](validation-status.md)
