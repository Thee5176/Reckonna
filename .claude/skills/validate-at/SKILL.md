---
name: validate-at
description: >-
  The validation LOOP — validate the acceptance-criteria todos (from /qa-plan) ONE at a time,
  like /ship-at but proving instead of shipping. For the next pending validation todo: QA it,
  derive the AT as a COMMITTED retestable test with detailed assertions + a mandatory invalid
  case, run it, close on a CLEAN SonarQube gate, mark the todo, STOP. Human-gated, one per run.
  Use when asked to "validate-at", "validate the next AT", "run the validation loop", or after
  /qa-plan has listed the criteria.
---

# /validate-at — one acceptance criterion per run: qa → derive committed test → validate → clean gate → STOP

The loop over the validation todo ledger produced by `/qa-plan`. Mirrors `/ship-at` exactly —
one criterion per invocation, human-gated, STOP after each — but it PROVES the criterion
(right side of the V) rather than shipping code.

## Steps (mirror /ship-at)

1. **Read the ledger** — `TaskList` (the criteria `/qa-plan` created). Guard: any `in_progress`
   (unaccepted) → STOP, the human accepts it first. Take the first `pending` (or the AT named
   in the args); mark it `in_progress`.
2. **Out-of-order guard** — if the user names an AT but earlier ones are pending, warn + confirm.
3. **QA the criterion** — local `jest`/`tsc`/`eslint` or `go test -race`/`golangci-lint`/`make test`;
   dispatch a code-reviewer when the criterion is a whole feature → require MERGE (a BLOCK is a
   real fix + a new regression AT, not a nit).
4. **Derive & run the acceptance test (V-model, committed + retestable):**
   - Read the plan: Section 3 impl ↔ Section 1 AT ↔ Section 2 contract; trace the implementation
     (the V gate — no AT validates without its impl present + green).
   - Write the AT as a COMMITTED test where the repo runner + CI pick it up (backend
     `e2e/AT<k>_<slug>.e2e_test.go` / `internal/**/*_it_test.go`; frontend
     `components/<X>.test.tsx` / `lib/<x>.test.ts`; infra `tests/AT<k>_<slug>.sh`). Header-comment
     it to the plan (`// AT<k>: <req> — plan <feature> §1`), name it by the AT, keep it
     deterministic + isolated.
   - Detailed value assertions + a MANDATORY invalid case for every money/domain AT (借方≠貸方 →
     error). Gap-discovery: a missed path becomes a committed `REGRESSION:` AT, then validate.
5. **Mandatory closing gate — always run SonarQube, require CLEAN.** Feed coverage
   (`jest --coverage` / `go test -coverpkg -coverprofile`), scan (`sonar-scanner`, SONAR credential
   from Vault, env only; CE overwrites the single project — scan the branch you are on), read
   `api/qualitygates/project_status`: require `STATUS: OK` (`new_violations 0`,
   `new_coverage ≥ threshold`, `new_duplicated_lines_density ≤ threshold`,
   `new_security_hotspots_reviewed 100%`) AND open-issues total 0. ERROR → the AT is NOT
   validated: fix + re-scan.
6. **Mark + STOP** — pass → `TaskUpdate` the todo `completed`, report the acceptance test + result
   verbatim. Fail → keep it open, fix, re-validate the SAME criterion. One per run. Say what's next.

## Always: append the run status to the validation log (document)

Every /validate-at run — PASS, FAIL, or BLOCKED(human) — appends/updates a status row in
`docs/validation-status.md` (create it if missing). This is mandatory output, like the Sonar
gate: the document is the durable source of truth for "what is proven", not just chat + todos.

Row: `| AT | criterion | result | gate STATUS | key conditions | evidence | date |`
- **result** = PASS / FAIL / BLOCKED(human)
- **gate STATUS** = the SonarQube quality-gate status + the deciding condition(s)
- **evidence** = commit SHA and/or the committed test path
- **date** = the run date
Update the SAME row on re-validation (never duplicate). Do this before the final STOP.


## Human-gated — surface, never fake
Security-hotspot review (or fix the code so it disappears), external/ghost CI checks (repo-admin),
expired credentials (rotate), merges & releases (human owns). Flag + stop.

## Rules
- Exactly ONE criterion per run (unless the user names several). Never advance past an
  `in_progress`/accepted item. Never force-push, merge, or release.
- Every validated criterion leaves a COMMITTED test behind — retestable, in the CI path.
- Prefer fixing code over marking-safe. Re-run the gate after every fix — prove it, don't assume.
