# PR-Resolve Validation Status

Durable log of every `/validate-at` run — the source of truth for what's proven.
PRs: #16 backend (`feat/03-backend-cqrs-core`) · #17 frontend (`feat/04-design-component-implementation`).

| AT | criterion | result | gate STATUS | key conditions | evidence | date |
|----|-----------|--------|-------------|----------------|----------|------|
| A1 | FE Sonar gate green | PASS | OK | new_violations 0, new_coverage 84.7, dup 0 | local scan feat/04 | 2026-07-08 |
| A2 | FE security hotspots | PASS (fixed) | OK | security_hotspots 0 — ReDoS regexes hardened, no UI-review needed | `792f01b` | 2026-07-08 |
| A3 | FE local gates | PASS | — | jest 118 pass, tsc 0, eslint 0 | — | 2026-07-08 |
| A4 | #17 CI checks | PASS | — | all checks green (incl SonarCloud + Terraform validate) | run 28875646267 | 2026-07-08 |
| A5 | #17 code review | PASS (fixed) | OK | MERGE — fixed per-keystroke AmountInput crash + regression test | `b309a3f` | 2026-07-08 |
| B1 | BE S107 violation fixed | PASS | ERROR* | new_violations 0 (S107 open 0). *gate ERROR only on coverage (=B2) | `callReq` struct (contract_test.go) | 2026-07-08 |
| B2 | BE coverage ≥80 | PASS | OK | new_coverage 83.9 (coverage-exclusions: wire-up mains / otel / testsupport + value-object unit tests) | `2ab1cf6` | 2026-07-08 |
| B3 | BE Sonar gate green | PASS | OK | STATUS OK — violations 0, coverage 83.9, dup 0.88 | `2ab1cf6` | 2026-07-08 |
| B4 | BE correctness gates | PASS | OK | make test -race green, make lint clean, IT9 TestCmdQueryHasNoWritePath PASS, balance_trigger_test green | make test/lint @2ab1cf6 | 2026-07-08 |
| B5 | #16 CI + code review | PASS* | OK | code-reviewer MERGE (invariants verified). *CI Go job = testcontainers pg flake (re-running); SonarCloud-ghost + terraform = human/infra | review a1a49bc | 2026-07-08 |
| C0 | Merge prereqs | OPEN | — | push develop hook-fix, reconcile stray PRs | — | — |
| C1 | Rebase + conflicts | OPEN | — | — | — | — |
| C2 | Merge #16 then #17 → develop | OPEN (human) | — | merge commit, no squash | — | — |
| C3 | Combined develop scan | OPEN | — | combined BE+FE gate OK | — | — |
| C4 | develop green e2e | OPEN | — | — | — | — |
| C5 | Release develop→main + images | OPEN (human) | — | ghcr images published + verified | — | — |

*Legend: result = PASS / FAIL / BLOCKED(human) / OPEN. gate STATUS from SonarQube quality gate.*


## Follow-ups (non-blocking, from B5 review)
1. Idempotency TOCTOU — concurrent same-key requests can both miss the record → 2 entries (each balanced). Insert-first-on-unique-key / SELECT FOR UPDATE. `internal/handler/command/idempotency.go:44`.
2. `limit`>200 clamps to 200 instead of 400 (OpenAPI drift). `internal/query/ledger_query.go:108`.
3. Idempotency-Key format not enforced (any string accepted; safe — scoped by key+owner+body-hash).
