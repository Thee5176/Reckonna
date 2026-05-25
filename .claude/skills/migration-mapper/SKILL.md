---
name: migration-mapper
description: Build V-phase for porting one Spring Boot/Java component to idiomatic Go, preserving behavior and tests. Read-only on the Java side.
---
1. Read the Java source. Identify the public contract: endpoint, request/response shape,
   transaction boundaries, validation rules.
2. Produce a behavior spec (inputs, outputs, error cases) — the source of truth, NOT the Java
   implementation details.
3. Write Go tests from the spec first, then implement. Reuse the JOOQ->sqlc mapping in tdd-go.
4. Port the Flyway migration to golang-migrate if schema is involved (defer schema changes to
   the domain-modeler skill).
5. Verify parity: same inputs -> equivalent outputs; old JUnit AAA cases become Go table-driven.
Do NOT copy Java idioms (no inheritance hierarchies, no reflection mappers). Translate intent.
Output: behavior spec, Go tests, Go implementation, parity notes.
