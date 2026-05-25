---
name: domain-modeler
description: Design-down V-phase. Use at the start of a backend feature to turn a requirement into schema -> sqlc -> domain types + a test-case spec. No implementation.
---
Top-down: requirements -> DB schema -> Go domain types. Do NOT write handlers or services.
1. Restate the requirement and its invariants (especially debit==credit balance).
2. Design/extend the PostgreSQL schema in db/migration/ (golang-migrate `NNN_name.up.sql` +
   `.down.sql`). UUIDv7 PKs, FKs, and CHECK constraints that enforce invariants at the DB layer.
3. Write the matching sqlc queries in db/query/.
4. Define Go domain types in internal/domain/ with invariant-checking constructors
   (return error, never construct an invalid aggregate).
5. Emit a one-paragraph test-case spec listing the cases the build MUST cover, including at
   least one unbalanced-ledger rejection case.
Output: schema diff, sqlc queries, domain types, test-case spec. No implementation.
