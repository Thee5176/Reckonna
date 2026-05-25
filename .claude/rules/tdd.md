---
description: TDD policy (loads every session, all agents)
---
# TDD — non-negotiable
- No production code without a failing test first. Order is always Red -> Green -> Refactor.
- For each feature, write the plan's acceptance (AT) + integration (IT) specs as failing
  tests FIRST, then the unit step tests, then code.
- Domain/money paths MUST include an invalid case (借方≠貸方 -> error).
- Never delete or weaken a test to make a build pass. Fix the code, or change the spec first.
- "Done" = every AT + IT in the plan is green and each step's unit test is green.
# Enforced by: verify-*.sh + check-ledger-invariant.sh
