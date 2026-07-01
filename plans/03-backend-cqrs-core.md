---
feature: 03-backend-cqrs-core
status: draft
domain: backend
depends_on:
  - 00-bootstrap-deps-vault
  - 01-infra-postgres-tailnet   # was 02 pre-renumber
  - 02-infra-k8s-cloudflare-tunnel  # new — deploy target + public ingress
renumbered_from: 01-backend-cqrs-core   # 2026-05-31 priority swap
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

<!-- renumbered 2026-05-31 from plan 01. Plans 01 (postgres-tailnet) + 02 (k8s + cloudflare) are
     prerequisite infra; this plan now lands on top of a live homelab k3s + tailnet-PG + reckonna.thee5176.com. -->
<!-- Terminology (D-RENUM resolved 2026-06-29): "this plan" = plan 03. "the docs-hardening plan"
     = a LATER, unnumbered feature plan (OpenAPI/Swagger/tbls ERD/mermaid/graphify) — NOT
     02-infra-k8s-cloudflare-tunnel. All in-body plan-number references below updated accordingly. -->

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
- **OpenAPI spec + Swagger UI + tbls ERD + mermaid sequences + docs CI gate (former S18–S22)** — deferred
  to **the docs-hardening plan** (a later feature plan) per eng-review scope reduction (2026-05-28). AT12,
  IT6, IT10, IT11 move with them. S17's contract tests use hand-built golden fixtures until the
  docs-hardening plan tightens to a spec-driven oracle. Risk owned: spec-vs-code drift window between
  this plan's land and the docs-hardening plan's land.

## Endpoint map (source → rewrite)

URL nouns follow the renamed domain (Section 7): `journal-entries` (was `ledgers`), `journal-lines`
(was `ledger-items`), `accounts` (was `code-of-accounts`/`coa`). Clean break — accountants see the new
nouns from day one; no legacy aliases. (Decision per /plan-design-review redirect 2026-05-28.)

| Source (Java)                                   | Rewrite (Go/Gin)                                      | Service |
|-------------------------------------------------|------------------------------------------------------|---------|
| `POST /command/ledger`                          | `POST /command/journal-entries`                       | command |
| `PUT /command/ledger`                           | `PUT /command/journal-entries/:id`                    | command |
| `DELETE /command/ledger?uuid=`                  | `DELETE /command/journal-entries/:id`                 | command |
| `GET /command/health`                           | `GET /command/health`                                 | command |
| `GET /query/api/ledgers/all`                    | `GET /query/journal-entries?limit=&cursor=`           | query   |
| `GET /query/api/ledgers?uuid=`                  | `GET /query/journal-entries/:id`                      | query   |
| `GET /query/api/ledger-items?uuid=`             | `GET /query/journal-lines/:id`                        | query   |
| `POST /query/available-coa/json`                | `GET /query/accounts`                                 | query   |
| `POST /query/outstanding/`                      | `GET /query/balances?account=10000&account=11000`     | query   |
| `GET /query/api/balance-sheet-statement`        | `GET /query/statements/balance-sheet`                 | query   |
| `GET /query/api/profit-loss-statement`          | `GET /query/statements/profit-loss`                   | query   |
| `GET /query/health`                             | `GET /query/health`                                   | query   |

## API Conventions (locked in this plan — applied to every endpoint)

### Status codes
- **200** GET success · **201** POST creation success · **204** DELETE success (no body)
- **400** syntactic errors: malformed JSON, missing required field, wrong type, invalid cursor
- **401** missing/invalid JWT (any non-`/health` endpoint)
- **403** owner-scope violation (token's `sub` ≠ resource owner)
- **404** resource not found OR not visible to owner (do not distinguish — prevents enumeration)
- **409** optimistic-concurrency conflict on PUT (see ETag rule below)
- **415** request `Content-Type` ≠ `application/json` on POST/PUT
- **422** semantic errors: business-rule violation (unbalanced, missing required dimension, mixed currency, unknown account code, malformed Idempotency-Key)
- **500** server error (catch-all; OTel span carries detail)

### Error envelope (RFC 7807 Problem Details — `application/problem+json`)

```json
{
  "type": "https://reckonna.dev/errors/unbalanced-entry",
  "title": "Unbalanced journal entry",
  "status": 422,
  "code": "unbalanced_entry",
  "detail": "Sum of debits (1000) does not equal sum of credits (500)",
  "instance": "/command/journal-entries",
  "errors": [
    { "line_index": 0, "field": "amount", "issue": "debit_credit_mismatch" }
  ]
}
```

- `code` is the locale-neutral key (snake_case, stable across releases). Client maps to localized message via `locales/<lang>.json`.
- `title` + `detail` are English fallback; server reads `Accept-Language` to swap when locale exists.
- `errors[]` lists per-line / per-field issues. Empty if whole-payload error.
- Tests assert on `code` + `status`, NEVER on `title`/`detail` text (locale-fragile).

Error code registry (v1): `unbalanced_entry`, `mixed_currency`, `missing_required_dimension`,
`unknown_account_code`, `invalid_cursor`, `concurrency_conflict`, `duplicate_idempotency_key`,
`unsupported_media_type`, `unauthorized`, `forbidden`, `not_found`, `validation_failed`.

### Idempotency (POST /command/journal-entries only)

- Client may send `Idempotency-Key: <uuid-v4>` header.
- Server stores `(key, owner_sub, request_hash, response_status, response_body, created_at)` in
  `idempotency_record` table, TTL 24h.
- On replay (same key + same `sub` + same request hash): return cached response unchanged.
- Replay with same key but DIFFERENT body → 422 `duplicate_idempotency_key`.
- Missing header → allowed (idempotency is opt-in for v1; encourage in client docs).

### Optimistic concurrency (PUT /command/journal-entries/:id)

- `GET` response includes `ETag: "<version>"` header where `<version>` is a monotonic int on
  `journal_entry`.
- `PUT` MUST include `If-Match: "<version>"` header. Server compares; mismatch → 409
  `concurrency_conflict` with current version in body.
- `If-Match` missing → 428 Precondition Required (force the client to opt in).

### Cursor pagination (GET /query/journal-entries)

Request: `?limit=<1..200>&cursor=<base64-uuidv7>`. Default `limit=50`. Invalid/expired cursor → 400 `invalid_cursor`.

Response:
```json
{
  "items": [ { /* journal entry */ } ],
  "next_cursor": "<base64-uuidv7>",
  "has_more": true
}
```
- `next_cursor` is `null` on the last page.
- `has_more` is redundant with `next_cursor != null` but explicit for client ergonomics.

### Content negotiation

- POST/PUT MUST send `Content-Type: application/json`; otherwise 415.
- Response always `application/json` (or `application/problem+json` on error).
- Server sends `Vary: Accept-Language` if it ever localizes (set unconditionally to be safe).

### Balance query param shape

`GET /query/balances?account=10000&account=11000` — repeated query param (Gin `c.QueryArray("account")`).
Max 50 accounts per request (server enforces, 400 otherwise). Empty → 400 `validation_failed`
(no implicit "all accounts").

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
`required_dimensions` (optional), plus the §6-**Required** attributes `description`, `ifrs_line_item`,
`allowed_books`, `owner` (all four MUST be present on every account; the S5a validator enforces §6
completeness — see Step notes). Currency-neutral (§4). 5-digit gapped codes per §4 ranges.
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
(ifrs/gaap) + the mapping layer + cross-book consolidation are LATER features — not in this plan.

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
| AT2  | Given valid JWT / When POST unbalanced entry (debit 1000 + credit 500) / Then 422 + Content-Type `application/problem+json` + body `{code:"unbalanced_entry", status:422, errors:[{line_index:0, field:"amount", issue:"debit_credit_mismatch"}], ...}` (test asserts on `code` only — locale-fragile fields ignored) | e2e/journal_entry_invalid.e2e_test.go |
| AT3  | Given two owners' ledgers / When owner A GET /query/ledgers / Then only A's rows returned                    | e2e/ledger_owner_scope.e2e_test.go       |
| AT4  | Given ledger owned by A / When B PUT/DELETE it / Then 403 AccessDenied                                       | e2e/ledger_ownership.e2e_test.go         |
| AT5  | Given ledger with N items / When DELETE ledger / Then ledger + all items gone (cascade)                      | e2e/ledger_delete.e2e_test.go            |
| AT6  | Given balanced ledgers / When GET /query/statements/balance-sheet / Then assets == liabilities + equity      | e2e/balance_sheet.e2e_test.go            |
| AT7  | Given revenue+expense ledgers / When GET /query/statements/profit-loss / Then netIncome == revenue − expenses | e2e/profit_loss.e2e_test.go              |
| AT8  | Given no/invalid JWT / When any non-health endpoint / Then 401                                               | e2e/auth_unauthorized.e2e_test.go        |
| AT9  | Given seeded CoA / When GET /query/code-of-accounts / Then full chart returned                               | e2e/coa_list.e2e_test.go                 |
| AT10 | Given POST ledger referencing unknown coa / When submit / Then 422 (FK/validation)                          | e2e/ledger_bad_coa.e2e_test.go           |
| AT11 | Given decimal amounts (1000.3333 exact-fit, 1000.33335 boundary, 0.12345 sub-cent) / When round-trip create→read / Then NUMERIC(20,4) rounding policy is explicit and stable (no float drift) | e2e/money_precision.e2e_test.go (table-driven, 3 cases) |
| AT12 | _Deferred to the docs-hardening plan — Swagger UI at /docs depends on S18/S19_ | _e2e/docs_served.e2e_test.go (docs-hardening plan)_  |
| AT13 | Given currency dimension / When POST single-currency USD entry (debit 1000 + credit 1000 USD) / Then 201, balanced | e2e/entry_currency_dim.e2e_test.go |
| AT14 | Given lines with mixed currencies in one entry / When POST / Then 422 + `code:"mixed_currency"` (v1: entry must be single-currency) | e2e/entry_mixed_currency.e2e_test.go |
| AT15 | Given POST with Idempotency-Key X and balanced body / When same client retries same key + same body / Then second response is the cached 201, no second row in journal_entry | e2e/idempotency_replay.e2e_test.go |
| AT15b| Given POST with Idempotency-Key X / When same key sent with DIFFERENT body / Then 422 + `code:"duplicate_idempotency_key"` | e2e/idempotency_conflict.e2e_test.go |
| AT16 | Given GET journal-entry returns `ETag: "3"` / When PUT with `If-Match: "2"` / Then 409 + `code:"concurrency_conflict"` + current version in body | e2e/concurrency_conflict.e2e_test.go |
| AT16b| Given valid PUT body / When `If-Match` header missing / Then 428 + `code:"validation_failed"` (precondition required) | e2e/concurrency_precondition.e2e_test.go |
| AT17 | Given POST with `Content-Type: text/plain` / When server receives / Then 415 + `code:"unsupported_media_type"` (no body parsing attempted) | e2e/content_type_415.e2e_test.go |
| AT18 | Given GET /query/journal-entries with `Accept-Language: ja` and an error case / When server responds / Then `code` is locale-neutral AND `title`+`detail` are Japanese | e2e/i18n_error.e2e_test.go |

## Section 2 — Integration-test spec   [from architecture / CQRS contracts]

| ID  | Condition to verify                                                                                  | Test file                                  |
|-----|-----------------------------------------------------------------------------------------------------|--------------------------------------------|
| IT1 | command.PostLedger → query.GetLedger returns same id; items sum debit == sum credit                  | internal/query/ledger_it_test.go           |
| IT2 | tx fails mid-write (2nd item insert errors) → NO partial rows remain (atomic rollback)                | internal/repository/ledger_tx_test.go      |
| IT3 | DB CONSTRAINT TRIGGER rejects an unbalanced multi-row insert even if domain check is bypassed         | internal/repository/balance_trigger_test.go|
| IT4 | OIDC middleware validates JWT against Keycloak JWKS; bad sig/issuer/aud/expiry → 401                  | internal/handler/middleware/auth_test.go   |
| IT5 | Query read paths filter by owner `sub`; no cross-owner leakage in SQL                                 | internal/query/owner_scope_test.go         |
| IT6 | _Deferred to the docs-hardening plan (openapi.yaml not in this plan) — S17 uses hand-built golden response fixtures until then_ | _internal/handler/contract_test.go (docs-hardening plan tightens to spec-driven)_ |
| IT7 | balance-sheet & profit-loss aggregates group by CoA element correctly                                 | internal/query/statement_test.go           |
| IT8 | migrations up→down→up idempotent; `sqlc generate` compiles; CHECK trigger present                     | db/migration/migrate_test.go               |
| IT9 | Query service has NO write path — compile-time guarantee via split sqlc packages: cmd/query imports only `internal/repository/query` (SELECTs only); a package-import assertion test fails if cmd/query directly imports `internal/repository/command` | internal/query/readonly_test.go (uses go/parser to walk cmd/query imports) |
| IT10| _Deferred to the docs-hardening plan — tbls ERD generation lives there_ | _ci: docs job (docs-hardening plan)_ |
| IT11| _Deferred to the docs-hardening plan — depends on openapi.yaml landing first_ | _internal/handler/openapi_coverage_test.go (docs-hardening plan)_ |
| IT12| a journal line missing a required dimension (counterparty on 21500) is rejected (422)                 | internal/handler/dimension_required_test.go|
| IT13| balance trigger validates Σ debit = Σ credit per (entry, book); a mixed-currency entry is rejected     | internal/repository/book_balance_test.go   |
| IT14| every `config/coa.yaml` code has a translation in each shipped locale (en, ja) AND every error `code` from the registry has a translation in each shipped locale | internal/config/i18n_coverage_test.go |
| IT15| Content-Type middleware rejects non-`application/json` POST/PUT before handler runs; OPTIONS preflight allowed | internal/handler/middleware/content_type_test.go |
| IT16| Idempotency table: composite UNIQUE on (key, owner_sub); replay returns cached row; TTL cleanup query works | internal/handler/middleware/idempotency_test.go |
| IT17| version trigger bumps `journal_entry.version` on every UPDATE; PUT handler reads it into ETag header     | internal/repository/command/version_trigger_test.go |
| IT18| rounding-divergence: domain balance check and the DB CONSTRAINT TRIGGER agree at NUMERIC(20,4). An entry balanced at full precision but unbalanced after 4dp rounding (and the reverse) is rejected by BOTH layers — never accepted by one while the other would reject | internal/repository/rounding_divergence_test.go |

## Section 3 — Implementation steps (one commit each; unit test per step)

One step = one commit = one PR-reviewable change. Each compiles & passes on its own. TDD visible:
failing-test commit BEFORE code commit for domain/money/auth paths. Every commit carries `Plan: S<n>`.

| ID  | Commit message (verbatim)                                       | Files                                                                 | Depends      | Unit test                              |
|-----|----------------------------------------------------------------|----------------------------------------------------------------------|--------------|----------------------------------------|
| S1  | `test(domain): failing Ledger balance + decimal money tests`   | internal/domain/ledger_test.go                                       | 00           | TestNewLedger/unbalanced, /decimal (RED)|
| S2  | `feat(domain): Ledger, LedgerItem, CoA entities + invariant`   | internal/domain/ledger.go, internal/domain/coa.go, internal/domain/money.go | S1   | TestNewLedger (GREEN); ErrUnbalanced    |
| S3  | `feat(db): schema — account, journal_entry/line, dimension(_type/value), book` | db/migration/001_accounting.up.sql, 001_accounting.down.sql | 00 | migrate up/down (IT8)        |
| S4  | `feat(db): deferred balance TRIGGER per (entry,book) (借方=貸方)` | db/migration/002_balance_check.up.sql, 002_balance_check.down.sql | S3      | balance_trigger_test (IT3)              |
| S5  | `feat(db): sqlc queries split per CQRS side (command writes, query reads-only)` | db/query/command/ledger.sql, db/query/query/ledger.sql, db/query/query/coa.sql, db/query/query/statement.sql; sqlc.yaml generates two packages (internal/repository/command, internal/repository/query) | S3 | `sqlc generate` produces two packages; cmd/query build imports only ./query (compile-time CQRS purity — replaces IT9 grep) |
| S5a | `chore(tooling): gen-coa (coa.yaml → seed + locale stubs + validate)` | scripts/gen-coa.go, Makefile                                 | -            | validates config/coa.yaml vs standard   |
| S5b | `feat(db): seed account (generated from config/coa.yaml)` | db/migration/003_seed_account.up.sql, 003_seed_account.down.sql  | S3,S5a       | 20 accounts; req-dim on 21500 (AT6/7/9) |
| S5c | `feat(db): seed base book + dimension types/values` | db/migration/004_seed_dimensions.up.sql, 004_seed_dimensions.down.sql | S3      | base book + currency members (JPY default) |
| S6  | `feat(repository): ledger command repo (thin sqlc wrappers)`   | internal/repository/command/ledger_repo.go                           | S2,S5        | ledger_repo_test (basic CRUD)           |
| S6a | `feat(service): tx orchestration helper (UoW-lite)`            | internal/service/tx.go                                               | S6           | tx test: rollback on mid-step error (IT2) |
| S7  | `feat(service): PostLedger use case (validate → atomic write)` | internal/service/ledger_command.go                                   | S6,S6a       | TestPostLedger                          |
| S8  | `feat(handler): POST /command/journal-entries + DTO + RFC 7807 errors` | internal/handler/journal_entry_command.go, internal/handler/dto.go, internal/handler/errors.go (Problem Details writer + error-code registry) | S7 | handler test (AT1, AT2, AT10) — assert on `code` field, not localized text |
| S8a | `feat(middleware): Content-Type 415 + Accept-Language`         | internal/handler/middleware/content_type.go, internal/handler/middleware/i18n.go | S8 | IT15 (415 on text/plain POST), Accept-Language swap for `title`/`detail` |
| S8b | `feat(handler): Idempotency-Key middleware for POST journal-entries` | internal/handler/middleware/idempotency.go, db/migration/005_idempotency.up.sql (table + index), db/query/command/idempotency.sql | S8 | AT15 (replay same key returns cached 201); AT15b (replay different body → 422 `duplicate_idempotency_key`) |
| S9  | `feat(service,handler): Update/Delete journal-entry + ownership + ETag/If-Match` | internal/service/journal_entry_command.go, internal/handler/journal_entry_command.go (PUT requires If-Match, returns 409 on mismatch; GET emits ETag header from version column) | S8,S9a | AT4, AT5, AT16 (409 on stale If-Match), AT16b (428 if If-Match missing on PUT) |
| S9a | `feat(db): version column on journal_entry for optimistic concurrency` | db/migration/006_journal_entry_version.up.sql (ADD COLUMN version INT NOT NULL DEFAULT 1; trigger to bump on UPDATE) | S3 | migrate up/down; concurrent-update test asserts version increments |
| S10 | `feat(auth): Keycloak OIDC middleware (JWKS, sub claim)`       | internal/handler/middleware/auth.go, internal/config/oidc.go        | 00, kc-prereq| auth_test mock-JWKS (IT4, AT8)          |
| S11 | `feat(query): GetLedger + ListLedgers owner-scoped reads (cursor pagination)` | internal/query/ledger_query.go, internal/handler/ledger_query.go (uses ?limit=N&cursor=<uuidv7>; server max=200) | S5,S10 | ledger_it_test (IT1, IT5, AT3) + AT3a cursor pagination |
| S12 | `feat(query): ledger-item get + CoA list + outstanding balances`| internal/query/coa_query.go, internal/handler/coa_query.go         | S11          | coa tests (AT9)                         |
| S13 | `feat(query): balance-sheet statement aggregate`              | internal/query/statement_query.go, internal/handler/statement.go    | S12          | TestBalanceSheet (AT6, IT7)             |
| S14 | `feat(query): profit-loss statement aggregate`               | internal/query/statement_query.go, internal/handler/statement.go    | S13          | TestProfitLoss (AT7, IT7)               |
| S15 | `feat(cmd): wire command + query Gin routers + config`        | cmd/command/main.go, cmd/query/main.go, internal/config/config.go   | S9,S14,S10   | boots; health 200; readonly_test (IT9)  |
| S16 | `feat(obs): OTel spans on command + query handlers (otelgin)`| internal/config/otel.go, cmd/command/main.go, cmd/query/main.go (otelgin middleware at router setup auto-traces every route) | S15 | trust otelgin (well-tested upstream); manual OTLP smoke-test verifies spans on /command/ledgers + /query/ledgers (devops Done) |
| S17.1 | `test(e2e): command write-path suite (create/update/delete/idempotency/concurrency)` | e2e/journal_entry_*.e2e_test.go, internal/handler/contract_test.go (command golden fixtures until the docs-hardening plan's openapi.yaml lands) | S15 | AT1, AT2, AT4, AT5, AT10, AT11, AT13, AT14 + IT2, IT3, IT12, IT13, IT18 green (testcontainers-go) |
| S17.2 | `test(e2e): query + statement suite (list/coa/balance-sheet/profit-loss)` | e2e/ledger_query_*.e2e_test.go, e2e/statement_*.e2e_test.go | S15 | AT3, AT6, AT7, AT9 + IT1, IT5, IT7 green (testcontainers-go) |
| S17.3 | `test(e2e): auth + migration + CQRS-purity suite` | e2e/auth_*.e2e_test.go, db/migration/migrate_test.go, internal/query/readonly_test.go, internal/config/i18n_coverage_test.go | S15 | AT8 + IT4, IT8, IT9, IT14 green (testcontainers-go) |
| S17b| `feat(release): multi-stage Dockerfile per service + CI build/push` | build/Dockerfile.command, build/Dockerfile.query (distroless static Go), .github/workflows/ci.yml (image job → ghcr.io tagged by commit SHA) | S15 | docker build green; image runs `--health` clean; ghcr.io publish on push to main |

### Step notes
- **Tier-0 domain reconcile (PREREQUISITE before S6 — F2):** the domain that landed on
  `feat/01-backend-cqrs-core` is the pre-rename `Ledger`/`LedgerItem`/`NewLedger` model (balance
  invariant only). Before any S6 repository work, S2 must be completed to the renamed
  `JournalEntry`/`JournalLine`/`NewEntry(lines)` model with `money.go`, `coa.go`, currency/dimension
  values, and all three error types (`ErrUnbalanced`, `ErrMixedCurrency`, `ErrRequiredDimension`).
  Building S6/S7/S8 on the old shape forces a redo.
- **Migration numbering (single authority — F4):** migration files follow ONE monotonic sequence
  owned by the backend HEAD: 001 (S3) · 002 (S4) · 003 (S5b) · 004 (S5c) · 005 (S8b) · 006 (S9a).
  These land in different PRs, so the HEAD assigns the next free number at merge time; a colliding
  branch rebases + renumbers. Never reuse, skip, or edit an applied migration (migrations.md).
- **RED-first on invariant/money steps (F5):** per tdd.md, S4 (balance trigger) and S7 (PostLedger
  use case) each commit a FAILING test BEFORE their feat commit — same Red→Green pattern as S1→S2.
  The unit-test column names the test; the failing-test commit precedes the listed feat commit.
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
  code/name, required_dimensions exist, **and §6-Required completeness — every account carries
  `description`, `ifrs_line_item`, `allowed_books`, `owner`**), then emits `003_seed_account.up/down.sql` and stubs any missing
  `locales/*.json` key. Run in CI so a hand-edited seed or an untranslated account fails the build.
- **S5 sqlc money mapping:** override `numeric` → `github.com/shopspring/decimal.Decimal` in sqlc.yaml
  (pgx/v5 + decimal). CoA `coa` is `int`; `element`/`type` are PG enums → Go typed constants.
- **S10 Keycloak (cross-domain prereq):** middleware validates RS256 JWT against Keycloak's JWKS
  (`/.well-known/openid-configuration`), checks issuer + audience + expiry, extracts `sub` → owner id.
  Unit-tested against a **mock JWKS / testcontainers Keycloak** so it is green WITHOUT the real infra
  Keycloak. Real Keycloak issuer URL comes from Vault-rendered config at runtime (never hardcoded).
- **S15 CQRS purity:** `cmd/query` links only read repos. IT9 greps the query build for
  INSERT/UPDATE/DELETE and fails if found — enforces "query MUST NOT mutate".
- **S17:** integration/e2e via testcontainers-go (Postgres). The migrations + trigger run against the
  real container, so AT/IT exercise the DB-level invariant, not a mock.

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

## Technical documentation (deferred to the docs-hardening plan)

This plan ships handlers + tests + Dockerfiles only. The full spec-first docs stack (OpenAPI + oapi-codegen,
tbls ERD + diff gate, mermaid sequence diagrams, Swagger UI at /docs, graphify re-index) moves to
**the docs-hardening plan** (a later feature plan). Until then, S17's contract tests rely on hand-built golden
response fixtures — intentional spec-vs-code drift window, owned by the docs-hardening plan.

(Original spec-first table moved to the docs-hardening plan. Source reference diagrams to port intent from:
`/tmp/accsrc/Design/create_sequence.md`, `7月_CQRS_patterns.drawio`.)

## Diagrams (added by eng-review 2026-05-28)

### A. POST /command/ledgers — write data flow

```
HTTP request (JWT, Content-Type: application/json, Idempotency-Key?)
   │
   ▼
┌──────────────────────────────────────────────────────────────────┐
│ otelgin (S16) ──► span: POST /command/journal-entries            │
│ content-type middleware (S8a) ──► 415 if not application/json    │
│ idempotency middleware (S8b) ──► cache hit? return cached 201    │
└──────────────────────────────────────────────────────────────────┘
   │
   ▼
┌──────────────────────────────────────────────────────────────────┐
│ auth middleware (S10) ──► JWKS verify → sub claim on ctx (401?)  │
└──────────────────────────────────────────────────────────────────┘
   │
   ▼
┌──────────────────────────────────────────────────────────────────┐
│ handler/journal_entry_command.go (S8): DTO → domain.NewEntry()   │
│   ErrUnbalanced │ ErrMixedCurrency │ ErrRequiredDimension → 422  │
│   RFC 7807 Problem Details envelope; locale-neutral `code`       │
└──────────────────────────────────────────────────────────────────┘
   │ entry value
   ▼
┌──────────────────────────────────────────────────────────────────┐
│ service.PostLedger (S7) ── BEGIN ─►  repository/command (S6)     │
│   1. INSERT journal_entry                                        │
│   2. INSERT journal_line × N                                     │
│   3. INSERT journal_line_dimension × M                           │
│   ── COMMIT  ──►  CONSTRAINT TRIGGER (S4) Σdebit=Σcredit         │
│       │ pass                  │ fail                             │
│       ▼                       ▼                                  │
│     201 Created             ROLLBACK → 422 (defense in depth)    │
└──────────────────────────────────────────────────────────────────┘
```

### B. CQRS write → read — same schema, separate builds

```
┌──────────────────────┐                ┌─────────────────────┐
│ cmd/command (S15)    │                │ cmd/query (S15)     │
│   imports:           │                │   imports:          │
│   internal/          │                │   internal/         │
│     repository/      │                │     repository/     │
│       command  ◄─────┤                ├───►  query          │
│       query    ◄─────┤  (read-only)   │                     │
│     service          │                │     query           │
└──────────┬───────────┘                └──────────┬──────────┘
           │ writes                                │ reads
           ▼                                       ▼
       ┌───────────────────────────────────────────────┐
       │     PostgreSQL — single `accounting` schema    │
       │  journal_entry · journal_line · account · ...  │
       │  CHECK CONSTRAINT TRIGGER (deferred) at COMMIT │
       └───────────────────────────────────────────────┘

Compile-time CQRS purity (S5, IT9): cmd/query has NO transitive path
to repository/command. Enforced by go/parser walk over cmd/query/main.go
imports — fails CI if command pkg ever appears.
```

### C. OIDC middleware — JWT → sub claim state machine

```
       request
          │
          ▼
   Authorization header?
       │             │
      no            yes
       │             │
       ▼             ▼
     401      Bearer token
                     │
                     ▼
              JWKS cache hit?
                  │      │
                  no    yes
                  │      │
        ┌─────────┘      │
        ▼                │
  GET issuer/.well-known │
  → fetch JWKS (cached   │
  with TTL)              │
        │                │
        └────────┬───────┘
                 ▼
            verify RS256 sig
                 │
        invalid  │  valid
            │    │
            ▼    ▼
          401   check iss / aud / exp
                 │
        any fail │ all pass
            │    │
            ▼    ▼
          401   extract sub → ctx
                 │
                 ▼
             next handler
```

## Hand-off to the heads
- **backend-engineer (HEAD):** owns S1–S17b (incl S5a/S5b/S5c, S6a, S8a/S8b, S9a). Write AT1–AT11,
  AT13–AT18 + IT1–IT5, IT7, IT8, IT9, IT12–IT17 as FAILING tests FIRST, then green via
  domain-modeler → tdd-implementer/migration-mapper → code-reviewer. Use `migration-mapper`
  for the Java→Go port (jOOQ→sqlc, Flyway→golang-migrate, Spring validation→domain+struct tags).
- **infra-engineer (HEAD) — NEW TASK (separate feature plan `infra/keycloak-oidc`):** stand up a
  self-hosted Keycloak as the OIDC provider (realm, client, audience, JWKS), expose issuer URL via
  Vault. Blocks S10 *at runtime* only — S10's tests run against mock/testcontainers Keycloak, so
  backend work is not gated on infra completion. Provision before e2e (S17) against a live stack.
- **plan-tracker:** logs each landed step to `03-backend-cqrs-core.impl.md`.

"Done" (this plan) = AT1–AT11, AT13–AT18 + IT1–IT5, IT7, IT8, IT9, IT12–IT17 green;
`make test` race-clean; `make lint` clean; cmd/query build proven read-only at compile time (IT9);
IT18 rounding-divergence green (domain + DB trigger agree at 4dp);
CoA seed generated from config/coa.yaml + validated vs the governance standard; all locales cover
every account; docker images publish to ghcr.io on push to main (S17b).
(AT12, IT6, IT10, IT11, Swagger UI, openapi.yaml, tbls ERD → the docs-hardening plan.)

## NOT in scope (this plan)
- `POST/PUT/DELETE /command/ledger_items` direct endpoints (invariant-bypassing — deliberately dropped).
- Frontend, infra/k8s, runtime Vault rendering, Keycloak provisioning (separate feature plans).
- OpenAPI spec, Swagger UI, oapi-codegen handler types — **deferred to the docs-hardening plan**.
- tbls ERD generation + `tbls diff` CI gate — **deferred to the docs-hardening plan**.
- Mermaid sequence diagrams in `docs/sequences/` (this plan ships ASCII diagrams inline) — **docs-hardening plan**.
- graphify re-index CI step — **docs-hardening plan**.
- Multi-tenant organisation/workspace model (owner = JWT `sub` only in v1).
- Cross-currency entries + functional-currency consolidation (books/measurement phase, plan TBD).
- Delta books, mapping layer, multi-framework (IFRS/GAAP/TFRS) — **later feature plans**.

## What already exists
- Plan 00 landed (`cade037`, `cbc247f`): `go.mod` with pinned deps, sqlc.yaml, golang-migrate, Makefile
  with empty-state guards, docker-compose, `tests/no-secrets_test.sh`, gitleaks CI gate. Nothing in
  `internal/`, `cmd/`, or `db/query/` yet — plan 03 is greenfield Go.
- `doc/coa-governance-standard.md` exists; `config/coa.yaml` does not — S5a creates the generator and
  S5b consumes the yaml. `locales/en.json` + `locales/ja.json` likely stub-only at plan 00 close.
- Source Java code referenced via `source_ref` (springboot_cqrs_command @ aa24678, _query @ 59ff36b)
  for the Java→Go port — read-only reference, not imported.

## Implementation Tasks
Synthesized from this eng-review's findings. Each derives from a specific finding above.
Run with Claude Code or Codex; checkbox as you ship.

- [ ] **T1 (P1, human: ~1h / CC: ~10min)** — `internal/service` — move tx orchestration out of repository per D3
  - Surfaced by: Architecture Review — S6 contradicted CLAUDE.md ("service owns tx").
  - Files: `internal/service/tx.go` (new S6a), `internal/repository/command/ledger_repo.go` (accept `pgx.Tx` / `DBTX`).
  - Verify: `internal/repository/ledger_tx_test.go` (renamed from plan); rollback case exercises IT2.
- [ ] **T2 (P1, human: ~1h / CC: ~15min)** — `internal/repository` — split sqlc into command + query packages per D4
  - Surfaced by: Architecture Review — grep-based CQRS purity check was brittle.
  - Files: `sqlc.yaml` (two `gen:` blocks), `db/query/command/*.sql`, `db/query/query/*.sql`, `internal/repository/command`, `internal/repository/query`.
  - Verify: `sqlc generate` produces two packages; `internal/query/readonly_test.go` walks cmd/query imports via go/parser and fails if `repository/command` appears.
- [ ] **T3 (P2, human: ~15min / CC: ~3min)** — `cmd/command/main.go`, `cmd/query/main.go` — wire otelgin per D5
  - Surfaced by: Architecture Review — S16 Files col contradicted body.
  - Files: `internal/config/otel.go`, both `cmd/*/main.go` (otelgin middleware at router setup).
  - Verify: Manual OTLP smoke run; spans appear for `/command/ledgers` and `/query/ledgers` (trust otelgin per D9).
- [ ] **T4 (P2, human: ~2h / CC: ~20min)** — `build/` — Dockerfiles + CI image build per D6
  - Surfaced by: Architecture Review — distribution gap (no container build).
  - Files: `build/Dockerfile.command`, `build/Dockerfile.query` (distroless static Go), `.github/workflows/ci.yml` (image job → ghcr.io tagged by SHA).
  - Verify: `docker build` green on PR; `ghcr.io/<owner>/<repo>-{command,query}:<sha>` published on push to main.
- [ ] **T5 (P2, human: ~30min / CC: ~5min)** — `plans/01-backend-cqrs-core.md` body — 3 ASCII diagrams per D7
  - Surfaced by: Code Quality Review — plan lacked visual data-flow / state-machine maps.
  - Files: This plan (already added in the eng-review write).
  - Verify: Diagrams render in plain text; team can read intent from plan without external references.
- [ ] **T6 (P2, human: ~15min / CC: ~3min)** — `e2e/money_precision.e2e_test.go` — AT11 boundary cases per D8
  - Surfaced by: Test Review — single 1000.3333 case left rounding policy undefined.
  - Files: `e2e/money_precision.e2e_test.go` (table-driven: 1000.3333 + 1000.33335 + 0.12345).
  - Verify: All three cases pass with explicit, documented NUMERIC(20,4) rounding policy.
- [ ] **T7 (P1, human: ~1h / CC: ~10min)** — `internal/handler/ledger_query.go` — cursor pagination per D10
  - Surfaced by: Performance Review — unbounded ListLedgers.
  - Files: `internal/handler/ledger_query.go`, `internal/query/ledger_query.go`, `db/query/query/ledger.sql` (cursor predicate on UUIDv7), DTO with `next_cursor`.
  - Verify: AT3a (new): 250 ledgers per owner, page through with limit=50, last page omits `next_cursor`; invalid cursor → 400.

### Added by API surface review (2026-05-28)

- [ ] **T8 (P1, human: ~3h / CC: ~30min)** — `internal/handler/errors.go` + endpoint renames — RFC 7807 envelope + noun rename
  - Surfaced by: API surface findings #1–#5 + #9.
  - Files: rename handlers (`ledger_*` → `journal_entry_*`); new `internal/handler/errors.go` (Problem Details writer + error-code registry); update every handler to emit `application/problem+json`; update endpoint paths (`/journal-entries`, `/journal-lines`, `/accounts`); update sqlc query files to match.
  - Verify: AT2 + AT14 assert `code` only (not text); IT14 asserts every code has en + ja translation.
- [ ] **T9 (P1, human: ~3h / CC: ~25min)** — `internal/handler/middleware/idempotency.go` + DB table — Idempotency-Key on POST
  - Surfaced by: API surface finding #6 — accounting double-post risk.
  - Files: `db/migration/005_idempotency.up.sql` (`idempotency_record(key text, owner_sub text, request_hash text, response_status int, response_body jsonb, created_at timestamptz, PRIMARY KEY(key, owner_sub))` + index on `created_at` for TTL cleanup), `db/query/command/idempotency.sql`, middleware.
  - Verify: AT15 cached replay returns 201 with same body; AT15b different-body replay → 422 `duplicate_idempotency_key`; IT16 unique constraint on (key, owner_sub).
- [ ] **T10 (P1, human: ~2h / CC: ~20min)** — `journal_entry.version` + ETag/If-Match — optimistic concurrency
  - Surfaced by: API surface finding #7 — silent overwrite under PUT.
  - Files: `db/migration/006_journal_entry_version.up.sql` (ADD COLUMN version + UPDATE trigger), `internal/handler/journal_entry_command.go` (PUT: read If-Match, compare, 409 on mismatch, 428 if absent; GET emits ETag), `db/query/command/journal_entry.sql` (UPDATE includes WHERE version = $expected).
  - Verify: AT16 (409 on stale If-Match), AT16b (428 if missing), IT17 (version trigger bumps).
- [ ] **T11 (P1, human: ~1h / CC: ~10min)** — `internal/handler/middleware/content_type.go` — 415 on non-JSON
  - Surfaced by: API surface finding #10.
  - Files: middleware checks `Content-Type: application/json` on POST/PUT before handler; OPTIONS preflight passes through.
  - Verify: AT17 (text/plain POST → 415), IT15 (middleware unit test).
- [ ] **T12 (P2, human: ~1h / CC: ~10min)** — `internal/handler/middleware/i18n.go` — Accept-Language → localized title/detail
  - Surfaced by: API surface finding #5 — Japanese hard-coded in error tests.
  - Files: middleware reads `Accept-Language`, attaches preferred locale to context; `errors.go` swaps `title`/`detail` per `locales/<lang>/errors.json` (new file, keyed by error `code`); fallback to en.
  - Verify: AT18 (Japanese error body for `Accept-Language: ja`), IT14 (every code has en+ja translation).
- [ ] **T13 (P2, human: ~30min / CC: ~5min)** — `db/query/query/balance.sql` + handler — repeated `?account=` param + max 50 cap
  - Surfaced by: API surface finding #3.
  - Files: handler reads `c.QueryArray("account")`, validates 1–50 entries, passes to sqlc query as `int[]`.
  - Verify: empty array → 400 `validation_failed`; 51+ entries → 400.
- [ ] **T14 (P2, human: ~30min / CC: ~5min)** — cursor pagination response shape — augment T7
  - Surfaced by: API surface finding #8.
  - Files: response DTO `{items, next_cursor, has_more}`; document max=200, default=50 in handler + plan.
  - Verify: last page returns `next_cursor: null, has_more: false`.
- [ ] **T15 (P1, no-code DECISION, human: ~10min / CC: ~2min)** — `plans/01-backend-cqrs-core.md` already locks the 400 vs 422 rule
  - Surfaced by: API surface finding #9.
  - Files: (none — rule is in "## API Conventions" section).
  - Verify: every error-emitting test asserts the documented status code per the rule.
- [ ] **T16 (P2, human: ~15min / CC: ~3min)** — `404 conflates not-found-vs-not-visible` policy doc
  - Surfaced by: API surface review — enumeration-attack prevention.
  - Files: comment in `internal/handler/middleware/auth.go` explaining the rule + a test asserting that cross-owner GET returns 404 (not 403).
  - Verify: new test in IT5 group asserts owner B requesting owner A's `/journal-entries/:id` gets 404, not 403 (403 leaks existence).

## GSTACK REVIEW REPORT

| Review | Trigger | Why | Runs | Status | Findings |
|--------|---------|-----|------|--------|----------|
| CEO Review | `/plan-ceo-review` | Scope & strategy | 0 | — | — |
| Codex Review | `/codex review` | Independent 2nd opinion | 0 | — | — |
| Eng Review | `/plan-eng-review` | Architecture & tests (required) | 1 | CLEAR (PLAN) | 8 issues raised + 10 API-surface findings; 16 implementation tasks queued; scope reduced (S18–S22 → the docs-hardening plan) |
| Design Review | `/plan-design-review` | UI/UX gaps | 0 | N/A (backend-only) | re-invocations redirected to ad-hoc API-surface review (2026-05-28) |
| DX Review | `/plan-devex-review` | Developer experience gaps | 0 | — | — |

- **API surface (ad-hoc 2026-05-28):** 10/10 findings folded. New steps S8a (Content-Type), S8b (Idempotency-Key), S9a (version column). New ATs AT15–AT18 (idempotency replay/conflict, concurrency 409/428, 415, i18n error). New ITs IT15–IT17 (content-type, idempotency table, version trigger). Locked: RFC 7807 envelope, error-code registry, 400 vs 422 rule, repeated `?account=` query, cursor response shape, 404-not-403 on cross-owner. URL nouns now match domain (`journal-entries`, `journal-lines`, `accounts`).
- **UNRESOLVED:** 0 (all D-numbered + API decisions answered)
- **VERDICT:** ENG CLEARED (PLAN) — ready to implement S1 once Plan 00 PR lands. Design review N/A (backend only). CEO review not required. Outside voice skipped at user request.
