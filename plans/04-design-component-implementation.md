---
feature: 04-design-component-implementation
status: draft   # require-prereq.sh greps this — flip to "approved" only via /plan-design-review + /plan-eng-review + human sign-off
approved_by: TBD
approved_at: TBD
domain: frontend
depends_on:
  - 00-bootstrap-deps-vault          # go.mod/Makefile irrelevant here; RN/Expo scaffold (app/, components/, package.json) landed alongside plan 00
  - 03-backend-cqrs-core             # API contract: GET reads, POST /command/journal-entries, RFC 7807 errors, Keycloak OIDC
design_gate:
  spec: design/02-frontend-ledger.design-system.html   # THE gating design system (Forest Reserve tokens) — status: draft, needs `approved` marker
  brand: design/reckonna.design-system.html            # inherited brand tokens
unblocks:
  - 05-frontend-screens-routing      # the expo-router screen wiring + Keycloak auth flow lands on top of these components
worktree:
  branch: feat/04-design-component-implementation
  path: .claude/worktrees/frontend-04
  base: develop
decisions:
  scope: COMPONENT LIBRARY ONLY — the 8 design sections become reusable RN/Expo (RN Web) components + their tests. Screen wiring, routing, live API, and Keycloak are plan 05.
  stack: RN 0.81 + Expo 54 + expo-router 6 + react-native-paper 5 + react-hook-form 7 (already in package.json). RN Web for browser parity.
  tokens: Forest Reserve tokens ported 1:1 from the design HTML into a typed theme module — NO new color/type invented (design §01 rule).
  money: amounts are decimal-exact strings end-to-end (mirror backend NUMERIC(20,4)); display rounds to 2dp, value keeps 4dp. NO float64/number arithmetic on money.
  fonts: Source Serif 4 (serif/headings) + JetBrains Mono (mono/data) via @expo-google-fonts (offline-bundled, NOT the Google CDN <link> the HTML uses).
  svg: react-native-svg for the ledger/empty-state marks (the inline <svg> in the HTML).
  testing: @testing-library/react-native + jest-expo (already configured). Render + interaction tests; every money/balance component has an UNBALANCED case (借方≠貸方).
  states: every async-capable surface ships Loading + Empty + Error (design §04 sign-off rule) — enforced as a test requirement.
  data: components are PRESENTATIONAL — props in, callbacks out. NO axios/fetch inside components; hooks + live wiring are plan 05. Storybook-style harness optional.
  human_only: none in this plan (pure frontend code + tests; no terraform/kubectl).
review_log:
  - drafted 2026-06-29 from design/02-frontend-ledger.design-system.html; pending /plan-design-review + /plan-eng-review before status→approved.
---

# Plan 04 — Ledger Design-System Component Implementation (RN/Expo, Forest Reserve)

Turns the feature design system
(`design/02-frontend-ledger.design-system.html`, 8 sections) into a **typed, tested
RN/Expo component library** under `components/` + `theme/`, consolidated for native
(RN Paper, bottom tabs) and web (RN Web, top nav). Every monetary figure is
decimal-exact, right-aligned, tabular; **借方=貸方 is shown, computed, and used
to gate the Post CTA in the UI** before the backend (plan 03) re-enforces it at the DB.

**This plan ships components + their tests ONLY.** Screen routing, live API calls,
Keycloak auth, and OTel are **plan 05** (`05-frontend-screens-routing`). Components are
presentational: props in, callbacks out, zero network code.

**Prerequisites:** Plan 00 landed (RN/Expo scaffold: `app/`, `components/`,
`package.json` with expo-router/paper/react-hook-form/axios). Plan 03 (backend CQRS)
defines the API contract these components are shaped against (RFC 7807 error `code`s,
`journal-entries`/`journal-lines`/`accounts` nouns, decimal money).

## Decisions (locked at draft, 2026-06-29)

| # | Decision | Rationale |
|---|----------|-----------|
| 1 | **Component library scope only** — the 8 HTML sections → components + tests. Screens/routing/API/Keycloak deferred to plan 05. | Keeps each PR reviewable; lets design-review gate the visual layer before the integration layer. Mirrors plan 03's "handlers+tests, docs later" split. |
| 2 | **Forest Reserve tokens ported 1:1** into `theme/tokens.ts` + a RN Paper theme. No new color/type (design §01). | The HTML is the single visual source of truth; drift is a bug. A token test asserts the values match. |
| 3 | **Money is a decimal-exact string**, never a JS `number`. A small, dependency-free `money` helper formats (2dp display / 4dp value) + sums for the balance bar. | JS `number` is IEEE-754 float — exactly the precision bug plan 03 killed on the backend (`Double`→`NUMERIC`). The UI must not reintroduce it. |
| 4 | **Fonts bundled via `@expo-google-fonts`**, not the Google `<link>` the HTML preview uses. | Offline-first, deterministic in tests + native; no runtime CDN dependency. |
| 5 | **`react-native-svg`** for the ledger/empty marks (the inline `<svg>`s). | RN has no DOM SVG; this is the vendor-neutral standard already whitelisted in `jest.config.js` transformIgnorePatterns. |
| 6 | **Presentational components** (props/callbacks, no data fetching). Loading/Empty/Error are PROPS-DRIVEN states, not internal network state. | Plan 05 owns hooks + axios + Keycloak. Keeps these unit-testable without a server and reusable across screens. |
| 7 | **Every async-capable surface ships Loading + Empty + Error** (design §04 — "no screen merges without them"). Enforced by a per-component state test. | Design sign-off rule. A list/statement component without its three states fails review. |
| 8 | **TDD visible**: failing render/interaction test commit BEFORE the component for every money/balance path (BalanceBar, AmountInput, JournalEntryForm). Unbalanced (借方≠貸方) case mandatory. | `tdd.md` + Reckonna CLAUDE.md ("every exported func that touches money has a test including an unbalanced case"). |
| 9 | One step = one commit = one `Plan: S<n>` trailer. Conventional Commits. No squashing across steps. | `devops.md`. |

## File structure

```
plans/04-design-component-implementation.md       # this file
theme/
  tokens.ts                                        # Forest Reserve: colors, fonts, radii, shadows, state aliases
  paperTheme.ts                                    # maps tokens → react-native-paper MD3 theme
  tokens.test.ts                                   # asserts values match design HTML (anti-drift)
lib/
  money.ts                                         # decimal-exact string format (2dp display / 4dp value) + sum
  money.test.ts                                    # rounding + sum, incl. boundary 1000.33335 / 0.12345
components/
  Button.tsx           Button.test.tsx             # variants: primary/secondary/ghost/accent/danger/disabled
  Badge.tsx            Badge.test.tsx              # draft/review/posted/flagged
  Field.tsx            Field.test.tsx              # label + input/error wrapper; invalid state
  AmountInput.tsx      AmountInput.test.tsx        # right-aligned, tabular, 4dp under the hood
  AccountSelect.tsx    AccountSelect.test.tsx      # CoA picker (code · name)
  DebitCreditSegment.tsx  DebitCreditSegment.test.tsx  # 借方/貸方 toggle — drives color + balance sign
  Skeleton.tsx Spinner.tsx EmptyState.tsx Alert.tsx  + *.test.tsx   # §04 Loading/Empty/Error primitives
  BalanceBar.tsx       BalanceBar.test.tsx         # debit/credit/difference + ok|bad + Balanced pill (借方=貸方 gate)
  LedgerTable.tsx      LedgerTable.test.tsx        # general + subsidiary rows; status badge; +Loading/Empty/Error
  JournalEntryForm.tsx JournalEntryForm.test.tsx   # 2-step: sentence capture + line table + BalanceBar; CTA gated
  StatementTable.tsx   StatementTable.test.tsx     # grp/row/tot rows; BalanceSheet + ProfitLoss compositions
  AppNav.tsx           AppNav.test.tsx             # adaptive: bottom tabs (native) / top nav (web)
  index.ts                                         # barrel export
package.json                                       # + @expo-google-fonts/source-serif-4, /jetbrains-mono, react-native-svg
```

**Out of this plan (→ plan 05):** `app/**` expo-router screens, `app/hooks/**` data
hooks, axios client, Keycloak OIDC flow, owner-scoped fetching, OTel spans.

---

## Section 1 — Acceptance-test spec (E2E / component-flow)   [from the design system]

Component-level flow tests via `@testing-library/react-native` (render + fire events).
"E2E across screens" is plan 05; here AT = a full user-meaningful interaction within a
composed component.

| ID  | Given / When / Then | Domain | Test file |
|-----|---------------------|--------|-----------|
| AT1 | Given a JournalEntryForm with debit 1000 + credit 1000 / When rendered / Then BalanceBar shows `ok`, "✓ Balanced" pill, and the **Post/Review CTA is enabled**. | frontend | components/JournalEntryForm.test.tsx |
| AT2 | Given a JournalEntryForm with debit 1000 + credit 500 / When rendered / Then BalanceBar shows `bad`, "✕ 借方≠貸方", Difference 500.00, and the **CTA is `aria-disabled`/disabled** (cannot post). | frontend | components/JournalEntryForm.test.tsx |
| AT3 | Given AmountInput with `-50.00` / When validated / Then `field invalid` styling + error "Amount must be positive." (mirrors plan 03 AT10 field validation). | frontend | components/AmountInput.test.tsx |
| AT4 | Given LedgerTable with `loading`, then `[]`, then rows / When each prop set / Then it renders Skeleton+Spinner, then EmptyState ("No entries yet."), then the rows — **all three states present** (§04). | frontend | components/LedgerTable.test.tsx |
| AT5 | Given a 422 `unbalanced_entry` error prop / When passed to the form/Alert / Then the inline balance Alert renders (maps plan 03 AT2 `code`); given a 5xx error / Then a Retry Alert renders. | frontend | components/Alert.test.tsx |
| AT6 | Given StatementTable as a balance sheet (assets 96,850.18 / liab+equity 96,850.18) / When rendered / Then the balance-check BalanceBar shows Difference 0.00 + "✓ Balanced". | frontend | components/StatementTable.test.tsx |
| AT7 | Given AppNav / When `platform=web` / Then top `navrow` renders; When `platform=native` / Then bottom `tabbar` renders — identical destinations, adaptive chrome. | frontend | components/AppNav.test.tsx |
| AT8 | Given DebitCreditSegment defaulting 借方 / When the user taps 貸方 / Then `onChange('credit')` fires and the active segment recolors to `--credit`. | frontend | components/DebitCreditSegment.test.tsx |

## Section 2 — Integration-test spec   [from architecture / design-token + contract seams]

| ID  | Condition to verify | Domain | Test file |
|-----|---------------------|--------|-----------|
| IT1 | `theme/tokens.ts` values equal the design HTML `:root` custom properties (`--debit #7a1d1d`, `--credit #2a5a2a`, `--accent #8a5a1c`, `--bg #efece1`, …). A drift in any token fails the build. | frontend | theme/tokens.test.ts |
| IT2 | `lib/money.ts` `sum(['1000.0000','-500.0000'])` === `'500.0000'`; `format('1000.33335')` rounds to `'1000.33'` display under an explicit, documented policy; `0.12345` keeps 4dp value `'0.1235'`. **No float drift** (mirrors plan 03 AT11). | frontend | lib/money.test.ts |
| IT3 | BalanceBar derives `ok|bad` purely from `money.sum(debits) === money.sum(credits)` — given exactly-equal decimal strings it is `ok`; given a 0.0001 mismatch it is `bad`. Computation, not a passed-in boolean. | frontend | components/BalanceBar.test.tsx |
| IT4 | JournalEntryForm `onSubmit` payload matches the plan 03 POST `/command/journal-entries` shape (lines with `account` code, `amount` decimal string, `debit|credit`); the form NEVER calls a network API itself (presentational). | frontend | components/JournalEntryForm.test.tsx |
| IT5 | Error props keyed by plan 03 RFC 7807 `code` (`unbalanced_entry`, `validation_failed`, `unauthorized`) map to the correct Alert/inline rendering — assert on `code`, never localized text (locale-fragile). | frontend | components/Alert.test.tsx |
| IT6 | Every list/statement component renders distinct output for `state="loading" | "empty" | "error" | "ready"` — a snapshot/branch test fails if any state is missing (§04 sign-off rule, decision #7). | frontend | components/{LedgerTable,StatementTable}.test.tsx |
| IT7 | AmountInput keeps a 4dp internal value while displaying 2dp (design §03 note: "4dp under the hood; display rounds to 2dp"); `onChangeValue` emits the 4dp string. | frontend | components/AmountInput.test.tsx |

## Section 3 — Implementation steps (one commit each; unit test per step)

One step = one commit = one PR-reviewable change. Each compiles & passes on its own.
TDD visible: the failing-test commit precedes the component for every money/balance path
(S6 money, S9 BalanceBar, S11 form). Every commit carries `Plan: S<n>`.

| ID  | Commit message (verbatim) | Files | Depends | Unit test |
|-----|---------------------------|-------|---------|-----------|
| S0  | `docs(plan): frontend plan 04 — ledger design-system component library` | `plans/04-design-component-implementation.md` (+ worktree `feat/04-design-component-implementation`) | — | review only |
| S1  | `chore(deps): bundle Source Serif 4 + JetBrains Mono + react-native-svg` | `package.json`, `package-lock.json` | — | `npm test` boots; fonts importable |
| S2  | `feat(theme): Forest Reserve tokens + react-native-paper theme` | `theme/tokens.ts`, `theme/paperTheme.ts` | S1 | IT1 (token-drift test) |
| S3  | `test(theme): failing token-drift assertion vs design HTML` | `theme/tokens.test.ts` | — (RED before S2 lands; ordering note below) | IT1 RED→GREEN |
| S4  | `feat(ui): Button + Badge from design controls (§02)` | `components/Button.tsx`, `components/Badge.tsx`, `*.test.tsx` | S2 | Button variants + disabled; Badge 4 states |
| S5  | `feat(ui): Field + AccountSelect + DebitCreditSegment (§03)` | `components/Field.tsx`, `AccountSelect.tsx`, `DebitCreditSegment.tsx`, `*.test.tsx` | S2 | AT8 (segment toggle); Field invalid state |
| S6  | `test(lib): failing decimal money format + sum (incl. boundaries)` | `lib/money.test.ts` | — | IT2 RED |
| S7  | `feat(lib): decimal-exact money helper (2dp display / 4dp value)` | `lib/money.ts` | S6 | IT2 GREEN |
| S8  | `feat(ui): AmountInput — tabular, right-aligned, 4dp value (§03)` | `components/AmountInput.tsx`, `AmountInput.test.tsx` | S7 | AT3, IT7 |
| S9  | `test(ui): failing BalanceBar 借方=貸方 incl. unbalanced case` | `components/BalanceBar.test.tsx` | S7 | IT3 RED (ok + bad) |
| S10 | `feat(ui): BalanceBar — derives ok|bad from money.sum (§05)` | `components/BalanceBar.tsx` | S9 | IT3 GREEN |
| S11 | `feat(ui): state primitives — Skeleton, Spinner, EmptyState, Alert (§04)` | `components/{Skeleton,Spinner,EmptyState,Alert}.tsx`, `*.test.tsx` | S2 | AT5, IT5 |
| S12 | `feat(ui): LedgerTable — general/subsidiary + loading/empty/error (§06)` | `components/LedgerTable.tsx`, `LedgerTable.test.tsx` | S4,S11 | AT4, IT6 |
| S13 | `test(ui): failing JournalEntryForm — CTA gated on balance` | `components/JournalEntryForm.test.tsx` | S10 | AT1, AT2 RED |
| S14 | `feat(ui): JournalEntryForm — sentence capture + lines + review (§05)` | `components/JournalEntryForm.tsx` | S13 | AT1, AT2, IT4 GREEN |
| S15 | `feat(ui): StatementTable — balance sheet + P&L + check bar (§07)` | `components/StatementTable.tsx`, `StatementTable.test.tsx` | S10 | AT6, IT6 |
| S16 | `feat(ui): AppNav — adaptive bottom tabs (native) / top nav (web) (§08)` | `components/AppNav.tsx`, `AppNav.test.tsx` | S4 | AT7 |
| S17 | `chore(ui): barrel export + design-parity snapshot pass` | `components/index.ts`, snapshot updates | S4–S16 | full `npm test` green |

### Step notes
- **S2/S3 ordering.** S3 (the RED token-drift test) is authored first in the working
  tree but `theme/tokens.ts` must exist to import; land the failing test referencing the
  expected literal values, then S2 makes it green. Keep the commit pair adjacent so the
  RED→GREEN is visible in history (`tdd.md`).
- **S7 money.** `lib/money.ts` operates on decimal **strings** (or a thin wrapper over a
  decimal lib if added — keep deps minimal; a hand-rolled fixed-point on integer cents at
  4dp is acceptable and dependency-free). `format(value, {display:2, store:4})`,
  `sum(strings)`, `isBalanced(debits, credits)`. NEVER `parseFloat` into arithmetic.
  Document the rounding policy (half-up, matching backend NUMERIC(20,4)) in a header
  comment so AT11/IT2 are unambiguous.
- **S10 BalanceBar.** `ok|bad` is COMPUTED from `money.isBalanced`, not a prop — so the
  component can't be told it's balanced when it isn't (IT3). Renders the three cells
  (借方/貸方/Difference) + the pill + the gated CTA slot.
- **S11 states.** Loading = Skeleton rows + Spinner; Empty = the CoA-grid SVG mark +
  "No entries yet." + a "+ New entry" accent button slot; Error = Alert (`error` variant)
  with an inline 借方≠貸方 body OR a 5xx Retry body. These are the shared primitives §04
  mandates on every async surface.
- **S12/S15 three-state rule.** LedgerTable + StatementTable take a `state` discriminator
  and MUST render loading/empty/error/ready distinctly (IT6). A component missing a state
  fails the test and review (decision #7).
- **S14 form.** Presentational: `onSubmit(payload)` only — no axios. Payload shape mirrors
  plan 03 POST `/command/journal-entries`. The CTA reads `BalanceBar`'s computed balance;
  disabled when unbalanced (AT2). Step 2 (review table) shows prev/this/new balances.
- **S16 AppNav.** Branch on `Platform.OS === 'web'` (RN Web) → top `navrow`; else bottom
  `tabbar` (RN Paper). Same destination list; identical logic, adaptive chrome (§08).

---

## Failure modes

| Codepath | Realistic failure | Test? | Error handling | User visibility |
|----------|-------------------|-------|----------------|-----------------|
| money.sum on user input | float reintroduced via `Number()` somewhere | IT2 boundary cases (1000.33335, 0.12345) | string/fixed-point only; lint rule bans `parseFloat` on money | wrong Difference in BalanceBar → caught by IT3 |
| BalanceBar balance | component told "balanced" by a stale prop | IT3 (derives, not props) | compute from money.isBalanced | CTA would wrongly enable → AT2 blocks it |
| token drift | designer edits HTML, tokens.ts lags | IT1 hard-fails build | single source asserted in test | visual mismatch caught in CI, not by eye |
| missing async state | a list ships without Empty/Error | IT6 per-component | `state` discriminator is required | blank screen → test fails pre-merge |
| font not bundled | `@expo-google-fonts` import missing | S1 boot test | bundled, not CDN | fallback serif/mono; degraded but functional |
| RN Web vs native nav | nav renders both/neither | AT7 platform branch | `Platform.OS` switch | wrong chrome on one platform → AT7 fails |

**No silent failures.** Every money/balance/state path has a test and an observable symptom.

---

## Worktree parallelization strategy

| Step | Module touched | Depends on |
|------|----------------|------------|
| S0 | plans/ + worktree setup | — |
| S1 | package.json | — |
| S2/S3 | theme/ | S1 |
| S4 | components/ (Button, Badge) | S2 |
| S5 | components/ (Field, AccountSelect, Segment) | S2 |
| S6/S7 | lib/money | — |
| S8 | components/AmountInput | S7 |
| S9/S10 | components/BalanceBar | S7 |
| S11 | components/ (state primitives) | S2 |
| S12 | components/LedgerTable | S4, S11 |
| S13/S14 | components/JournalEntryForm | S10 |
| S15 | components/StatementTable | S10 |
| S16 | components/AppNav | S4 |
| S17 | components/index.ts | all |

**Lanes (after S1+S2 land the deps+theme foundation):**
- Lane A: S4 (Button/Badge) → S16 (AppNav)
- Lane B: S5 (Field/Select/Segment) — independent
- Lane C: S6→S7→S8 (money → AmountInput)
- Lane D: S6→S7→S9→S10 (money → BalanceBar) → S13→S14 (form) → S15 (statements)
- Lane E: S11 (state primitives) → S12 (LedgerTable)

S1 + S2/S3 are the sequential bottleneck (all lanes import the theme). After that,
Lanes A/B/C/E run parallel; Lane D is the longest chain (money → balance → form →
statements). S17 merges last. All lanes share `components/` but each owns distinct
files; only S17's `index.ts` is a shared-file touch and lands last.

---

## Hand-off to the heads

- **frontend-engineer (HEAD):** owns S0–S17. Writes the AT1–AT8 + IT1–IT7 specs as
  FAILING render/interaction tests FIRST (RED), then greens via the `tdd-frontend` /
  `tdd-implementer` pattern → `code-reviewer`, dispatching a ruflo sonnet swarm. Works in
  the `feat/04-design-component-implementation` worktree. The design HTML is the visual
  oracle; `theme/tokens.test.ts` is the anti-drift gate.
- **backend-engineer (HEAD):** NOT involved — components are presentational. The API
  contract (plan 03) is consumed as a shape reference only; live wiring is plan 05.
- **infra-engineer (HEAD):** NOT involved in plan 04.
- **plan-tracker:** logs each landed step to `04-design-component-implementation.impl.md`.

**"Done" (plan 04)** = AT1–AT8 + IT1–IT7 green; `npm test` (jest-expo) clean;
`expo lint` clean; token-drift test (IT1) green against the design HTML; every list /
statement component proven to render all four states (IT6); money paths proven
float-free with boundary cases (IT2). Design-review sign-off on the rendered components
vs `design/02-frontend-ledger.design-system.html`.

## NOT in scope (plan 04)

- expo-router screens + navigation wiring (`app/**`) — **plan 05**.
- Data hooks, axios client, owner-scoped fetching, cursor pagination UI — **plan 05**.
- Keycloak OIDC login flow + token storage (AsyncStorage) — **plan 05**.
- OTel browser/RN spans on user actions — **plan 05** (devops "done" for screens).
- Real CoA data — components take a CoA list as props; the seed is plan 03's `config/coa.yaml`.
- i18n display-name lookup (`locales/<lang>.json`) wiring — components accept already-resolved
  labels; the locale plumbing is plan 05.
- Storybook/Chromatic visual regression infra — optional dev harness, not a deliverable here.

## What already exists

- Plan 00 landed the RN/Expo scaffold: `package.json` (expo 54, expo-router 6,
  react-native-paper 5, react-hook-form 7, axios, AsyncStorage), `jest.config.js`
  (jest-expo preset, RN-svg + paper whitelisted in transformIgnorePatterns),
  `app/hooks/.gitkeep`, `components/.gitkeep`. No components yet — `components/` is
  greenfield.
- `design/02-frontend-ledger.design-system.html` (the gating design system, status:
  draft) + `design/reckonna.design-system.html` (brand tokens) are committed on `develop`.
- Plan 03 (backend) defines the API nouns (`journal-entries`, `journal-lines`,
  `accounts`), the RFC 7807 error-code registry, and decimal money — the contract these
  components are shaped against (read-only reference; no live calls here).

---

## GSTACK REVIEW REPORT

| Review | Trigger | Why | Runs | Status | Findings |
|--------|---------|-----|------|--------|----------|
| Design Review | `/plan-design-review` | UI/UX fidelity vs design system (required, frontend) | 0 | PENDING | — |
| Eng Review | `/plan-eng-review` | Architecture & tests (required) | 0 | PENDING | — |
| CEO Review | `/plan-ceo-review` | Scope & strategy | 0 | — | — |
| Codex Review | `/codex review` | Independent 2nd opinion | 0 | — | — |
| DX Review | `/plan-devex-review` | Developer experience | 0 | — | — |

**UNRESOLVED:** 0
**VERDICT:** DRAFT — awaiting `/plan-design-review` (fidelity vs design HTML) +
`/plan-eng-review` (token-drift gate, money-float gate, three-state rule) before a human
flips `status: draft → approved`. The design system itself (`design/02-frontend-ledger.design-system.html`)
must also carry its own `approved` marker before S0 commits (design gate).
