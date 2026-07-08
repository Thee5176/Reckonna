# V-model Right Side — Validation Skill Pipeline (replan)

## Context

The right side of the V (verification/validation) is covered by a set of Claude skills. They
must run **bottom-up** (prove the small things first) and each layer should **delegate** to the
one below, not re-implement it. The current `/pr-resolve` bundles QA + per-AT validation +
review + merge into one skill — that duplicates work and inverts the order. This replan splits
the right side into four ordered skills, each owning one V-model verification phase.

## The mapping (left → right, bottom-up execution order)

```
LEFT (build)                 RIGHT (verify)                 SKILL              runs
Implementation      ───►  Unit / component tests     ─►  (tdd-* skills)      during build
Module design       ───►  Integration tests          ─►  /qa-validate  ┐
Requirements (AT)   ───►  Acceptance test (one AT)   ─►  /qa-validate  ┘   per AT
        ▲                 Acceptance ledger (all ATs) ─►  /validate-at      loop over ATs
        │                 PR code review              ─►  /pr-review-dispatch after ATs green
        └──── resolve ───  Merge / release            ─►  /pr-resolve        orchestrates all
```

**Execution order (do NOT skip a layer):**
`/qa-validate` (one AT) → `/validate-at` (all ATs) → `/pr-review-dispatch` (review) → `/pr-resolve` (merge).

## Skill responsibilities (each delegates down, never duplicates)

| Skill | V phase | Owns | Delegates to | Stops when |
|-------|---------|------|--------------|-----------|
| `/qa-validate` | one acceptance/integration test | QA a single AT: derive it as a committed retestable test, detailed assertions + invalid case, run it, close on a clean Sonar gate, mark it | tdd-* test conventions | 1 AT validated (human-gated `[~]`→`[x]`) |
| `/validate-at` (NEW) | the whole AT ledger | Loop: call `/qa-validate` for each `[ ]` AT in order until all `[x]`; track the todo ledger; surface human-gated ATs | `/qa-validate` per AT | all ATs `[x]`, or a `[~]`/human gate |
| `/pr-review-dispatch` | PR-level review | Dispatch code-reviewer(s) on the PR diff → require MERGE verdict; a BLOCK is a real fix that loops back to `/qa-validate` (new regression AT) | code-reviewer agents | MERGE verdict or a BLOCK routed back down |
| `/pr-resolve` | resolve + release | Orchestrate the merge train: prereqs, rebase/conflict-resolve, merge (no squash), combined develop scan, release; ONLY after the three below are green | `/validate-at` + `/pr-review-dispatch` | PR merged / released (human-gated) |

## The refactor this replan asks for

1. **`/qa-validate`** — keep as the single-AT validator (already built). It is the leaf.
2. **`/validate-at`** — NEW skill: the loop driver. Runs `/qa-validate` over the ledger one AT
   at a time (like running `/ship-at` repeatedly), stopping on `[~]` or human gates. Mirrors
   `/ship-at`'s "one per run" but wraps the *validation* side.
3. **`/pr-review-dispatch`** — keep as the review gate; it runs AFTER `/validate-at` reports all
   ATs green. A BLOCK finding creates a new regression AT and drops back to `/qa-validate`.
4. **`/pr-resolve`** — REFACTOR from "does everything" to "orchestrates": its Phase-A/B
   per-criterion QA becomes a call to `/validate-at`; its review step becomes a call to
   `/pr-review-dispatch`; it retains only the merge/rebase/combined-scan/release (Phase C). The
   V-model + committed-test + mandatory-Sonar-gate detail lives in `/qa-validate` and is
   inherited, not copied.

## Invariants (unchanged, enforced at the `/qa-validate` leaf so all layers inherit them)

- Every AT is a COMMITTED, retestable test (repo convention path, traceable header, named by AT).
- Detailed value assertions + a mandatory invalid case for every money/domain AT (借方≠貸方 → error).
- Mandatory closing gate: always run SonarQube, require `STATUS: OK` + 0 open issues before an
  AT is marked done.
- One AT / one criterion per run; human owns acceptance, merge, release; prefer fixing code over
  marking-safe.

## Build order to realize this replan

1. `/validate-at` skill (loop over `/qa-validate`) — Reckonna + Homelab_Setup.
2. Refactor `/pr-resolve` to delegate (call `/validate-at` + `/pr-review-dispatch`; keep only merge/release).
3. Confirm `/pr-review-dispatch` sits between `/validate-at` and `/pr-resolve`.
