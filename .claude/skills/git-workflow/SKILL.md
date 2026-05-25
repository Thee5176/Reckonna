---
name: git-workflow
description: Branching, pull-request, and issue workflow for this repo. Trunk-based with short-lived branches off develop, Conventional Commits + `Plan: S<n>`, CI + code-reviewer gate before merge. Use when creating branches, opening/merging PRs, or filing/decomposing issues.
---

# Git Workflow

Successful = trunk-based, short-lived branches, small PRs, automated gates (DORA). Aligns with
`.claude/rules/devops.md`. git-flow's long-lived release/feature branches are deliberately avoided —
they accrue merge debt at this team size.

## Branch model — two long-lived branches
- **`main`** — released + deployable. Tagged per release (SemVer). Protected: no direct commits, no force-push.
- **`develop`** — integration trunk, ALWAYS green (`code-reviewer` diffs against it). Protected: no direct commits.
- **work branches** — short-lived (≤ ~3 days), branched off `develop`, rebased often, deleted after merge.

## Branch naming
`<type>/<issue#-or-plan#>-<slug>` where `<type>` ∈ `feat|fix|chore|docs|ci|refactor|perf|test`.
- `feat/01-backend-cqrs-core` · `fix/42-balance-trigger` · `chore/00-bootstrap` · `ci/sonar-gate`
- Default: one branch per **feature plan**. Split per V-phase only if a slice lands independently — still one PR per landable slice.

## Commits (per `.claude/rules/devops.md`)
- **Conventional Commits only**: `type(scope): summary`.
- **One step = one commit**, in plan order, with trailer `Plan: S<n>`.
- **No squashing across steps. No force-push** to `main`/`develop`.
- TDD visible: the failing-test commit lands BEFORE its code commit.
- End commit body with the `Co-Authored-By` trailer.

## Issues — one per feature, decomposed into plan steps
Template:
```
Title: <type>: <feature>            e.g. "feat: backend CQRS core"
Body:
  ## Context        — why; link the spec/plan (plans/<feature>.md)
  ## Acceptance     — link the plan's AT/IT IDs
  ## Steps          — checklist S1..Sn mirroring Section 3 of the plan
  ## Definition of done — every AT+IT green; each step's unit test green
Labels: domain:{backend|frontend|infra} · type:{feat|fix|chore} · plan:<NN>
```
Lifecycle: **issue → branch → PR (`Closes #N`) → merge → auto-close**.
```bash
gh issue create --title "feat: backend CQRS core" --label "domain:backend,type:feat" --body-file -
```

## Pull requests
Open EARLY as a **draft**; mark ready when green. Keep diff small (< ~400 LOC).
- **Base**: `develop` for features/chores; `main` only for release or hotfix.
- Title = Conventional-Commit summary. Body template:
```
Closes #<issue>
Plan: <plans/feature.md> steps S<a>–S<b>

## What / Why
## Tests        — AT/IT green (list IDs + files); unit test per step
## Risk / Rollback
## Checklist
- [ ] CI green (go test -race · jest · e2e · terraform validate · Sonar · gitleaks)
- [ ] code-reviewer verdict: MERGE
- [ ] make docs-verify clean (openapi + tbls diff)
- [ ] no secrets (gitleaks + no-secrets hook)
```
- **Merge gates** (all required): CI green · `code-reviewer` returns **MERGE** · all review threads resolved.
- **Merge method: rebase-merge** — preserves per-step commits (NO squash), keeps `develop` history linear.
- `terraform apply` / `kubectl delete` are **human-only** — never automated in a PR.

## Release
- `develop → main` via a **release PR** when a milestone is green.
- Tag `vMAJOR.MINOR.PATCH` (SemVer) on `main`; changelog generated from Conventional Commits since the last tag.

## Hotfix
- `fix/<issue>-<slug>` off **`main`** → expedited PR into `main` → tag patch → **back-merge `main` into `develop`** (never let main drift ahead).

## Command reference
```bash
git switch -c feat/01-backend-cqrs-core develop      # branch off develop
# … one commit per step, each: git commit -m "feat(domain): … " -m "Plan: S2"
git push -u origin HEAD
gh pr create --base develop --fill --draft           # draft early
# … CI + code-reviewer …
gh pr ready
gh pr merge --rebase --delete-branch                 # rebase-merge, no squash
```

## Chains with
`plan-writer` (defines steps S<n>) → branch + commits here → `code-reviewer` (MERGE gate) →
`plan-tracker` (logs landed steps to `*.impl.md`).

## Anti-patterns (rejected)
Long-lived feature branches · squashing step history · force-push to shared branches · direct commits to
`main`/`develop` · PRs that mix multiple plan steps without `Plan:` trailers · merging red CI.
