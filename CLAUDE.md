# Accounting CQRS — Go Rewrite

Double-ledger accounting system. Migrating Spring Boot/Java → Go/Gin while preserving the CQRS split and domain invariants.

## Methodology
- **V-model + TDD**, top-down (DB design → module design → implementation), Human-in-the-loop.
- Every implementation phase is gated by its mirrored verification phase. No code merges without its test layer green. See `V_MODEL_PLAN.md`.
- The V-model is applied **into an agent swarm**: 3 head agents (backend/frontend/infra) each own their domain's V and invoke the V-phase skills (domain-modeler → tdd-implementer/migration-mapper → code-reviewer), dispatching a ruflo sonnet swarm.

## Tech stack
- Backend: **Go 1.23 + Gin**. CQRS preserved as two services: `cmd/command`, `cmd/query`.
- DB: **PostgreSQL**. Codegen: **sqlc** (DB-first, mirrors old JOOQ workflow). Migrations: **golang-migrate**. Driver: **pgx/v5**. IDs: **UUIDv7**.
- Frontend: **React Native + Expo** (consolidate web MUI app into RN Web).
- IaC: **Terraform** (cloud-agnostic). Orchestration: **Kubernetes** (vendor-neutral). Observability: **OpenTelemetry** (vendor-neutral, OTLP exporter).
- CI: GitHub Actions. Testing: `go test` + testify, table-driven, AAA (Arrange-Act-Assert). Integration: testcontainers-go.

## Domain invariants (NON-NEGOTIABLE — enforce in code + tests)
- A `Ledger` is balanced: `SUM(debit) == SUM(credit)` across its `LedgerItems`. Reject any write that breaks this.
- Command side writes; query side reads. Query services MUST NOT mutate.
- Validate early and across layers: handler/DTO → domain → DB constraint.

## Package layout (per service)
```
internal/
  domain/        # entities, invariants, no deps on infra
  service/       # use cases, tx orchestration
  repository/    # sqlc-generated + thin wrappers
  handler/       # Gin handlers, DTO mapping
  config/        # env, otel setup
```

## Conventions
- Errors: wrap with `fmt.Errorf("...: %w", err)`. No naked `panic` outside `main`.
- No business logic in handlers. Map DTO↔domain explicitly (no reflection mappers).
- Avoid names near SQL reserved words (ubiquitous language).
- Every exported func that touches money has a table-driven test including an unbalanced-ledger case.

## Build / test / run
```bash
make generate        # sqlc + migrations
make test            # go test ./... -race
make lint            # golangci-lint run
make up              # docker compose up -d (postgres + services)
make migrate         # golang-migrate up
```

## When migrating a Java component
Use the `migration-mapper` skill (via the backend head). Java→Go mapping table lives in `.claude/skills/tdd-go/SKILL.md`.

## Imports
@.claude/rules/tdd.md
@.claude/rules/devops.md
@.claude/rules/secrets-vault.md
@.claude/rules/migrations.md
