# Plan-03 Acceptance Validation Status

Durable log of `/validate-at` runs on `feat/03-backend-cqrs-core` (PR #16).
Source ledger: `docs/qa-plan-03.md`. One row per criterion; updated in place on re-validation.

| AT  | criterion | result | gate STATUS | key conditions | evidence | date |
|-----|-----------|--------|-------------|----------------|----------|------|
| A1/AT1  | balanced create → persist+read, 借方=貸方 | PASS | ERROR* | 201, retrievable, 2 lines, ETag `"1"` | `TestE2E_CreateAndRead` (e2e) | 2026-07-08 |
| A2/AT2  | unbalanced 1000/500 → reject **[money invalid case]** | PASS | ERROR* | 422 problem+json, `code:unbalanced_entry` | `TestE2E_SemanticRejections/unbalanced (AT2)` | 2026-07-08 |
| A3/AT3  | owner scope — cross-owner GET → 404 | PASS | ERROR* | bob cannot see alice; bob list empty | `TestE2E_OwnerScope (AT3/T16)` | 2026-07-08 |
| A4/AT4  | non-owner PUT/DELETE → 403 | PASS | ERROR* | bob delete alice → 403 | `TestE2E_OwnerScope (AT4)` + `ledger_command_test` cross-owner | 2026-07-08 |
| A5/AT5  | DELETE entry cascades lines | PASS | ERROR* | remaining line rows == 0 (real PG) | `internal/service/ledger_command_test.go:162 (AT5)` | 2026-07-08 |
| A6/AT6  | balance-sheet: assets == liab+equity | PASS | ERROR* | statement balances | `TestE2E_Statements/balance_sheet (AT6)` | 2026-07-08 |
| A7/AT7  | profit-loss: netIncome == rev−exp | PASS | ERROR* | net income correct | `TestE2E_Statements/profit-loss (AT7)` | 2026-07-08 |
| A8/AT8  | no/invalid JWT any non-health → 401 | PASS | ERROR* | missing + garbage bearer → 401 | `TestE2E_Unauthorized (AT8)` | 2026-07-08 |
| A9/AT9  | seeded CoA list → full chart | PASS | ERROR* | `Accounts` len == 20 | `internal/handler/query/handler_test.go:152 (AT9)` | 2026-07-08 |

*Legend: result = PASS / FAIL / BLOCKED(human). gate STATUS = SonarQube quality gate.

## Gate note (shared BE blocker — not an A1–A9 defect)
All 9 acceptance criteria are **functionally PASS** (committed tests green, real Postgres via
podman testcontainers, DB balance trigger exercised). The SonarQube gate STATUS is **ERROR** on
`new_coverage` (74.5 < 80 project-wide = tracked item B2/B3 in the merge-train
`validation-status.md`), NOT on any A1–A9 behavior. Validated packages (domain+service) sit at
81.3%; the coverage debt is in handler/query. A full `sonar-scanner` rescan was deferred (CE
overwrites the single project; would clobber develop/FE scan state) — run it once at merge time.

## Env
- Test DB: podman testcontainers (`~/.testcontainers.properties` → rootless podman sock,
  `ryuk.disabled=true`) OR Vault shared-PG DSN via `scripts/render-test-db-url.sh` (auto by `make test`).
- e2e run: `go test -tags e2e ./e2e/... -count=1` (35.2s, all green).
