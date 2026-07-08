---
name: qa-plan
description: >-
  Plan the validation side of a V-model plan — read its acceptance-criteria / AT ledger and
  enumerate EVERY validation criterion into the agent todo list (one todo per criterion, each
  stating what to check + how (exact command/API) + expected result + whether it is
  human-gated). The planning step that feeds /validate-at. Does NOT validate — it lists.
  Use when asked to "qa-plan", "plan the validation", "list the acceptance criteria as todos",
  "set up the validation checklist", or before running the /validate-at loop.
---

# /qa-plan — enumerate every validation criterion into the agent todo list

The first step of the V-model right side. Reads the plan and turns its acceptance/integration
criteria into a tracked todo ledger (TaskCreate). Output = the validation checklist that
`/validate-at` then works ONE at a time. It plans; it does not validate.

## Steps

1. **Locate the plan** — named in the conversation, else search `plans/*.md`, `docs/specs/*.md`
   for the AT ledger matching the branch/task. If ambiguous, ask which plan.
2. **Extract EVERY criterion** — no cherry-picking:
   - Section 1 acceptance tests (`AT<k>`: Given / When / Then + expected).
   - Section 2 integration tests (CQRS write→read, tx atomicity, DTO/contract shapes).
   - Any explicit checklist in the plan (`A1…`, `B1…`, `C0…`).
   For each capture: **id, what-to-check, how (exact command/API), expected result,
   human-gated?** (security review, external checks, merges, releases).
3. **Write one todo per criterion** via TaskCreate: `subject = "AT<k> — <requirement>"`,
   `description = what + how + expected (+ HUMAN-GATED note)`. Preserve phase + order
   (A→B→C / ledger order). Set `activeForm` for the spinner.
4. **Report the ledger** — N criteria created, which are human-gated, the order. Mark none
   done. Hand off to `/validate-at`.

## Rules

- Enumerate EVERYTHING. Gap-discovery items (an AT the plan is missing) are added as todos too.
- One todo per criterion, ordered. Never validate here — that is `/validate-at`.
- Idempotent: `TaskList` first; do not duplicate existing criterion todos.
- Human-gated criteria are still listed (flagged), never dropped.
