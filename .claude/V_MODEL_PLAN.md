# Claude-Driven V-Model Plan — Spring Boot → Go/Gin Rewrite

The V-model is applied INTO an agent swarm. Three HEAD agents (backend/frontend/infra, opus)
each own their domain's V-model and walk it by invoking the V-phase SKILLS in order
(domain-modeler → tdd-implementer/migration-mapper → code-reviewer), dispatching a ruflo
sonnet swarm. Nothing climbs the right side until the left-side artifact + its test layer is green.

```
Requirements ───────────────────────────► Acceptance test
  (CLAUDE.md domain rules)                 (human-in-the-loop merge gate)
   \                                       /
    System design ──────────────► System / e2e test
     (/plan + plan-writer)        (code-reviewer skill verdict)
      \                           /
       Module design ──► Integration test
        (domain-modeler skill:    (testcontainers-go, CQRS parity)
         schema→sqlc→domain)       /
         \                        /
          Implementation ──► Unit test
           (tdd-implementer/      (PostToolUse hook: gofmt+lint+go test)
            migration-mapper skill)
```

## Phase → owner map (head ▸ skill)
| V-phase | Side | Owner | Gate / verification |
|---|---|---|---|
| Requirements & invariants | Design | CLAUDE.md (balance rule, CQRS boundary) | Acceptance: human approves at PR |
| System / architecture | Design | /plan (plan-writer + superpowers) | code-reviewer skill MERGE/BLOCK |
| Module (schema→types) | Design | head ▸ domain-modeler skill | Integration test (testcontainers) |
| Implementation | Build | head ▸ tdd-implementer / migration-mapper skill | Unit test via verify-*.sh |
| Unit verification | Verify | verify-*.sh PostToolUse | blocks edit on lint/test fail (exit 2) |
| Invariant verification | Verify | check-ledger-invariant.sh (backend head Stop) | blocks finish if money path untested |
| Integration verification | Verify | head ▸ code-reviewer skill + testcontainers | CQRS write→read asserted |
| System/acceptance | Verify | human + GitHub Actions | green CI required to merge |

## Execution walkthrough (per feature / per ported endpoint)
1. DESIGN DOWN. /plan (2 steps) writes the approved spec + HTML design system. The relevant head
   then invokes the domain-modeler skill → migration (db/migration/), sqlc (db/query/), invariant
   domain types (internal/domain/), and a test-case spec naming the required edge cases.
2. IMPLEMENT. /tdd <unit> or /migrate-endpoint <java-path>; the head runs the tdd-implementer /
   migration-mapper skill (red→green→refactor), dispatching sonnet workers via ruflo when available.
   verify-go.sh formats/lints/tests on every edit and blocks regressions.
3. VERIFY UP. On head Stop, check-ledger-invariant.sh confirms every money path is tested. Then
   the code-reviewer skill runs git diff, checks CQRS boundaries + tx atomicity, returns MERGE/BLOCK.
4. ACCEPTANCE. Human reviews the PR; GitHub Actions runs go test -race + golangci-lint + integration.
   Merge only on green — the human-in-the-loop gate.

## Migration sequence (suggested submodule order)
| Step | Scope | Head ▸ skill |
|---|---|---|
| 0 | Scaffold Go monorepo, sqlc, migrate, otel config, CI (run with CLAUDE_BOOTSTRAP=1) | backend ▸ tdd-implementer |
| 1 | Port schema: Flyway V*.sql → golang-migrate; gen sqlc | backend ▸ domain-modeler |
| 2 | Command service (write side, tx) — springboot_cqrs_command | backend ▸ migration-mapper |
| 3 | Query service (read side, JOIN/flatten N+1) — springboot_cqrs_query | backend ▸ migration-mapper |
| 4 | OTel instrumentation (traces/metrics, OTLP) | infra ▸ tdd-implementer + iac-ops |
| 5 | RN/Expo client → Go API; consolidate MUI web → RN Web | frontend ▸ tdd-implementer + tdd-frontend |
| 6 | Terraform/K8s manifests target the two Go services | infra ▸ reviewed only, apply gated |

## Why this is "V-model" and not just TDD
TDD is the inner loop of the bottom box. The V-model adds vertical traceability: each test layer
verifies a specific design artifact, and the heads pin skills to phases so the agent can't skip
from requirements straight to code — the hooks and the code-reviewer verdict are the structural
gates that enforce the climb back up.
