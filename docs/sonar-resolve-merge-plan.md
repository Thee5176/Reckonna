# Plan — Resolve Frontend → Backend → Merge into develop (authoritative Sonar)

## Context

Plans 03 (backend CQRS) and 04 (frontend component library + Storybook + 100% coverage) are
built, green, and pushed on `feat/03-backend-cqrs-core` (PR #16) and
`feat/04-design-component-implementation` (PR #17). The 26 frontend SonarTS smells were
already fixed (`dfdf1cc`, verified in code). BUT the self-hosted SonarQube gate is
**stale + misleading**:

1. **CI's `SonarQube` job reports "success" without updating the server analysis** — the
   live gate is frozen at `bac67a4` (07-02 00:15): still shows the fixed 26 + an `ncloc`
   that predates the fix and the new `app/` screens. So the ERROR gate is a stale reading.
2. **`new_coverage: 0%`** on both branches — CI never feeds coverage reports to the scanner
   (the `sonar` job runs only checkout + scan, no `go test -coverprofile` / `jest --coverage`).
3. **Community Edition, one project, no branch analysis** — every branch scan clobbers the
   single `reckonna` analysis, so there is never a combined BE+FE gate.

Goal: get an **authoritative** gate by running `sonar-scanner` **locally** (bypasses the
broken CI job, and finally feeds coverage), resolve any real findings on **frontend then
backend**, then **merge both into `develop`** and scan develop for the true combined gate.

Prereqs verified: `sonar-scanner` installed at `~/.local/sonar-scanner` (Java 25); the scan
credential lives in Vault at `secret/homelab/sonar/ci-token` (Execute-Analysis scope);
`git merge-tree` shows `develop`↔feat/03 and `develop`↔feat/04 both merge **clean** (no
conflicts).

**Credential handling (secrets-vault.md):** export the scan credential into the `SONAR_TOKEN`
environment variable from Vault at run time — `sonar-scanner` reads `SONAR_TOKEN` from the
environment automatically, so it is NEVER written on the command line or into any file.

---

## Phase A — Resolve FRONTEND (feat/04), authoritative local scan

Worktree `.claude/worktrees/frontend-04` (branch feat/04, hook fixed, Edit/Write works).

1. **Generate JS coverage** (the missing input): `npx jest --coverage`. Confirm it emits
   `coverage/lcov.info` (jest.config.js already has `collectCoverageFrom`; if `coverageReporters`
   lacks `lcov`, add `coverageReporters: ['lcov','text-summary']`). Coverage is 100% (114 tests).
2. **Local scan** from the worktree: export the Vault credential into `SONAR_TOKEN`, then run
   `sonar-scanner -Dsonar.host.url=https://sonar.thee5176.com` (it auto-reads `SONAR_TOKEN` +
   the worktree's `sonar-project.properties` + `coverage/lcov.info`).
3. **Read the fresh gate** (`api/qualitygates/project_status?projectKey=reckonna`, basic auth
   with the Vault credential). Expect: `new_violations` 26→0 (dfdf1cc fixed them),
   `new_coverage` 0→~100% (lcov now fed), `new_duplicated_lines_density` 0%.
4. **Resolve residual only if real:** the new `app/` screens (`b34c797`) weren't in the stale
   analysis — a fresh scan may surface a few new smells there; fix by rule (same mechanical
   patterns as dfdf1cc). Review any `new_security_hotspots_reviewed` items — mark safe/fixed
   via the hotspots API or fix. Re-scan until the FE gate is clean. Keep `tsc`/`eslint`/`jest`
   green; commit (`refactor(ui): …`) + push feat/04.

## Phase B — Resolve BACKEND (feat/03), authoritative local scan

Worktree `.claude/worktrees/backend-03` (branch feat/03). Its `sonar-project.properties`
carries the sqlc-exclusion fix (`a43e187`).

1. **Generate Go coverage:** `go test -coverprofile=coverage.out ./...`. testcontainers e2e
   need the rootless-podman socket (per repo memory): `DOCKER_HOST=unix:///run/user/1000/podman/podman.sock`
   `TESTCONTAINERS_RYUK_DISABLED=true`. If containers are unavailable, cover the non-e2e
   packages (`go test -coverprofile=coverage.out $(go list ./... | grep -v /e2e)`) and note it.
2. **Local scan** from backend-03 (same env-token + `sonar-scanner` invocation as A; it picks
   up `coverage.out`).
3. **Read the fresh gate.** Expect: `new_violations` 0, `new_duplicated_lines_density` 0%
   (dedup landed), `new_coverage` 0→real (coverage.out now fed). The 2 openapi mismatches
   (limit>200 not 400; Idempotency-Key format not enforced) are pre-flagged non-blockers —
   fix or ticket per taste, not gate-blocking.
4. Resolve any residual real findings; re-scan until BE gate clean. Keep `make test`/`make lint`
   green; commit + push feat/03.

## Phase C — Merge into develop + combined scan + CI cleanup

1. **Merge both into `develop`** (merge-tree clean): `feat/04 → develop`, then `feat/03 → develop`
   (order irrelevant, no conflicts). Via git in the `branch-protection` worktree (which holds
   develop) or via the PRs. Both PRs are DRAFT → mark ready first if merging via GitHub.
2. **Authoritative COMBINED scan on develop:** in a develop checkout, generate BOTH coverages
   (`go test -coverprofile` + `npx jest --coverage`) then `sonar-scanner`. This is the real
   BE+FE gate the whole exercise is for — one tree, both languages, both coverages.
3. **CI check cleanup (blocks PR-based merge; skip if merging locally):**
   - **SonarCloud ghost check** — the `SonarCloud Code Analysis` GitHub App points at the dead
     `Thee5176_Reckonnna` project (fails on #16). Remove the SonarCloud app/check from the repo
     (repo Settings → GitHub Apps, or branch-protection required-checks) — human/repo-admin action.
   - **Terraform validate — FAIL** — investigate (likely the dead terraform GitHub PAT from the
     earlier 401, or a real tf issue); fix or de-require for these non-infra PRs.
   - Note: CI's own `Sonar gate` check already passes (it only verifies the scan ran), so the
     self-hosted quality gate doesn't hard-block; the two above do.
4. **Push develop** (the hook fix `69626e4` is already on develop). PRs auto-close as merged.

---

## Critical files / commands

- Scanner: `~/.local/sonar-scanner/bin/sonar-scanner`; per-worktree `sonar-project.properties`
  (`sonar.go.coverage.reportPaths=coverage.out`, `sonar.javascript.lcov.reportPaths=coverage/lcov.info`,
  `sonar.sources=cmd,internal,app,components,infra`). Host `https://sonar.thee5176.com`;
  scan credential from Vault path `secret/homelab/sonar/ci-token`, exported to the
  `SONAR_TOKEN` env var only (never a file / never on the command line).
- FE coverage: `frontend-04` `npx jest --coverage` → `coverage/lcov.info` (jest.config.js `collectCoverageFrom`).
- BE coverage: `backend-03` `go test -coverprofile=coverage.out ./...` (+ podman `DOCKER_HOST`/RYUK).
- Merge target: `develop` (branch-protection worktree). PRs: #16 (feat/03), #17 (feat/04) → develop.
- Gate read (read-only): `GET https://sonar.thee5176.com/api/qualitygates/project_status?projectKey=reckonna`
  (basic auth, Vault credential as the username).

## Verification

- **Phase A done:** local scan of feat/04 → gate `new_violations 0`, `new_coverage ≥80%`,
  `new_duplicated_lines_density ≤3%`, hotspots reviewed → **OK**. tsc/eslint/jest green.
- **Phase B done:** local scan of feat/03 → same conditions OK; `make test`/`make lint` green.
- **Phase C done:** develop merged (no conflicts), combined local scan → gate **OK** for both
  languages together; develop pushed; both PRs merged/closed; SonarCloud ghost + terraform
  checks resolved (removed/fixed) or explicitly de-required.

## Risks / notes

- The local scan **overwrites** the single CE analysis each run (A, then B, then develop) —
  expected; only the **develop** scan is the final authoritative combined gate. A/B scans are
  per-branch confirmations.
- Scan credential stays in the `SONAR_TOKEN` env var (from Vault), never written to a file —
  `secrets-vault.md`.
- `new code` period: metrics are on new code since the project baseline; the whole library is
  "new," so `new_coverage` ≈ overall coverage once lcov is fed.
- SonarCloud-ghost removal + terraform-validate are repo-admin / infra items; may need the user
  (GitHub App config, terraform PAT rotation from the earlier 401).
- Merging draft PRs / pushing develop are shared-branch actions — done only on user go.
