---
name: sonar-fix
description: >-
  Run the authoritative quality gate (self-hosted SonarQube scan + ShellCheck for the .sh
  blind spot the CE server cannot see) and, when it finds issues, dispatch ONE fixer subagent
  per file CONCURRENTLY to resolve them, re-scan until clean, append status to a document, STOP.
  Use when asked to "sonar-fix", "run the quality gate and fix", "fix sonar issues",
  "resolve sonarqube", or after a feature is done (the standing "always check sonar" rule).
---

# /sonar-fix — gate check → concurrent per-file agent fixers → clean → STOP

Detector + dispatcher in one. A Claude Code hook is a shell command and cannot spawn subagents,
so this is a SKILL: it runs the gate engines itself, then the lead fans out fixer `Agent()`s.
Self-hosted SonarQube (Community Edition) only has terraform+yaml/go/js sensors — it is BLIND to
shell (`scripts/`,`tests/`,`.claude/hooks/` aren't in `sonar.sources`, and there is no Shell
plugin). So the gate MUST run ShellCheck too, or `.sh` issues visible on SonarCloud stay unseen.

## Steps
1. **Load credential** — `SONAR_TOKEN` from Vault `secret/homelab/sonar/ci-token`, env only
   (`export SONAR_TOKEN=$(vault kv get -field=token secret/homelab/sonar/ci-token)`),
   `SONAR_HOST_URL=https://sonar.thee5176.com`. Never a file, never on the command line
   (secrets-vault.md). `projectKey` = read from `sonar-project.properties` (portable).
2. **Run BOTH engines:**
   - **Sonar** — feed coverage if it exists (`go test -coverpkg=./... -coverprofile=coverage.out ./...`
     with the podman socket for e2e; `npx jest --coverage`), then
     `~/.local/sonar-scanner/bin/sonar-scanner -Dsonar.projectBaseDir=<repo>`. Read
     `api/qualitygates/project_status?projectKey=<key>` and
     `api/issues/search?componentKeys=<key>&resolved=false&ps=500`.
   - **ShellCheck** — `shellcheck -f gcc $(git ls-files '*.sh')`. The ONLY local shell gate.
3. **Aggregate → group by FILE.** One brief per file: `{path, [(line, rule, message)]}`.
   Sonar issues carry their component path; shellcheck lines are `file:line:col: note: msg [SCxxxx]`.
4. **Clean?** Sonar `STATUS: OK` + open-issues 0 + shellcheck 0 findings → append the status doc,
   report verbatim, **STOP**. Nothing to fix.
5. **Fan out — one fixer `Agent()` per file, IN PARALLEL** (single message, multiple Agent calls;
   `name` each by file, `run_in_background`). Disjoint files ⇒ no write-conflict, no worktree needed.
   Each brief contains:
   - **Degraded-mode paragraph** (CLAUDE.md Agent-Comms): "If your coordination tools (SendMessage,
     TaskUpdate, hive-mind_*) are missing, do NOT abort. Read ONLY this file, fix ONLY the issues
     listed, change nothing else, return a diff summary."
   - The file's exact absolute path + its issue list (line/rule/message).
   - Rules: **real fix preferred** (add quotes, `local`, `find` over `ls|grep`, add shebang).
     `# shellcheck disable=SCxxxx  # <reason>` ONLY for a confirmed false positive — e.g. SC2317
     unreachable-after-`trap`/`return`, SC2034 vars consumed by a `source`/harness. Never weaken or
     delete a test to silence a rule (tdd.md). Preserve behaviour.
6. **Verify + re-scan.** Lead confirms each file changed (`git diff --stat`), then re-runs BOTH
   engines from step 2. Loop until clean, or surface a file that could not be auto-fixed (why).
   Run `bash tests/*.sh` to prove the harness still passes (CI omits these — run locally).
7. **Append status to `docs/sonar-fix-status.md`** (mandatory output, like /validate-at's ledger):
   `| file | issues before | issues after | engine | gate STATUS | date |`. Update the SAME row on
   re-run, never duplicate. Do this before the final STOP.
8. **STOP.**

## Human-gated — surface, never fake
Security-hotspot review (the ci-token has Execute-Analysis but NOT hotspot-review — mark Safe in the
UI or fix the code so it disappears), external/ghost CI checks (repo-admin), expired credentials
(rotate), merges & releases (human owns). Flag + stop.

## Rules
- Both engines every run — Sonar alone is blind to shell. A "clean" that skipped shellcheck is a lie.
- One agent per file, concurrent. Fixers touch ONLY their file. Re-scan after every round — prove it.
- Prefer fixing code over marking-safe; a `disable` needs a written reason. Never weaken a test.
- Never force-push, merge, or release. Leave the status document updated behind every run.
