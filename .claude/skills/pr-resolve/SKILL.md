---
name: pr-resolve
description: >-
  Resolve an open PR by validating its acceptance criteria (A1, A2, … / AT ledger)
  ONE at a time, human-gated, completing the V-model loop: read the implementation plan,
  derive each acceptance test as a COMMITTED, retestable test file with detailed
  assertions, run it, then mark it. For each criterion: ship the change (ship-at), QA it
  (tests + lint + the authoritative external gate), validate its acceptance test, STOP.
  Never batches, never auto-merges. Use when asked to "resolve the PR", "validate
  acceptance criteria", "check and merge the PRs", "ship-at then qa", or when working a
  PR-resolve checklist phase by phase (Phase A → B → merge).
---

# /pr-resolve — one acceptance criterion at a time: ship → qa → validate

Wraps `/ship-at` + `/qa` into a gated per-criterion loop that resolves a PR the way a
careful reviewer does: prove each criterion green before touching the next, and hand the
human every decision that is theirs (security review, external checks, merges, releases).

Sits on top of the V-model AT ledger (`~/.claude/v-model-workflow.md`), but also works
against any explicit acceptance-criteria checklist in the plan file (`A1…`, `B1…`, `AT<k>…`).

## Preconditions

- A plan file with an acceptance-criteria checklist — each item stating **what to check +
  how (exact command/API) + expected result**. Example:
  `- [ ] A1 — FE Sonar gate green → local sonar-scanner → new_violations 0, new_coverage ≥80`.
- One criterion in flight at a time. If any item is `[~]`, STOP — the human accepts it first.

## The loop (one criterion per invocation)

For the first `[ ]` criterion (or the one named in the args):

### 1. Ship (if it needs a code change)
Some criteria are *verification only* (code already shipped); others need a fix. If a fix is
needed, implement ONLY this criterion's change, one commit, push to the open PR branch. Do
NOT run the full `/ship` pipeline. Mirror `/ship-at`.

### 2. QA it
Run the criterion's QA gate — the check that actually *proves* it, not a proxy:
- **Tests / lint / typecheck** local (`jest`/`tsc`/`eslint`, `go test -race`/`golangci-lint`).
- **External quality gate**, run AUTHORITATIVELY — don't trust a possibly-stale CI job. (In
  Reckonna the CI `SonarQube` job can report success without updating the server: run
  `sonar-scanner` locally with coverage fed — `jest --coverage` / `go test -coverpkg
  -coverprofile` — and read `api/qualitygates/project_status`. CE has one project, so each
  scan overwrites the last — re-scan the branch you are checking.)
- **Pre-merge code review** — dispatch a code-reviewer on the PR diff → require MERGE. A BLOCK
  finding is a real fix + a new committed regression test (§ below), not a nit.

### 3. Validate the acceptance test  →  see the V-model section
Derive/create the AT as a committed test file, run it, confirm the detailed expectation.
- **Pass** → mark `[x]` (or `[~]` for the human to accept), report the evidence.
- **Fail** → fix and re-ship the SAME criterion (do not advance).

### 4. STOP and hand off
Print the acceptance test + result. Update the tracked todo. STOP — one criterion per run.
Tell the user what is next and what is `[ ]`.

## V-model: derive & validate the acceptance test (committed, retestable)

Every acceptance criterion resolves through a real, committed test — never an ad-hoc,
throwaway check. This closes the V (Section 3 implementation ↔ Section 1 acceptance) and
leaves the repo able to re-prove the criterion on any future run.

**a. Read the plan (the left side of the V).** Parse three things for the AT in flight:
- **Section 3** — the implementation steps/files that were supposed to satisfy it.
- **Section 1** — the AT's `Given / When / Then` + expected.
- **Section 2** — the integration contract it must honor (DTO shape, CQRS write→read,
  domain invariant e.g. 借方=貸方).

**b. Trace implementation ↔ plan (the V gate).** Confirm the code implementing this AT
exists and follow it end to end. No AT validates until its mirrored implementation is
present and its test layer is green. If impl is missing → it is NOT ready to ship; if impl
exists with no AT → that is a gap, create the AT (step d).

**c. Write the AT as a COMMITTED, ORGANIZED, RETESTABLE test file.** Turn the prose AT into
a runnable test placed where the repo's runner + CI already pick it up, so it re-runs
forever:
- **Location by convention** (match the repo, don't invent a parallel structure):
  - Reckonna backend ATs → `e2e/AT<k>_<slug>.e2e_test.go` (testcontainers) or the package's
    `*_test.go`; integration → `internal/**/<x>_it_test.go`.
  - Reckonna frontend ATs → `components/<Component>.test.tsx` / `lib/<x>.test.ts` (jest).
  - Homelab / infra ATs → `tests/AT<k>_<slug>.sh` (the repo's `bash tests/*.sh` grep/IT
    convention) + `make k8s-validate` / `tf validate` gates.
- **Traceable header** — every AT test file/block carries a comment mapping it back:
  `// AT<k>: <requirement> — plan <feature> §1  (report: <path>)` so the plan ↔ test link
  survives.
- **Named by AT** — `AT<k>` / `IT<k>` / the criterion id in the test name, so a reviewer can
  grep the criterion to its proof.
- **Deterministic + isolated** — no reliance on one-shot state or scan order; re-runs green
  offline where possible (mock external services; per-test DB schema / testcontainers for IT).

**d. Detailed assertions + the mandatory invalid case.** Assert specific observable values,
never "it works":
- Given = exact input/state; When = the precise action; Then = the asserted value(s)
  (`toValue('4200.') === '4200.0000'` AND `not.toThrow()`; or `gate STATUS OK,
  new_violations 0`).
- **Every money/domain AT includes a failing case** (V-model + repo TDD rule): 借方≠貸方 →
  error; bare `.`/`-` → throw; unbalanced entry → 422. A money/domain AT without an invalid
  case is incomplete.

**e. Gap-discovery — the loop self-corrects the plan.** QA/review often shows the plan's ATs
are incomplete (they exercised only the happy path). When found, CREATE the missing AT as a
committed regression test, tagged `REGRESSION: <what broke>` with a `Found by /pr-resolve on
<date>` header, validate it, and only then let the criterion pass. Example: a component that
crashed on a per-keystroke intermediate state the existing AT never fired.

**f. Validate → gate → STOP.** Run the committed AT (its own file, plus the full suite to
prove no regression), confirm the detailed expectation, mark the criterion, one per run.

## Human-gated items — surface, never silently skip or fake

- **Security-hotspot review** ("Safe") when the token lacks perms — OR resolve by *fixing the
  flagged code* so the hotspot disappears (preferred: e.g. rewrite a ReDoS-prone regex to a
  non-backtracking form).
- **External/ghost CI checks** (a dead SonarCloud project) — repo-admin removal.
- **Infra credential failures** (an expired PAT) — the human rotates it.
- **Merges & releases** (`gh pr merge`, `develop→main`) — human owns acceptance + merge.

## Mandatory closing output — SonarQube gate must be CLEAN (always)

Before marking ANY criterion accepted or reporting it done, ALWAYS run a fresh, authoritative
SonarQube scan of the branch and confirm the quality gate has ZERO open issues. "Green tests"
is NOT "gate clean" — this is non-negotiable output on every run.

1. **Feed coverage, then scan** (the CI scan may be stale — do it locally):
   - frontend: `npx jest --coverage` → `coverage/lcov.info`
   - backend: `go test -coverpkg=./... -coverprofile=coverage.out ./...`
   - `sonar-scanner -Dsonar.host.url=<host>` (`SONAR_TOKEN` from Vault, env only). Community
     Edition has one project — each scan overwrites the last, so scan the branch you are on.
2. **Read the gate and REQUIRE every condition OK**:
   `GET /api/qualitygates/project_status?projectKey=<key>` → `STATUS: OK` with
   `new_violations 0`, `new_coverage ≥ threshold`, `new_duplicated_lines_density ≤ threshold`,
   `new_security_hotspots_reviewed 100%`; AND `api/issues/search?...&statuses=OPEN` total = 0.
3. **If any condition is ERROR → the criterion is NOT done.** Resolve it (fix code / add a
   committed test / fix or review the hotspot) and RE-SCAN. Never mark a criterion accepted,
   and never report "done", while the gate is ERROR.
4. **Report the gate block verbatim** (STATUS + each condition + open-issue count) as the
   criterion's closing evidence.


## Rules

- Exactly ONE criterion per run. Never two (unless the user explicitly names several).
- Never advance while an item is `[~]`. Never force-push, merge, or release.
- Every accepted criterion leaves a COMMITTED test behind — retestable, in the CI path.
- Prefer fixing code over marking-as-safe when it removes a finding at the source.
- Re-run the authoritative gate AFTER every fix — prove the finding is gone, don't assume.
- Track progress in the todo list (TaskCreate/TaskUpdate): one todo per criterion.
- Report plainly: which criterion, the evidence, PR URL, what's still `[ ]`.

## Typical shape (as run to resolve a PR)

```
Phase A (frontend PR):  A1 gate → A2 hotspots → A3 local → A4 CI → A5 review
Phase B (backend PR):   B1 violation → B2 coverage → B3 gate → B4 correctness → B5 CI+review
Phase C (merge):        C0 prereqs → C1 rebase → C2 merge (no squash) → C3 combined scan → …
```
Each cell = one `/pr-resolve` (or `/ship-at`) run: ship → qa → derive+commit AT → validate → STOP.
