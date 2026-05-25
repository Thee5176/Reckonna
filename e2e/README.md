# `e2e/` — End-to-End Acceptance Tests

Top of the V-model. These verify whole-system behavior against the **acceptance spec** in
`plans/01-backend-cqrs-core.md` (AT1–AT14) — the business-requirement layer.

## What lives here
- Go tests behind the `e2e` build tag (`//go:build e2e`), so a normal `go test ./...` skips them.
- Each test drives the real HTTP API of `cmd/command` + `cmd/query` against a **real Postgres**
  (testcontainers-go) with migrations + the deferred balance trigger applied — so the DB-level
  借方=貸方 invariant is exercised, not mocked.

## Mapping (plan 01)
| Test file | Acceptance |
|-----------|-----------|
| `ledger_create.e2e_test.go` | AT1 balanced create |
| `ledger_invalid.e2e_test.go` | AT2 unbalanced → 422 (借方≠貸方) |
| `ledger_owner_scope.e2e_test.go` | AT3 owner scoping |
| `ledger_ownership.e2e_test.go` | AT4 non-owner → 403 |
| `ledger_delete.e2e_test.go` | AT5 cascade delete |
| `balance_sheet.e2e_test.go` / `profit_loss.e2e_test.go` | AT6 / AT7 statements |
| `auth_unauthorized.e2e_test.go` | AT8 401 |
| `coa_list.e2e_test.go` / `ledger_bad_coa.e2e_test.go` | AT9 / AT10 |
| `money_precision.e2e_test.go` | AT11 decimal exactness |
| `docs_served.e2e_test.go` | AT12 Swagger UI at /docs |
| `ledger_multicurrency.e2e_test.go` / `ledger_fx_missing.e2e_test.go` | AT13 / AT14 multi-currency + FX |

## Run
```bash
go test -tags e2e ./e2e/... -count=1        # needs Docker (testcontainers)
```
Auth: tests mint OIDC tokens against a mock/testcontainers Keycloak; real Keycloak only for the full
live-stack run. CI runs this as the `e2e` job.
