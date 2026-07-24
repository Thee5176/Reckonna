# /qa-plan ledger — Plan 03 Backend CQRS Core

Agent todo list produced by `/qa-plan`. Enumeration only — **not** validation.
`/validate-at` works these ONE at a time, flipping `[ ]`→`[x]` with evidence.

- **Plan:** `plans/03-backend-cqrs-core.md` · **Branch:** `feat/03-backend-cqrs-core` (PR #16)
- **Scope (plan "Done"):** AT1–11, AT13–18 + IT1–5, IT7–9, IT12–18.
- **Excluded (docs-hardening plan):** AT12, IT6, IT10, IT11.
- **Total:** 37 criteria · 0 done · human-gated: 3 (C2, C3) · C3 additionally BLOCKED.

## Run commands (referenced by `how`)
- `U`  = unit/IT: `make test` (`go test ./... -race`)
- `E`  = e2e testcontainers: `go test -tags e2e ./e2e/... -count=1` (Docker/podman: `DOCKER_HOST=unix:///run/user/$(id -u)/podman/podman.sock`)
- `Elive` = e2e vs live homelab PG: `RECKONNA_TEST_DATABASE_URL=postgres://…tailnet…/accounting go test -tags e2e ./e2e/... -count=1` (per-test schema isolation)
- migrate fresh on real PG: `make migrate`

---

## Group A — Acceptance (E2E, `//go:build e2e`)

- [x] **A1 — AT1** balanced create → persist+read, 借方=貸方 · `E TestE2E_CreateAndRead` → 201, retrievable, sum debit=credit
- [x] **A2 — AT2** unbalanced 1000/500 → reject · `E TestE2E_SemanticRejections/unbalanced` → 422 `application/problem+json`, `code:unbalanced_entry`, `errors[0]{line_index:0,field:amount,issue:debit_credit_mismatch}` **[MONEY: unbalanced-ledger rejection]**
- [x] **A3 — AT3** owner scope — A sees only A's · `E TestE2E_OwnerScope` → only A rows; cross-owner GET → 404 (T16)
- [x] **A4 — AT4** B PUT/DELETE A's entry · `E TestE2E_OwnerScope/bob cannot delete` → 403
- [x] **A5 — AT5** DELETE entry cascades lines · `E` (delete case) → entry + all lines gone
- [x] **A6 — AT6** balance-sheet · `E TestE2E_Statements/balance sheet` → assets == liabilities+equity
- [x] **A7 — AT7** profit-loss · `E TestE2E_Statements/profit-loss` → netIncome == revenue−expenses
- [x] **A8 — AT8** no/invalid JWT any non-health · `E TestE2E_Unauthorized` → 401
- [x] **A9 — AT9** seeded CoA list · `E` coa case → full 20-account chart
- [ ] **A10 — AT10** unknown account code · `E TestE2E_SemanticRejections/unknown account` → 422
- [ ] **A11 — AT11** decimal round-trip 1000.3333 / 1000.33335 / 0.12345 · `E TestE2E_MoneyPrecision` (table 3) → NUMERIC(20,4) stable, no float drift
- [ ] **A12 — AT13** single-currency USD entry · `E TestE2E_CurrencyDimension/single…USD` → 201 balanced
- [ ] **A13 — AT14** mixed-currency in one entry · `E …/mixed currency` → 422 `code:mixed_currency`
- [ ] **A14 — AT15** idempotency replay same key+body · `E idempotency_replay` → cached 201, no 2nd `journal_entry` row
- [ ] **A15 — AT15b** same key, different body · `E idempotency_conflict` → 422 `code:duplicate_idempotency_key`
- [ ] **A16 — AT16** stale If-Match · `E concurrency_conflict` → 409 `code:concurrency_conflict` + current version
- [ ] **A17 — AT16b** If-Match missing on PUT · `E concurrency_precondition` → 428 `code:validation_failed`
- [ ] **A18 — AT17** `Content-Type: text/plain` POST · `E content_type_415` → 415 `code:unsupported_media_type`, no body parse
- [ ] **A19 — AT18** `Accept-Language: ja` error · `E i18n_error` → `code` locale-neutral AND `title`+`detail` Japanese

## Group B — Integration (`make test`)

- [x] **B1 — IT1** command.Post → query.Get same id · `U internal/query` → same id; sum debit=credit
- [x] **B2 — IT2** tx fails mid-write · `U ledger_tx/service` → atomic rollback, no partial rows
- [x] **B3 — IT3** DB trigger rejects unbalanced (domain bypassed) · `U internal/repository/balance_trigger_test` → trigger raises, insert refused **[MONEY: unbalanced-ledger rejection]**
- [x] **B4 — IT4** OIDC JWKS bad sig/iss/aud/exp · `U middleware/auth_test` (mock JWKS) → each 401
- [x] **B5 — IT5** read paths owner-filtered · `U internal/query/readonly_test`+owner → no cross-owner leakage
- [x] **B6 — IT7** statement aggregates group by CoA element · `U statement test` → BS/P&L grouped correct
- [x] **B7 — IT8** migrate up→down→up idempotent + trigger present — `migrate_test.go` · `make migrate`/migrate_test → clean cycle, CHECK trigger exists
- [x] **B8 — IT9** cmd/query read-only (compile-time) · `U internal/query/readonly_test` (go/parser walk) → fails if `repository/command` imported
- [x] **B9 — IT12** required dimension (counterparty on 21500) · `U dimension_required` → 422
- [x] **B10 — IT13** per-(entry,book) balance + mixed-currency reject · `U book_balance` → trigger rejects **[MONEY: unbalanced-ledger rejection]**
- [x] **B11 — IT14** every coa code + every error code has en+ja · `U internal/config/i18n_coverage_test` → 100% locale coverage
- [x] **B12 — IT15** Content-Type middleware pre-handler; OPTIONS passes · `U content_type_test` → non-JSON POST/PUT blocked early
- [x] **B13 — IT16** idempotency UNIQUE(key,owner_sub)+replay+TTL · `U idempotency_test` → cached row, cleanup query works
- [x] **B14 — IT17** version trigger bumps on UPDATE → ETag · `U version_trigger_test` → version increments
- [x] **B15 — IT18** rounding-divergence: domain ∧ DB trigger agree @4dp — Option B (reject >4dp), `32049ae` · `U rounding_divergence_test` → both reject full-precision-balanced/4dp-unbalanced (& reverse) **[MONEY: unbalanced-ledger rejection]**

## Group C — LIVE server + DB (gap items — added per request)

- [ ] **C1 — L1** e2e vs live homelab PG (not testcontainers) · `make migrate` on tailnet PG, then `Elive` → all Group A green vs `RECKONNA_TEST_DATABASE_URL`; migrations apply clean (fresh schema)
- [ ] **C2 — L2 [HUMAN]** live running binaries — `cmd/command`+`cmd/query` over real HTTP · `make up`; out-of-process HTTP client for AT1+AT8 → 201 create + 401 unauth over real network (closes httptest in-process gap)
- [ ] **C3 — L3 [HUMAN · BLOCKED]** full public live-stack — `reckonna.thee5176.com` + real Keycloak JWT + homelab PG · deploy (plan-05 images S17b), mint real OIDC token, smoke AT1/AT8/AT6 → public 201/401 + balanced statement w/ real JWKS. BLOCKED: PR#16 merge → S17b ghcr images → plan-05 deploy

---

## Notes
- **Order:** B (unit/IT) → A (e2e testcontainers) → C1 (live DB) → C2/C3 (live server, gated).
- **Gap:** plan e2e is in-process (`httptest.NewRequest`/`NewRecorder`) — "live server" (C2/C3) was never a plan criterion; added here.
- **Unbalanced-ledger rejection (money invariant)** covered by A2, B3, B10, B15.
- Handoff: `/validate-at` starts at B1.
