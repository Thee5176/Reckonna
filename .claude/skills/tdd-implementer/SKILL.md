---
name: tdd-implementer
description: Build + unit-verify V-phase. Strict TDD red-green-refactor for service/handler/repository code. Never write production code before a failing test.
---
Cycle per unit:
1. RED  — table-driven test in *_test.go (testify, AAA). Include the domain-modeler edge case
   (always an unbalanced-ledger case for money paths). `go test ./... -run <Name>` must FAIL
   for the right reason.
2. GREEN — minimum code to pass.
3. REFACTOR — clean up; keep tests green. The PostToolUse hook runs gofmt + golangci-lint +
   go test on every edit; fix anything it blocks before continuing.
Rules: command-side mutates, query-side read-only; pgx tx for multi-entity (Ledger+Items atomic);
wrap errors with %w; no business logic in handlers. Stop and ask the human before changing a DB
schema or domain invariant (domain-modeler's job).
