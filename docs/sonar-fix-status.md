# /sonar-fix — status ledger

Durable record of `/sonar-fix` runs. The self-hosted SonarQube CE gate is blind to shell
(only terraform+yaml/go/js sensors; `scripts/`,`tests/`,`.claude/hooks/` aren't in
`sonar.sources` and there is no Shell plugin), so **ShellCheck is the authoritative shell
engine** here — findings SonarCloud's automatic analysis shows on `.sh` are invisible to the
self-hosted gate otherwise.

| file | issues before | issues after | engine | fix | date |
|---|---|---|---|---|---|
| `.devcontainer/versions.sh` | 13 (SC2148, SC2034×12) | 0 | shellcheck | `# shellcheck shell=bash` + file-level SC2034 disable (sourced manifest; vars consumed externally) | 2026-07-08 |
| `scripts/test-quality-gate.sh` | 9 (SC2317) | 0 | shellcheck | 2 function-level SC2317 disables (`delete_project`/`cleanup` invoked via EXIT trap) | 2026-07-08 |
| `tests/pg-endpoint_test.sh` | 2 (SC2015) | 0 | shellcheck | `A && {fail} \|\| true` → `if A; then fail; fi` (assertion preserved) | 2026-07-08 |
| `.claude/hooks/require-prereq.sh` | 1 (SC2010) | 0 | shellcheck | `ls\|grep` → glob `for` loop | 2026-07-08 |
| `scripts/deploy.sh` | 1 (SC2015) | 0 | shellcheck | `&&\|\|` → explicit `if/else` | 2026-07-08 |
| `scripts/pg-probe.sh` | 1 (SC2317) | 0 | shellcheck | removed genuinely-dead trailing `shift` (all case arms exit) | 2026-07-08 |
| `tests/pg-probe_test.sh` | 1 (SC2015) | 0 | shellcheck | cleanup trap → explicit `if` | 2026-07-08 |
| `tests/tunnel-config_test.sh` | 1 (SC2015) | 0 | shellcheck | order-assert → `if ! {A && B && C}; then fail` (correctness fix) | 2026-07-08 |

**Run 2026-07-08:** 29 shellcheck findings across 8 files → **0**. Fixed concurrently, one agent
per file. `bash -n` clean on all 8; `tunnel-config_test.sh` green. Sonar (terraform+yaml) gate
was already `STATUS: OK`. Note: SonarCloud showed ~23 `.sh` issues (subset of these 29, minus
codes it dedups) — driving shellcheck→0 clears them.
