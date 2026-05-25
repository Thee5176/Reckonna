---
name: plan-writer
description: Acts as the planner (with superpowers). Outputs plans/<feature>.md containing (1) the acceptance-test spec, (2) the integration-test spec, and (3) a realistic step-by-step commit list with unit tests. The heads read this plan and implement its specs.
---
# Plan Writer — planner output spec
The approved plan is the single source of truth every head reads. It MUST write the test specs
for ALL THREE V-model layers, top-down:
  acceptance (<- business requirements) -> integration (<- architecture) -> unit (<- step)

## Section 1 — Acceptance-test spec (E2E)   [from business requirements]
| ID  | Given / When / Then                                                           | Domain | Test file                        |
|-----|-------------------------------------------------------------------------------|--------|----------------------------------|
| AT1 | Given balance 0 / When post debit 1000 + credit 1000 / Then recorded, 借方=貸方 | e2e    | e2e/journal-entry.e2e.ts         |
| AT2 | Given unbalanced entry / When submit / Then rejected with 422 + message       | e2e    | e2e/journal-entry-invalid.e2e.ts |

## Section 2 — Integration-test spec   [from architecture / CQRS contracts]
| ID  | Condition to verify                                                    | Domain   | Test file                                |
|-----|------------------------------------------------------------------------|----------|------------------------------------------|
| IT1 | command.PostLedger -> query.GetLedger returns same id; items sum equal | backend  | internal/query/ledger_it_test.go         |
| IT2 | tx fails -> no partial write remains                                   | backend  | internal/handler/ledger_tx_test.go       |
| IT3 | Form submit hits API contract; API error path renders message          | frontend | components/JournalEntryForm.int.test.tsx |

## Section 3 — Implementation steps (one commit each; unit test per step)
- One step = one commit = one PR-reviewable change. >~5 files or cross-domain -> SPLIT.
- Each step compiles & passes ON ITS OWN. TDD visible: failing-test commit BEFORE code commit.
| ID | Commit message (verbatim)                      | Files                                                        | Domain   | Depends | Unit test                     |
|----|------------------------------------------------|--------------------------------------------------------------|----------|---------|-------------------------------|
| S1 | test(domain): failing Ledger balance test      | internal/domain/ledger_test.go                               | backend  | -       | TestNewLedger/unbalanced (RED)|
| S2 | feat(domain): Ledger invariant constructor     | internal/domain/ledger.go                                    | backend  | S1      | TestNewLedger (GREEN)         |
| S3 | feat(db): ledger + ledger_items schema         | db/migration/001_ledger.up.sql, 001_ledger.down.sql          | backend  | -       | migrate up/down               |
| S4 | feat(db): sqlc queries for ledger              | db/query/ledger.sql, internal/repository/ledger_gen.go       | backend  | S3      | sqlc generate; compile        |
| S5 | feat(api): PostLedger command handler          | internal/handler/ledger.go, cmd/command/main.go              | backend  | S2,S4   | TestPostLedger (satisfies IT1)|
| S6 | feat(ui): useLedger hook                        | app/hooks/useLedger.ts                                       | frontend | S5      | useLedger.test.ts             |
| S7 | feat(ui): JournalEntryForm component           | components/JournalEntryForm.tsx, JournalEntryForm.stories.tsx| frontend | S6      | Form.test.tsx (satisfies IT3) |
| S8 | chore(infra): OTel span on PostLedger + CI job | infra/otel.tf, .github/workflows/ci.yml                      | infra    | S5      | terraform validate            |

## Hand-off to the heads
Each head reads this file and writes its assigned AT/IT specs as FAILING tests FIRST (red),
then runs domain-modeler -> tdd-implementer/migration-mapper -> code-reviewer to green. "Done"
= every AT + IT green. plan-tracker logs each landed step to *.impl.md.
