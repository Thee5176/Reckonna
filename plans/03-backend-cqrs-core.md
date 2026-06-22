---
feature: 03-backend-cqrs-core
status: approved
domain: backend
depends_on: 00-bootstrap-deps-vault
external_prereq: infra/keycloak-oidc   # self-hosted Keycloak — new infra task (see Hand-off)
source_ref:
  command: Accounting_CQRS_Project/springboot_cqrs_command @ aa24678
  query:   Accounting_CQRS_Project/springboot_cqrs_query   @ 59ff36b
decisions:
  strategy: backend-complete first (all endpoints); frontend + infra are later features
  money: PostgreSQL NUMERIC(20,4) + shopspring/decimal in Go (source used Double — precision bug, dropped)
  auth: Keycloak OIDC (self-hosted) — resource-server JWT validation via JWKS, owner scoping by `sub`
  data: greenfield (old schema is reference only; no import)
  cqrs_db: single shared `accounting` schema, command writes / query reads (faithful to source)
  balance_invariant: enforced at domain (NewLedger) AND db (deferred CONSTRAINT TRIGGER) — deviation: source had NO db check
  ids: UUIDv7 (source used UUIDv4)
  reads: normalized POST-for-read endpoints → GET (CQRS read-purity; deviation from source)
  docs: spec-first OpenAPI (api/openapi.yaml) as the API contract + test oracle; tbls-generated ERD
        with tbls-diff CI gate; mermaid sequence diagrams in docs/; Swagger UI at /docs; graphify re-index.
        Docs are verification artifacts, not hand-maintained prose (anti-drift — see Technical documentation).
  standard: doc/coa-governance-standard.md is THE governing standard (4 layers: accounts · dimensions ·
        books · mappings). v1 is PHASED: build accounts + dimensions + the single `base` book.
        Delta books, the mapping layer, and multi-framework (IFRS/GAAP/TFRS) are LATER feature plans.
  currency: currency is a DIMENSION (standard P2), NOT a column. No fx_rate/base_amount pair. v1: a
        journal entry is single-currency (its currency dimension); 借方=貸方 balances in that currency per
        the base book. Cross-currency entries + functional-currency consolidation arrive with the
        books/measurement phase. (Supersedes the earlier base+FX-column model.)
  coa: DOC-DRIVEN. doc/coa-governance-standard.md (rules) → config/coa.yaml (canonical, 5-digit gapped,
        framework-neutral English names, §6 attributes) → generated seed (S5b) + locales. Currency-neutral.
  i18n: DB language-neutral (account `code` + canonical English `title`). Display names via
        locales/<lang>.json keyed by code (v1: en, ja; extensible). Translations never touch schema.
---

# Plan 03 — Backend CQRS Core (Go/Gin rewrite of the Spring Boot command+query services)

Faithful Go/Gin rewrite of BOTH Java services into `cmd/command` + `cmd/query`, preserving the
CQRS split, the double-entry domain, and every business endpoint. Improvements mandated by Reckonna
CLAUDE.md are folded in: DB-level 借方=貸方 enforcement, exact decimal money, UUIDv7.

**Prerequisite:** Plan 00 must be `approved` and landed (go.mod, sqlc.yaml, Makefile, docker-compose,
golang-migrate) — this plan writes no toolchain.

## Scope

**In:** Ledger create/update/delete (command); ledger get/list, ledger-item get, Chart-of-Accounts
list, outstanding balances, balance-sheet statement, profit-loss statement (query); Keycloak OIDC
auth middleware + owner scoping; OTel spans; health checks.

**Out (documented, not reimplemented):**
- Source `POST/PUT/DELETE /command/ledger_items` direct endpoints — marked "not recommended" in source,
  bypass the balance check. Reimplementing would let callers break the invariant. Dropped by design.
- Frontend (RN/Expo) and infra (Terraform/k8s/Vault migration, Keycloak provisioning) — separate features.

## Endpoint map (source → rewrite)

| Source (Java)                                   | Rewrite (Go/Gin)                         | Service |
|-------------------------------------------------|------------------------------------------|---------|
| `POST /command/ledger`                          | `POST /command/ledgers`                  | command |
| `PUT /command/ledger`                           | `PUT /command/ledgers/:id`               | command |
| `DELETE /command/ledger?uuid=`                  | `DELETE /command/ledgers/:id`            | command |
| `GET /command/health`                           | `GET /command/health`                    | command |
| `GET /query/api/ledgers/all`                    | `GET /query/ledgers`                      | query   |
| `GET /query/api/ledgers?uuid=`                  | `GET /query/ledgers/:id`                  | query   |
| `GET /query/api/ledger-items?uuid=`             | `GET /query/ledger-items/:id`            | query   |
| `POST /query/available-coa/json`                | `GET /query/code-of-accounts`            | query   |
| `POST /query/outstanding/`                      | `GET /query/balances?coa=1&coa=2`        | query   |
| `GET /query/api/balance-sheet-statement`        | `GET /query/statements/balance-sheet`    | query   |
| `GET /query/api/profit-loss-statement`          | `GET /query/statements/profit-loss`      | query   |
| `GET /query/health`                             | `GET /query/health`                      | query   |

---

## Master data (PREREQUISITE — provide/confirm before S5b seed + before functions work)

Governed by **`doc/coa-governance-standard.md`** (4 layers: accounts · dimensions · books · mappings).
v1 is PHASED: accounts + dimensions + the single `base` book. Delta books, mappings, multi-framework
are later features. **Scaffolded** this phase: `config/coa.yaml` (20 5-digit starter accounts),
`locales/en.json` + `locales/ja.json`. Seed (S5b) + extra locales are GENERATED + validated vs the standard.

### 1. Account model — `account` (doc-driven; S5b seed) — 20 starter accounts
Source of truth = `config/coa.yaml`. DB enums: `account_type` = (asset, liability, equity, income, expense);
`normal_balance` = (debit, credit). Per §6 each account has: 5-digit `code`, `name` (English canonical),
`type`, `normal_balance`, `postable`, `current_noncurrent` (assets/liabilities), `status`,
`required_dimensions` (optional). Currency-neutral (§4). 5-digit gapped codes per §4 ranges.
Statement mapping (v1, base book): BS = asset/liability/equity; P&L = income/expense.

| Range | Codes (v1) | Accounts |
|-------|-----------|----------|
| Current assets 10000–13999 | 10000,11000,12000 | Cash & equivalents · Trade & other receivables · Prepayments |
| Non-current assets 14000–19999 | 14000,14500 | PP&E · Intangible assets |
| Current liabilities 20000–23999 | 20000,21000,21500 | Trade & other payables · Accrued expenses · Customer escrow payable* |
| Non-current liabilities 24000+ | 24000 | Lease liabilities |
| Equity 30000–39999 | 30000,31000 | Contributed capital · Retained earnings |
| Income 40000 / 70000 | 40000,70000 | Service / fee revenue · Finance income |
| Expense 50000/60000s/71000/80000 | 50000,60000,61000,62000,63000,71000,80000 | Cost of services · Staff · Marketing · G&A · Depreciation · Finance costs · Income tax |

\* `21500 Customer escrow payable` has `required_dimensions: [counterparty]` — a seed/validation test
asserts a required dimension is enforced (proves §7 R7.4).

### 2. Dimensions (§7) — `dimension_type` + `dimension_value` (S3 + S5c seed)
Context lives in dimensions, NOT in codes or accounts. v1 dimension types: **entity**, **currency**,
**counterparty**. Members are data (added without CoA change). Every journal line carries dimension values.
- `currency` is a dimension. Members seeded: JPY (functional default), USD, EUR, GBP… (ISO 4217).
- An account may require dimensions (`required_dimensions`); the command rejects a line missing one (→422).
- Daily entry uses defaults (single entity, functional currency) so the user never sees this (R7.5).

### 3. Books (§8) — `book` (S3 + seed) — v1 = `base` only
One `base` book. **Each journal entry balances within a book** (R8.4): Σ debit = Σ credit. Delta books
(ifrs/gaap) + the mapping layer + cross-book consolidation are LATER features — not in plan 03.

### 4. Currency handling (v1, phased)
Currency is a dimension value on each line (§2). **A v1 journal entry is single-currency** — all lines
share the entry's currency dimension — so 借方=貸方 balances in that currency, no conversion needed.
Cross-currency entries + functional-currency consolidation (measurement/books) are deferred. No
`fx_rate`/`base_amount` columns. (Supersedes the earlier base+FX model.)

### 5. Identity master (Keycloak — infra feature, not this DB)
- Keycloak realm + client (audience) + JWKS issuer URL.
- ≥1 seed owner (`sub`) for owner-scoping tests (IT5) and e2e (AT3/AT4). Provided by `infra/keycloak-oidc`;
  S10 unit tests use a mock issuer so backend isn't blocked.

### 6. Localization (i18n)
- DB language-neutral: `code` + English `name`. Display names in `locales/<lang>.json` keyed by code.
- v1 ships `en` + `ja` (20 accounts each); +more on request. Generator stubs missing keys.
- A coverage test asserts every `config/coa.yaml` code has an entry in every shipped locale (IT14).

### 7. Domain naming under the standard
Source `Ledgers`→ **JournalEntry** (header: date, description, owner, book=base); source `LedgerItems`→
**JournalLine** (account, amount, debit/credit, dimension values incl. currency). Tables:
`journal_entry`, `journal_line`, `account`, `dimension_type`, `dimension_value`, `journal_line_dimension`,
`book`. (Renames the earlier `ledgers`/`ledger_items` — downstream steps inherit this.)

### 8. Resolved decisions (this turn)
- Adopt **`doc/coa-governance-standard.md`** as THE standard; my `docs/` duplicate deleted.
- **Phased v1**: accounts + dimensions + base book. Delta books + mappings + multi-framework later.
- CoA = **5-digit starter ranges** from the standard appendix (20 accounts), doc-driven.
- **Currency = dimension**; v1 single-currency entries; no FX columns; consolidation deferred.

## Section 1 — Acceptance-test spec (E2E)   [from business requirements]

| ID   | Given / When / Then                                                                                          | Test file                                |
|------|------------------------------------------------------------------------------------------------------------|------------------------------------------|
| AT1  | Given valid JWT / When POST balanced ledger (debit 1000 + credit 1000) / Then 201, persisted, retrievable, 借方=貸方 | e2e/ledger_create.e2e_test.go            |
| AT2  | Given valid JWT / When POST unbalanced ledger (debit 1000 + credit 500) / Then 422 + body `{error:"unbalanced ledger: 借方≠貸方"}` | e2e/ledger_invalid.e2e_test.go           |
| AT3  | Given two owners' ledgers / When owner A GET /query/ledgers / Then only A's rows returned                    | e2e/ledger_owner_scope.e2e_test.go       |
| AT4  | Given ledger owned by A / When B PUT/DELETE it / Then 403 AccessDenied                                       | e2e/ledger_ownership.e2e_test.go         |
| AT5  | Given ledger with N items / When DELETE ledger / Then ledger + all items gone (cascade)                      | e2e/ledger_delete.e2e_test.go            |
| AT6  | Given balanced ledgers / When GET /query/statements/balance-sheet / Then assets == liabilities + equity      | e2e/balance_sheet.e2e_test.go            |
| AT7  | Given revenue+expense ledgers / When GET /query/statements/profit-loss / Then netIncome == revenue − expenses | e2e/profit_loss.e2e_test.go              |
| AT8  | Given no/invalid JWT / When any non-health endpoint / Then 401                                               | e2e/auth_unauthorized.e2e_test.go        |
| AT9  | Given seeded CoA / When GET /query/code-of-accounts / Then full chart returned                               | e2e/coa_list.e2e_test.go                 |
| AT10 | Given POST ledger referencing unknown coa / When submit / Then 422 (FK/validation)                          | e2e/ledger_bad_coa.e2e_test.go           |
| AT11 | Given decimal amounts (1000.3333) / When round-trip create→read / Then no float drift (exact)                | e2e/money_precision.e2e_test.go          |
| AT12 | Given the running service / When GET /docs / Then Swagger UI renders from api/openapi.yaml                    | e2e/docs_served.e2e_test.go              |
| AT13 | Given currency dimension / When POST single-currency USD entry (debit 1000 + credit 1000 USD) / Then 201, balanced | e2e/entry_currency_dim.e2e_test.go |
| AT14 | Given lines with mixed currencies in one entry / When POST / Then 422 (v1: entry must be single-currency)    | e2e/entry_mixed_currency.e2e_test.go     |
| AT15 | Given Swagger UI at /docs / When user pastes valid JWT into Authorize widget + runs "Try it out" on POST /command/ledgers with the embedded request example / Then 201 with response example schema | e2e/docs_interactive.e2e_test.go (chromedp headless) |
| AT16 | Given api/collections/ (Bruno or .http) generated from openapi.yaml / When CI runs every request once against the e2e stack / Then every collection request returns its documented 2xx (interactive parity with browser Try-it-out) | e2e/collection_runner.e2e_test.go |

## Section 2 — Integration-test spec   [from architecture / CQRS contracts]

| ID  | Condition to verify                                                                                  | Test file                                  |
|-----|-----------------------------------------------------------------------------------------------------|--------------------------------------------|
| IT1 | command.PostLedger → query.GetLedger returns same id; items sum debit == sum credit                  | internal/query/ledger_it_test.go           |
| IT2 | tx fails mid-write (2nd item insert errors) → NO partial rows remain (atomic rollback)                | internal/repository/ledger_tx_test.go      |
| IT3 | DB CONSTRAINT TRIGGER rejects an unbalanced multi-row insert even if domain check is bypassed         | internal/repository/balance_trigger_test.go|
| IT4 | OIDC middleware validates JWT against Keycloak JWKS; bad sig/issuer/aud/expiry → 401                  | internal/handler/middleware/auth_test.go   |
| IT5 | Query read paths filter by owner `sub`; no cross-owner leakage in SQL                                 | internal/query/owner_scope_test.go         |
| IT6 | Every request/response is schema-valid against `api/openapi.yaml` (spec is the oracle, not hand goldens) | internal/handler/contract_test.go          |
| IT7 | balance-sheet & profit-loss aggregates group by CoA element correctly                                 | internal/query/statement_test.go           |
| IT8 | migrations up→down→up idempotent; `sqlc generate` compiles; CHECK trigger present                     | db/migration/migrate_test.go               |
| IT9 | Query service has NO write path (no INSERT/UPDATE/DELETE in cmd/query build) — static assert/grep gate | internal/query/readonly_test.go            |
| IT10| `tbls diff` is clean — committed ERD/schema docs match the live migrated schema (anti-drift gate)      | ci: docs job (`make docs-verify`)          |
| IT11| `api/openapi.yaml` lints valid; every Gin route is present in the spec and vice-versa (no orphan paths) | internal/handler/openapi_coverage_test.go  |
| IT12| a journal line missing a required dimension (counterparty on 21500) is rejected (422)                 | internal/handler/dimension_required_test.go|
| IT13| balance trigger validates Σ debit = Σ credit per (entry, book); a mixed-currency entry is rejected     | internal/repository/book_balance_test.go   |
| IT14| every `config/coa.yaml` code has a translation in each shipped locale (en, ja)                         | internal/config/i18n_coverage_test.go      |
| IT15| every operation in `api/openapi.yaml` ships a request `example` AND an `example` for every documented response code — Swagger UI Try-it-out is never blank | internal/handler/openapi_examples_test.go  |
| IT16| `api/collections/` (Bruno or .http) is regenerated from `api/openapi.yaml` and checked in; CI fails if generator drift detected (`make collections-verify`) | internal/handler/collections_drift_test.go |
| IT17| e2e harness boots in <30s on CI (testcontainers Postgres + Keycloak mock + command + query) — perf budget so the e2e job stays usable | e2e/harness_perf_test.go                   |

## Section 3 — Implementation steps (one commit each; unit test per step)

One step = one commit = one PR-reviewable change. Each compiles & passes on its own. TDD visible:
failing-test commit BEFORE code commit for domain/money/auth paths. Every commit carries `Plan: S<n>`.

| ID  | Commit message (verbatim)                                       | Files                                                                 | Depends      | Unit test                              |
|-----|----------------------------------------------------------------|----------------------------------------------------------------------|--------------|----------------------------------------|
| S1  | `test(domain): failing Ledger balance + decimal money tests`   | internal/domain/ledger_test.go                                       | 00           | TestNewLedger/unbalanced, /decimal (RED)|
| S2  | `feat(domain): Ledger, LedgerItem, CoA entities + invariant`   | internal/domain/ledger.go, internal/domain/coa.go, internal/domain/money.go | S1   | TestNewLedger (GREEN); ErrUnbalanced    |
| S3  | `feat(db): schema — account, journal_entry/line, dimension(_type/value), book` | db/migration/001_accounting.up.sql, 001_accounting.down.sql | 00 | migrate up/down (IT8)        |
| S4  | `feat(db): deferred balance TRIGGER per (entry,book) (借方=貸方)` | db/migration/002_balance_check.up.sql, 002_balance_check.down.sql | S3      | balance_trigger_test (IT3)              |
| S5  | `feat(db): sqlc queries for ledger command + reads`            | db/query/ledger.sql, db/query/coa.sql, db/query/statement.sql        | S3,S4        | `sqlc generate`; compile (IT8)          |
| S5a | `chore(tooling): gen-coa (coa.yaml → seed + locale stubs + validate)` | scripts/gen-coa.go, Makefile                                 | -            | validates config/coa.yaml vs standard   |
| S5b | `feat(db): seed account (generated from config/coa.yaml)` | db/migration/003_seed_account.up.sql, 003_seed_account.down.sql  | S3,S5a       | 20 accounts; req-dim on 21500 (AT6/7/9) |
| S5c | `feat(db): seed base book + dimension types/values` | db/migration/004_seed_dimensions.up.sql, 004_seed_dimensions.down.sql | S3      | base book + currency members (JPY default) |
| S6  | `feat(repository): ledger command repo + tx orchestration`     | internal/repository/ledger_repo.go                                   | S2,S5        | ledger_tx_test (IT2)                    |
| S7  | `feat(service): PostLedger use case (validate → atomic write)` | internal/service/ledger_command.go                                   | S6           | TestPostLedger                          |
| S8  | `feat(handler): POST /command/ledgers + DTO mapping`           | internal/handler/ledger_command.go, internal/handler/dto.go         | S7           | handler test (AT1, AT2, AT10)           |
| S9  | `feat(service,handler): Update/Delete ledger + ownership`      | internal/service/ledger_command.go, internal/handler/ledger_command.go | S8        | ownership tests (AT4, AT5)              |
| S10 | `feat(auth): Keycloak OIDC middleware (JWKS, sub claim)`       | internal/handler/middleware/auth.go, internal/config/oidc.go        | 00, kc-prereq| auth_test mock-JWKS (IT4, AT8)          |
| S11 | `feat(query): GetLedger + ListLedgers owner-scoped reads`      | internal/query/ledger_query.go, internal/handler/ledger_query.go    | S5,S10       | ledger_it_test (IT1, IT5, AT3)          |
| S12 | `feat(query): ledger-item get + CoA list + outstanding balances`| internal/query/coa_query.go, internal/handler/coa_query.go         | S11          | coa tests (AT9)                         |
| S13 | `feat(query): balance-sheet statement aggregate`              | internal/query/statement_query.go, internal/handler/statement.go    | S12          | TestBalanceSheet (AT6, IT7)             |
| S14 | `feat(query): profit-loss statement aggregate`               | internal/query/statement_query.go, internal/handler/statement.go    | S13          | TestProfitLoss (AT7, IT7)               |
| S15 | `feat(cmd): wire command + query Gin routers + config`        | cmd/command/main.go, cmd/query/main.go, internal/config/config.go   | S9,S14       | boots; health 200; readonly_test (IT9)  |
| S16 | `feat(obs): OTel spans on command + query handlers`          | internal/config/otel.go, internal/handler/ledger_command.go         | S15          | span emitted on PostLedger (devops Done)|
| S17a| `test(e2e): harness — testcontainers Postgres + Keycloak mock + JWT minter + spec-validator helper` | e2e/harness/stack.go, e2e/harness/jwt.go, e2e/harness/spec_oracle.go | S15,S18      | harness_smoke_test (IT17 boot budget)   |
| S17b| `test(contract,e2e): AT1–AT14 + IT6 contract test (response auto-validated vs openapi.yaml)`     | internal/handler/contract_test.go, e2e/*.e2e_test.go               | S17a         | all AT/IT green                         |
| S18 | `feat(api): openapi.yaml contract + oapi-codegen types + bearerAuth + per-op examples` | api/openapi.yaml, internal/handler/oapi_gen.go            | S8           | spec lints; IT6, IT11, IT15 (examples)  |
| S19 | `feat(api): serve Swagger UI at /docs from openapi.yaml (with bearerAuth widget)`     | internal/handler/docs.go, cmd/command/main.go, cmd/query/main.go    | S18          | GET /docs 200 (AT12); AT15 interactive  |
| S19a| `feat(api): generate Bruno/.http collection from openapi.yaml + make collections`     | scripts/gen-collections.go, api/collections/*.bru, Makefile         | S18          | collections-verify drift gate (IT16)    |
| S19b| `chore(dev): make docs-serve — local Swagger UI + Redoc live-reload on api/openapi.yaml` | Makefile, scripts/docs-serve.sh                                  | S18          | `make docs-serve` boots on dev          |
| S20 | `docs(db): tbls config + generated ERD + make docs(-verify)` | .tbls.yml, docs/schema/, Makefile                                  | S5b          | `tbls diff` clean (IT10)                 |
| S21 | `docs(arch): mermaid sequence diagrams (post/cqrs/oidc)`     | docs/sequences/post-ledger.md, cqrs-write-read.md, oidc-auth.md     | S15          | mermaid renders in CI                    |
| S22 | `ci(docs): wire tbls-diff + openapi-lint + mermaid + graphify`| .github/workflows/ci.yml (docs job)                                | S18,S20,S21  | docs job green; graphify re-index runs   |
| S22a| `ci(e2e): wire testcontainers e2e job + collection runner` | .github/workflows/ci.yml (e2e job)                                  | S17b,S19a    | e2e + AT16 collection runner green      |

### Step notes
- **S2 money + dimensions:** `domain.Money` wraps `shopspring/decimal.Decimal`. A `JournalLine` carries
  `amount`, `debit|credit`, an `account` ref, and dimension values (incl. `currency`). A `JournalEntry`
  is single-currency in v1: `NewEntry(lines)` returns `ErrMixedCurrency` if lines disagree on currency,
  `ErrUnbalanced` if Σ(amount·sign) ≠ 0, and `ErrRequiredDimension` if an account's `required_dimensions`
  are unset. No `float64` on any money path.
- **S4 trigger:** `CREATE CONSTRAINT TRIGGER ... DEFERRABLE INITIALLY DEFERRED` firing per
  `(journal_entry_id, book_id)` at COMMIT: `SUM(amount) FILTER (debit) = SUM(amount) FILTER (credit)`.
  Deferred so a multi-line insert in one tx is validated once, at commit — not per row. v1 entries are
  single-currency so the sum is in one unit; cross-book/cross-currency consolidation is a later phase.
- **S5a generator:** `scripts/gen-coa` reads `config/coa.yaml`, validates it against
  `doc/coa-governance-standard.md` (5-digit code in §4 range, type↔normal-balance consistency, unique
  code/name, required_dimensions exist), then emits `003_seed_account.up/down.sql` and stubs any missing
  `locales/*.json` key. Run in CI so a hand-edited seed or an untranslated account fails the build.
- **S5 sqlc money mapping:** override `numeric` → `github.com/shopspring/decimal.Decimal` in sqlc.yaml
  (pgx/v5 + decimal). CoA `coa` is `int`; `element`/`type` are PG enums → Go typed constants.
- **S10 Keycloak (cross-domain prereq):** middleware validates RS256 JWT against Keycloak's JWKS
  (`/.well-known/openid-configuration`), checks issuer + audience + expiry, extracts `sub` → owner id.
  Unit-tested against a **mock JWKS / testcontainers Keycloak** so it is green WITHOUT the real infra
  Keycloak. Real Keycloak issuer URL comes from Vault-rendered config at runtime (never hardcoded).
- **S15 CQRS purity:** `cmd/query` links only read repos. IT9 greps the query build for
  INSERT/UPDATE/DELETE and fails if found — enforces "query MUST NOT mutate".
- **S17a e2e harness (reusable):** `e2e/harness/` exposes three primitives every AT/IT case composes:
  (1) `Stack()` — testcontainers Postgres + a Keycloak mock issuer (JWKS endpoint serving a static RSA
  keypair) + the command + query binaries booted on random ports, returned as a `*Stack` with `BaseURL`,
  `DB`, `Cleanup()`; (2) `MintJWT(sub, aud)` — signs an RS256 token with the harness keypair so each
  case mints its own owner identity (AT3/AT4 use two `sub`s); (3) `AssertSpecValid(t, req, resp)` —
  loads `api/openapi.yaml` once per package via `kin-openapi`, validates the captured request+response
  against the operation; called automatically from `harness.Do()`. Net effect: every AT case gets a
  free schema-validation pass (IT6 is enforced by every e2e case, not one dedicated test). Harness
  boot is budget-checked at <30s (IT17) so the e2e CI job stays usable.
- **S17b e2e cases:** thin — each AT case is ~30 LOC because the harness owns boot, auth, and the
  spec oracle. Failure mode: a handler that returns a body the spec doesn't describe fails AT1 AND
  IT6 with one assertion. No hand-maintained golden JSON.
- **S18 examples (IT15):** every operation in `openapi.yaml` carries a request `example` AND an
  `example` per documented response code. These doubles serve as: (a) Swagger UI "Try it out" prefill
  so the interactive flow is one-click not blank-form, (b) the canonical payload that AT15 replays
  through a headless chromedp drive of /docs, (c) drift detection — if the handler's response
  diverges from the example, IT6 (spec validation) and the contract test both fail. Spec also ships
  the `bearerAuth` security scheme so the Authorize widget shows up.
- **S19a interactive collection:** `scripts/gen-collections.go` reads `api/openapi.yaml` and emits
  `api/collections/*.bru` (Bruno format) — versioned alongside the spec so a checkout works in Bruno
  without re-importing. `make collections-verify` regenerates into a temp dir and diffs vs checked-in
  files; drift fails CI (IT16). AT16 runs the collection through `bruno-cli` (or the equivalent .http
  runner) against the live e2e stack — proves "what Swagger UI shows" matches "what curl sees" matches
  "what the spec promises". Three-way parity.
- **S19b dev loop:** `make docs-serve` runs Swagger UI + Redoc side-by-side on `localhost:8000` with a
  fs watcher that reloads on `api/openapi.yaml` change. Pure dev convenience; not gated.
- **S22a CI e2e:** dedicated `e2e` job in `.github/workflows/ci.yml` (separate from the docs job) runs
  `go test -tags=e2e ./e2e/...` against testcontainers. Caches the Postgres + Keycloak-mock images so
  the budget in IT17 holds. AT16 collection runner runs in the same job after the suite passes so it
  exercises the already-booted stack.

### Deviations from source (all deliberate, all logged here)
1. Money `Double` → `NUMERIC`/`decimal` (precision).
2. No DB balance check → deferred CONSTRAINT TRIGGER (defense in depth; Reckonna mandate).
3. UUIDv4 → UUIDv7 (Reckonna mandate).
4. POST-for-read query endpoints → GET (CQRS read-purity).
5. Auth0 → self-hosted Keycloak OIDC (de-vendor; issuer becomes Vault config).
6. Dropped deprecated direct `ledger_items` write endpoints (invariant-bypassing).
7. RESTful resource paths (`/ledgers/:id`) replace query-param `?uuid=`.
8. Single-currency `Double` → **currency as a dimension** (standard P2); v1 single-currency entries;
   functional consolidation via books/measurement deferred (phased).
9. Hand-coded JP CoA → **doc-driven** 5-digit CoA per `doc/coa-governance-standard.md` (accounts +
   dimensions + base book; delta books/mappings/multi-framework phased) + i18n.

## Technical documentation (spec-first, CI-gated — anti-drift)

Docs are generated from a source of truth and gated in CI so they cannot rot (this repo previously
claimed gitleaks in 3 docs with zero CI — never again).

| Doc | Source of truth | Tool | Output | Gate |
|-----|-----------------|------|--------|------|
| API reference | `api/openapi.yaml` (spec-first, written before handlers) | oapi-codegen + kin-openapi | Swagger UI at `/docs` (bearerAuth + per-op examples) | IT6 validates every req/resp vs spec; IT11 route coverage; IT15 examples present |
| Interactive collection | `api/openapi.yaml` (same spec) | `scripts/gen-collections.go` → Bruno `.bru` | `api/collections/` | IT16 drift gate (`make collections-verify`); AT16 runs the collection against the e2e stack |
| E-R diagram | `db/migration/*.sql` (live schema) | `tbls` | `docs/schema/` (md + mermaid ERD) | IT10 `tbls diff` fails CI on schema≠ERD |
| Sequence diagrams | plan design intent | mermaid in `docs/sequences/` | post-ledger, cqrs-write-read, oidc-auth | rendered in CI; PR review |
| Knowledge graph | the whole repo | `graphify --update` | `graphify-out/` | re-indexed in the docs CI job (S22) |

Spec-first ordering matters: `api/openapi.yaml` (S18) lands BEFORE the contract test tightens (S17),
and the SAME `openapi.yaml` is the contract the later frontend feature consumes — one oracle, two sides.
Port intent (not pixels) from source reference diagrams: `/tmp/accsrc/Design/create_sequence.md`,
`7月_CQRS_patterns.drawio`.

## Hand-off to the heads
- **backend-engineer (HEAD):** owns S1–S22a (incl S5a/S5b/S5c, S17a/S17b, S19a/S19b). Write AT1–AT16 + IT1–IT17 as FAILING tests FIRST, then
  green via domain-modeler → tdd-implementer/migration-mapper → code-reviewer. Use `migration-mapper`
  for the Java→Go port (jOOQ→sqlc, Flyway→golang-migrate, Spring validation→domain+struct tags).
- **infra-engineer (HEAD) — NEW TASK (separate feature plan `infra/keycloak-oidc`):** stand up a
  self-hosted Keycloak as the OIDC provider (realm, client, audience, JWKS), expose issuer URL via
  Vault. Blocks S10 *at runtime* only — S10's tests run against mock/testcontainers Keycloak, so
  backend work is not gated on infra completion. Provision before e2e (S17) against a live stack.
- **plan-tracker:** logs each landed step to `03-backend-cqrs-core.impl.md`.

"Done" = AT1–AT14 + IT1–IT14 green, `make test` race-clean, `make lint` clean, query build read-only,
`make docs-verify` clean (openapi lint + tbls diff), CoA seed generated from config/coa.yaml + validated
vs the governance standard, all locales cover every account, Swagger UI live at /docs.
