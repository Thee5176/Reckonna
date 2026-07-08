# Merge Train M-A — Land infra + backend to trunk (unblock plan 05)

**Status:** proposed runbook · **Owner:** infra-engineer (HEAD) + human for the `main` release
**Goal:** integrate plans 01 → 02 → 03 into `develop`, then release `develop → main` so plan 03's
CI publishes the ghcr images that plan 05 (deploy) pulls. This is the single highest-leverage move:
until it lands, everything downstream (05 deploy, 06 otel) is blocked on images that do not exist.

## Why this exists

The trunk is nearly empty. `main` is `Initial commit`; `develop` has only the toolchain/Vault
bootstrap (plan 00) plus the `no-secrets.sh` hook fix. Every real deliverable is stranded on an
independent feature branch:

| Plan | Branch (⚠ numbering drift) | Built | vs `develop` |
|------|----------------------------|-------|--------------|
| 01 postgres-tailnet | `feat/02-infra-postgres-tailnet` | pg StatefulSet + netpol + tailscale | behind 2, ahead 54 |
| 02 k8s + cloudflare tunnel | `feat/plan02-cloudflare-tunnel` | reckonna-app nginx harness + cloudflared + tunnel tf | behind 2, ahead 69 |
| 03 backend CQRS | `feat/03-backend-cqrs-core` | 53 Go files + `build/Dockerfile.{command,query}` + ci.yml image job | behind 1, ahead 85 |

> **⚠ Branch/plan numbering drift.** Postgres was plan **02** before a renumber and its branch kept
> the old number: **plan 01 = `feat/02-infra-postgres-tailnet`**. Do not merge by number — merge by
> the mapping above. Ignore the stale `origin/feat/03-cache-otel-sidecar` and `feat/05-otel-*`
> (that is plan **06** otel, not part of M-A).

The three branches are **independent off `develop`** (03 does NOT contain 01 or 02), and all three
edit shared files (`Makefile`, `.github/workflows/ci.yml`, `infra/k8s/**`, `plans/**`, `docs/**`).
Conflicts are therefore expected on every merge after the first — hence a deliberate ordered train,
not three parallel PRs.

## The train

```
develop ──> +01 postgres ──> +02 tunnel/harness ──> +03 backend ──> RELEASE develop→main
   │            │                  │                     │                     │
   │        rebase+PR          rebase+PR             rebase+PR           triggers 03 CI:
   │        review+merge       review+merge          review+merge       ghcr image publish
   └────────────────────────────────────────────────────────────────────────────┘
                                                                    unblocks plan 05 deploy
```

Order rationale: dependency order (03's plan declares 01+02 as prerequisites) AND conflict
minimization (land the smaller infra surfaces first so the large backend rebases onto a settled
tree, not the reverse).

## Per-branch procedure (repeat for 01, then 02, then 03)

Trunk-based per `.claude/rules/devops.md` + the `git-workflow` skill: short-lived branches off
`develop`, Conventional Commits, no force-push to shared branches, CI + code-reviewer gate before
merge. `terraform apply` / `kubectl apply` stay human-only and are NOT part of this train (M-A is
code integration only; live apply is plan 05's M-C).

1. **Rebase onto latest `develop`** (each branch is behind by 1–2 commits):
   ```
   git fetch origin
   git switch <branch> && git rebase origin/develop
   ```
   Resolve conflicts (hotspots below). Never force-push a branch someone else shares; if the branch
   is solo, `--force-with-lease` after rebase is fine.
2. **Local verification** — CI has known gaps (see "CI coverage gap" below), so run these locally
   before opening the PR:
   - `make test` (`go test ./... -race`) — for 03. Backend IT/e2e need a DB: set
     `RECKONNA_TEST_DATABASE_URL` (per-test schema isolation) or a podman-socket `DOCKER_HOST` for
     testcontainers; migrate a fresh DB (gaps go dirty).
   - `make k8s-validate` (`kubeconform -strict` on each `infra/k8s/**` base) — for 01/02.
   - `make tf-validate` (`terraform fmt -check && validate`) — for 01/02.
   - `gitleaks detect --no-git` — all three.
   - `bash tests/*.sh` — the repo's grep/IT shell tests (CI does not run these).
3. **Open PR → `develop`.** Base MUST be `develop`, not `main`. Title = Conventional Commit; body
   references the plan. If you retarget an existing PR's base, see the CI-retrigger gotcha below.
4. **Gates:** CI green (`go test -race` + jest + terraform validate + Sonar + gitleaks) AND the
   `code-reviewer` skill returns MERGE (read-only pre-merge gate).
5. **Merge** with a merge commit (no squash across steps — preserve the `Plan: S<n>` history per
   `devops.md`). Then move to the next branch and rebase it onto the new `develop` tip.

## Conflict hotspots (independent branches, shared files)

| File / dir | Why it conflicts | Resolution |
|------------|------------------|------------|
| `Makefile` | each plan appends targets | union the targets; keep one `help`/`.PHONY` block |
| `.github/workflows/ci.yml` | each plan adds jobs | union jobs; 03 owns the image-publish job |
| `infra/k8s/reckonna-app/kustomization.yaml` | 02 seeds it; later plans extend | take 02's base; plan 05 (later) evolves it |
| `go.mod` / `go.sum` | only 03 touches | take 03's; run `go mod tidy` post-merge |
| `plans/**`, `docs/**` | multiple plan docs | additive; no real conflict, accept both |
| `.gitleaks.toml`, `sonar-project.properties` | tuning per branch | union rules |

## Release step (develop → main) — the payoff

`main` is currently `Initial commit`. After 01+02+03 are on `develop` and green:

1. Open the release PR `develop → main` (human-gated; this is the trunk release).
2. On merge to `main`, plan 03's `ci.yml` image job builds + pushes
   `ghcr.io/thee5176/reckonna-{command,query}:<sha>` (S17b: publish on push to `main`).
3. **Verify the images exist** before starting plan 05's live apply:
   ```
   gh api /users/thee5176/packages/container/reckonna-command/versions --jq '.[0].metadata.container.tags'
   # (or check the GitHub Packages UI for reckonna-command / reckonna-query)
   ```
   Canonical repo is `Thee5176/Reckonna` (two n's) — a 3-n alias breaks the packages API.

## CI-retrigger gotcha

A force-push that also retargets a PR's base **suppresses** the `synchronize` CI run — the PR shows
stale/green checks that never re-ran. If checks do not fire after a rebase+retarget, **close and
reopen the PR** to force a fresh run. Prefer `git rebase --onto` for clean history; expect the same
conflicts to recur across successive rebases of the large 03 branch.

## Definition of done (M-A)

- [ ] Plan 01 merged to `develop` (pg + netpol + tailscale manifests).
- [ ] Plan 02 merged to `develop` (reckonna-app harness + cloudflared + tunnel tf).
- [ ] Plan 03 merged to `develop` (Go command+query + Dockerfiles + image job).
- [ ] `develop` green: `go test -race`, kubeconform, tf validate, gitleaks, `bash tests/*.sh`.
- [ ] `develop → main` released; `ghcr.io/thee5176/reckonna-{command,query}` published + tag verified.
- [ ] Downstream unblocked: plan 05 (M-C) can pin real image SHAs; plan 06 (M-E) has a running target.

## Not in this train

- **Plan 05 deploy** (M-C) — approve its draft + implement in parallel with M-A, but its live apply
  waits on the published images from this train.
- **Plan 04 frontend** (M-D) and **plan 06 otel** (M-E) — separate branches, off the critical path.
- **Keycloak** (M-B) — plan 03's `external_prereq`; unowned. Blocks 03's auth ATs and 05's smoke.
  Assign an owner before relying on the auth-gated ATs, even though M-A can merge the code without it.
- **Any `kubectl apply` / `terraform apply`** — human-only, belongs to plan 05's rollout, not here.
