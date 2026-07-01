# Branch protection — setup (apply on GitHub)

Enforces the rules `.claude/skills/git-workflow` assumes. Apply once, on `Thee5176/Reckonnna`.
`gh` is not installed in the dev container by default — install it (`gh` is in the toolchain) or use the
GitHub UI steps. These are **human-applied** (admin scope), like `terraform apply`.

> ✅ **APPLIED 2026-05-25** on `main` + `develop` via the API: strict required checks (the 5 display
> names below), `required_linear_history`, no force-push, no deletions, `enforce_admins=false`,
> `required_approving_review_count=0` (solo). Repo: rebase-merge only, auto-delete head branches.
> `SonarQube` is intentionally NOT required (gated behind the `SONAR_ENABLED` repo var).

## Rules (both `main` and `develop`)
- Require a pull request before merging — **no direct commits**.
- Require a review: teams = 1 approval; **solo = 0** (GitHub blocks self-approval) — rely on CI + the `code-reviewer` agent gate.
- Require status checks (job **display names**): `Go build · test -race · lint`, `Jest`,
  `E2E (testcontainers)`, `Terraform validate`, `gitleaks (secret scan)`. (`SonarQube` added once enabled.)
- Require branches up to date before merge.
- **Block force-pushes** and **block deletions**.
- Require linear history (we **rebase-merge**, never squash across steps).
- `main` additionally: restrict who can merge (release PRs only).

## Via `gh` (after `gh auth login`)
```bash
REPO=Thee5176/Reckonnna
for BR in main develop; do
  gh api -X PUT "repos/$REPO/branches/$BR/protection" \
    -H "Accept: application/vnd.github+json" \
    -f 'required_pull_request_reviews[required_approving_review_count]=1' \
    -f 'required_status_checks[strict]=true' \
    -f 'required_status_checks[contexts][]=backend' \
    -f 'required_status_checks[contexts][]=frontend' \
    -f 'required_status_checks[contexts][]=e2e' \
    -f 'required_status_checks[contexts][]=terraform' \
    -f 'required_status_checks[contexts][]=gitleaks' \
    -f 'required_status_checks[contexts][]=sonar' \
    -F 'enforce_admins=true' \
    -F 'restrictions=null' \
    -F 'allow_force_pushes=false' \
    -F 'allow_deletions=false' \
    -F 'required_linear_history=true'
done
# Repo merge settings: enable rebase-merge only, auto-delete head branches
gh api -X PATCH "repos/$REPO" \
  -F allow_merge_commit=false -F allow_squash_merge=false \
  -F allow_rebase_merge=true  -F delete_branch_on_merge=true
```

## Via GitHub UI
Settings → Branches → Add rule, for `main` and `develop`:
- ☑ Require a pull request before merging · ☑ Require approvals (1)
- ☑ Require status checks to pass · ☑ Require branches to be up to date · add: backend, frontend, e2e, terraform, gitleaks, sonar
- ☑ Require linear history · ☑ Do not allow force pushes · ☑ Do not allow deletions
- Settings → General → Pull Requests: allow **Rebase merging** only; ☑ Automatically delete head branches.

> Status-check names must match the CI job names in `.github/workflows/ci.yml`. They appear in the
> "Require status checks" search box only after the workflow has run at least once.
