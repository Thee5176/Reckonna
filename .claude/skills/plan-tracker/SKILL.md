---
name: plan-tracker
description: Document implementation progress next to the agreed plan so a developer can check actual work against the plan.
---
# Plan Tracker
Maintain an implementation log co-located with the plan, mirroring each step + each AT/IT.
- plans/<feature>.md        # agreed plan - IMMUTABLE after approval
- plans/<feature>.impl.md   # implementation log - updated as steps/specs land
Record per ID: Status (done|partial|skipped|deviated), Commit (SHA+msg), Tests (file::name),
Deviation (what+why). "done" = all AT + IT green and every step's unit test green.
| ID  | Status   | Commit                               | Tests          | Deviation                   |
|-----|----------|--------------------------------------|----------------|-----------------------------|
| AT1 | done     | -                                    | e2e green      | -                           |
| IT1 | done     | a1b2c3 feat(api): PostLedger         | ledger_it_test | -                           |
| S5  | deviated | a1b2c3 feat(api): PostLedger         | TestPostLedger | split into 2 endpoints; note|
