# `e2e/` — End-to-End Acceptance Tests

Top of the V-model. These verify whole-system behavior against the **acceptance spec** in
`plans/03-backend-cqrs-core.md` (AT1–AT18) — the business-requirement layer.
## What lives here
- Go tests behind the `e2e` build tag (`//go:build e2e`), so a normal `go test ./...` skips them.
- Each test drives the real HTTP API of `cmd/command` + `cmd/query` against a **real Postgres**
  (testcontainers-go) with migrations + the deferred balance trigger applied — so the DB-level
  借方=貸方 invariant is exercised, not mocked.

## Mapping (plan 03)
Mirrors the AT1–AT18 acceptance spec in `plans/03-backend-cqrs-core.md`.

| Test file | Acceptance |
|-----------|-----------|
| `ledger_create.e2e_test.go` | AT1 balanced create |
| `journal_entry_invalid.e2e_test.go` | AT2 unbalanced → 422 (借方≠貸方) |
| `ledger_owner_scope.e2e_test.go` | AT3 owner scoping |
| `ledger_ownership.e2e_test.go` | AT4 non-owner → 403 |
| `ledger_delete.e2e_test.go` | AT5 cascade delete |
| `balance_sheet.e2e_test.go` / `profit_loss.e2e_test.go` | AT6 / AT7 statements |
| `auth_unauthorized.e2e_test.go` | AT8 401 |
| `coa_list.e2e_test.go` / `ledger_bad_coa.e2e_test.go` | AT9 / AT10 |
| `money_precision.e2e_test.go` | AT11 decimal exactness |
| _(deferred to plan 02)_ | AT12 Swagger UI at /docs — depends on S18/S19 |
| `entry_currency_dim.e2e_test.go` / `entry_mixed_currency.e2e_test.go` | AT13 / AT14 currency dimension + mixed-currency 422 |
| `idempotency_replay.e2e_test.go` / `idempotency_conflict.e2e_test.go` | AT15 / AT15b idempotency replay + duplicate-key 422 |
| `concurrency_conflict.e2e_test.go` / `concurrency_precondition.e2e_test.go` | AT16 / AT16b ETag/If-Match 409 + 428 |
| `content_type_415.e2e_test.go` | AT17 unsupported media type → 415 |
| `i18n_error.e2e_test.go` | AT18 locale-neutral `code` + Japanese title/detail |

## Run
```bash
go test -tags e2e ./e2e/... -count=1        # needs Docker (testcontainers)
```
Auth: tests mint OIDC tokens against a mock/testcontainers Keycloak; real Keycloak only for the full
live-stack run. CI runs this as the `e2e` job.
