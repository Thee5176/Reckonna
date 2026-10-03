# Hybrid CI/CD — CI on GitHub Actions, CD on GitLab

Follow-along guide for the push-mirror strategy: GitHub stays the source of
truth and runs all CI (tests, lint, Sonar, gitleaks); GitLab receives a
mirror of `main` **only after CI is green** and runs CD only (package +
deploy to the homelab k3s cluster).

```
GitHub repo (source of truth)
  │  push / PR → CI: backend · frontend · e2e · terraform ·
  │                    infra-manifests · gitleaks · sonar-gate
  │
  └─ all green on main → mirror-gitlab job → git push --force gitlab main
                                                  │
                                       GitLab repo (mirror, CD only)
                                          .gitlab-ci.yml → pipeline
                                          stages: package → deploy
                                          runner: homelab (inside cluster network)
                                          target: k3s homelab
```

**The seam contract:** the mirror preserves history, so GitLab's
`$CI_COMMIT_SHORT_SHA` equals GitHub's `${{ github.sha }}`. The image tag is
traceable across both platforms with zero extra plumbing. The mirror push
itself is the quality gate — no green CI, no push, no CD.

---

## Files touched

| File | Side | Status |
|------|------|--------|
| `.gitlab-ci.yml` (repo root) | GitLab | scaffold ready — you fill 4 TODO slots |
| `.github/workflows/ci.yml` | GitHub | add `mirror-gitlab` job (snippet below) |

The scaffold lives at the worktree it was drafted in; copy it into the main
checkout:

```bash
cp .claude/worktrees/gitlab-cd-scaffold/.gitlab-ci.yml .gitlab-ci.yml
```

The full scaffold is also reproduced in [Appendix A](#appendix-a---gitlab-ciyml-scaffold)
so this doc stays self-contained.

---

## Step 0 — Prerequisites checklist

- [ ] GitLab project created (self-hosted or gitlab.com), e.g. `<group>/Reckonna`
- [ ] A GitLab runner registered that can reach the k3s API server
      (the homelab runner — same network position as the GitHub
      `[self-hosted, tailscale]` runner used by the Sonar job)
- [ ] Know the runner's tag (scaffold assumes `homelab` — change if different)
- [ ] Vault reachable from that runner (AppRole or OIDC per
      `.claude/rules/secrets-vault.md`)
- [ ] GitLab project access token with `write_repository` scope (for the mirror)

---

## Step 1 — GitHub side: add the mirror job

Paste into `.github/workflows/ci.yml` as a new job. Note: repo hooks block
agent writes to workflow files — paste this yourself.

```yaml
  mirror-gitlab:
    name: Push mirror → GitLab (CD trigger)
    needs: [backend, frontend, e2e, terraform, infra-manifests, gitleaks, sonar-gate]
    if: github.event_name == 'push' && github.ref == 'refs/heads/main'
    runs-on: ubuntu-latest
    permissions:
      contents: read
    steps:
      - uses: actions/checkout@v4
        with: { fetch-depth: 0 }
      - name: Push to GitLab
        run: |
          git remote add gitlab "https://oauth2:${{ secrets.GITLAB_MIRROR_TOKEN }}@<GITLAB_HOST>/<group>/Reckonna.git"
          git push gitlab main --force
```

Why it is shaped this way:

- **`needs: [... sonar-gate]`, not `sonar`** — GitHub *skips* dependent jobs
  when a needed job is skipped. `sonar` is conditionally skipped
  (`SONAR_ENABLED` gate); `sonar-gate` always reports. Same trick already
  used for the required status check.
- **`if:` push + main only** — PR CI runs never mirror. Merging to main
  fires the push event, which fires CI, which mirrors on green.
- **`--force`** — mirror-only hygiene. A history rewrite on main would
  otherwise wedge the mirror with a non-fast-forward rejection. This is not
  a shared-branch force-push violation: the GitLab repo is a downstream
  mirror nobody commits to directly.
- Replace `<GITLAB_HOST>` and `<group>` with real values.

### Secret setup for the mirror

Per `.claude/rules/secrets-vault.md`, Vault is the only source:

1. Create the GitLab project access token (role `Maintainer` or `Developer`
   with `write_repository`; scope to the mirror project).
2. Store it in Vault: `vault kv put secret/app/reckonna/gitlab-mirror-token token=<TOKEN>`
3. Put the same value into GitHub repo secret `GITLAB_MIRROR_TOKEN`
   (GitHub secrets are the runtime delivery mechanism; Vault remains the
   recorded source of truth). Never commit the token to any file.

---

## Step 2 — GitLab side: runner

1. GitLab project → Settings → CI/CD → Runners → register the homelab
   runner (or reuse an existing group runner).
2. Give it the tag the scaffold expects: `homelab`.
3. Verify the runner executor can pull images (`golang:1.23`, `node:20`,
   `bitnami/kubectl:1.31`) and reach: the OCI registry, Vault, the k3s API.
4. The runner needs `vault` CLI available in the job image or mounted —
   plan for `vault kv get` at job runtime (see Step 4).

---

## Step 3 — GitLab side: the scaffold's fixed parts

`.gitlab-ci.yml` ships with these already decided (do not need changes):

- **`workflow:rules:`** — pipelines for `$CI_COMMIT_BRANCH == "main"` only.
  Kills double pipelines: without it GitLab fires a pipeline for every
  mirrored ref, and future tag pushes would deploy unintentionally.
- **`stages: [package, deploy]`** — CD only. No test/lint jobs ever go in
  this file; that duplicates GitHub CI and breaks the "green CI = gate"
  model.
- **`IMAGE_TAG: $CI_COMMIT_SHORT_SHA`** — the seam contract.
- **`tags: [homelab]`** on every job — everything runs on the self-hosted
  runner inside the homelab network.

---

## Step 4 — Your part: 4 decisions, 4 TODO slots

Each slot is 5–10 lines. Trade-offs below; write the scripts in
`.gitlab-ci.yml` where the `TODO(you)` comments sit.

### Decision 1 — `package:backend` build strategy

| Strategy | Pros | Cons |
|----------|------|------|
| `services: docker:27-dind` | Familiar docker CLI | Daemon spin-up per job (slow), needs privileged runner |
| `buildah` (rootless) | Fast, no daemon, plays well in k8s-hosted runners | CLI differs slightly from docker |
| `kaniko` | No privileged mode, Google-maintained | Layer-cache quirks, slower cold |
| Runner docker socket | Fastest, simplest | Least isolated — any job gets daemon root |

Runner already lives in the homelab cluster → **buildah or socket** are the
natural picks. Two images to build: `cmd/command` and `cmd/query` (or one
image, two entrypoints — your call, affects the Dockerfile too).

Registry creds + push:

```bash
# GitLab built-in registry — creds are automatic, no Vault needed:
echo "$CI_REGISTRY_PASSWORD" | buildah login -u "$CI_REGISTRY_USER" --password-stdin "$CI_REGISTRY"
```

### Decision 2 — `package:frontend` artifact strategy

| Strategy | Pros | Cons |
|----------|------|------|
| `npx expo export --platform web` → static bundle | Self-hosted, fits k3s (nginx image or existing static serve), no cloud dependency | You own the serving infra |
| EAS build/update (Expo cloud) | OTA updates, managed native builds | Needs `EXPO_TOKEN` (Vault), external dependency, paid tiers |

Web-only consolidation (MUI app folded into RN Web) → **export path** is
simpler. `npm ci` first either way.

### Decision 3 — deploy gates

`when: manual` is commented out in both deploy jobs. The devops rule
(`.claude/rules/devops.md`) forces human-only on `terraform apply` and
`kubectl delete` — plain `kubectl apply` of the kustomize render may stay
automated. Uncomment if you want a human click before every deploy anyway.
GitLab manual jobs also enable on-demand redeploy of the last pipeline.

### Decision 4 — registry

| Choice | Pros | Cons |
|--------|------|------|
| GitLab built-in (`$CI_REGISTRY`) | Creds automatic (`$CI_REGISTRY_USER`/`_PASSWORD`), zero Vault plumbing | Images locked to GitLab project |
| ghcr.io | GitHub CI could push there too later (single registry both sides) | Needs PAT in Vault + login step |

Built-in = least plumbing. Switch later is cheap (one variable + login line).

### Vault at deploy time (both deploy jobs)

Kubeconfig never lives in a file or CI variable — pull at runtime:

```bash
vault kv get -mount=secret -field=kubeconfig app/reckonna/kubeconfig > /tmp/kubeconfig
export KUBECONFIG=/tmp/kubeconfig
```

Runner auth to Vault: AppRole (role-id/secret-id as **masked, protected**
GitLab CI/CD variables — the only acceptable GitLab-side storage, mirroring
how `VAULT_ROLE_ID`/`VAULT_SECRET_ID` are handled on the GitHub side) or
OIDC if your Vault + GitLab support it.

Backend deploy core (after kubeconfig is set):

```bash
cd infra/k8s/reckonna-app
kustomize edit set image backend="$BACKEND_IMAGE:$IMAGE_TAG"   # or edit images: in overlay
kubectl kustomize . | kubectl apply -f -
kubectl rollout status deploy/command deploy/query -n reckonna --timeout=120s
```

---

## Step 5 — Validate before first run

1. **Lint the YAML** — install `glci` (offline, no token):
   ```bash
   go install gitlab.com/gitlab-org/ci-cd/runner-tools/glci@latest   # or curl installer
   glci lint .gitlab-ci.yml
   ```
   Fallback: GitLab UI → CI/CD → Pipelines → CI/CD Lint (needs the repo pushed).
   (At scaffold time neither `glci` nor `glab` was installed locally; only a
   YAML parse-check ran.)
2. **Dry-run the mirror manually** before wiring it into CI:
   ```bash
   git remote add gitlab "https://oauth2:<TOKEN>@<GITLAB_HOST>/<group>/Reckonna.git"
   git push gitlab main --force
   ```
   Confirm the GitLab pipeline fires and `workflow:rules:` admits it.
3. **First pipeline**: watch `package:*` jobs pull images and reach the
   registry; watch `deploy:*` reach the k3s API.

---

## Step 6 — Commit discipline

Per devops rules: Conventional Commits, one step = one commit.

```
ci(gitlab): add CD-only pipeline scaffold (package + deploy stages)
ci(github): add mirror-gitlab job to push main to GitLab after CI green
feat(cd): implement backend package/deploy scripts
feat(cd): implement frontend package/deploy scripts
```

---

## Operations notes

- **GitLab repo is downstream.** Never commit directly to it; fixes go to
  GitHub, mirror carries them down. Direct GitLab commits get clobbered by
  the next `--force` mirror push.
- **"CI green" now spans two platforms:** GitHub CI green = merge gate
  (unchanged); GitLab pipeline green = deploy succeeded. Branch protection /
  required checks stay on GitHub only.
- **Redeploy without new code:** GitLab UI → retry the deploy job (manual
  gate makes this safe), or re-run the mirror job on GitHub.
- **Disable GitLab CI entirely** (Settings → General → Visibility → CI/CD)
  only if you want the mirror without pipelines — not the goal here.
- **Runner single point of failure:** the homelab runner down = no deploys.
  GitHub CI keeps running (cloud-hosted jobs), so merges stay unblocked;
  only CD pauses.

---

## Appendix A — `.gitlab-ci.yml` scaffold

```yaml
# =============================================================================
# Reckonna CD — GitLab side of the hybrid split.
#   CI (tests/lint/sonar/gitleaks) stays on GitHub Actions. Green CI there is
#   the quality gate. This file is CD ONLY: package + deploy to homelab k3s.
#   Trigger: GitHub `mirror-gitlab` job push-mirrors main here AFTER CI green.
#   No tests run here. Do not add them — that duplicates GitHub CI.
# Secrets policy (.claude/rules/secrets-vault.md): NO secrets in this file.
#   Runner pulls kubeconfig / registry creds from Vault at job runtime:
#     vault kv get -mount=secret app/reckonna/<key>
# =============================================================================

workflow:
  rules:
    # Mirror lands as a branch push. Pipelines for main ONLY — blocks tags,
    # MRs, schedules, and stray branch pushes from doubling up.
    - if: $CI_COMMIT_BRANCH == "main"

stages:
  - package   # build shippable artifacts (backend OCI image, frontend bundle)
  - deploy    # ship to homelab k3s

variables:
  # --- registry: TODO(you) — GitLab built-in ($CI_REGISTRY) or ghcr.io ---
  IMAGE_REGISTRY: "$CI_REGISTRY"
  BACKEND_IMAGE: "$CI_REGISTRY/$CI_PROJECT_PATH/backend"
  FRONTEND_IMAGE: "$CI_REGISTRY/$CI_PROJECT_PATH/frontend"
  # Mirror keeps full history; SHA is the deploy contract with GitHub CI.
  IMAGE_TAG: "$CI_COMMIT_SHORT_SHA"

# -----------------------------------------------------------------------------
# BACKEND — Go 1.23, two services: cmd/command + cmd/query
# -----------------------------------------------------------------------------
package:backend:
  stage: package
  tags: [homelab]          # TODO(you): your runner's actual tag
  image: golang:1.23       # TODO(you): swap if build strategy needs docker/buildah/kaniko image
  script:
    # TODO(you): build + push OCI image(s) for cmd/command and cmd/query.
    # Decision to make: dind vs buildah vs kaniko vs runner docker socket.
    - echo "package:backend — write me"

deploy:backend:
  stage: deploy
  tags: [homelab]
  image: bitnami/kubectl:1.31
  # Devops rule: `terraform apply` + `kubectl delete` human-only.
  # kustomize apply may stay automated — uncomment for a human gate anyway:
  # when: manual
  script:
    # TODO(you): pull kubeconfig from Vault, then apply the reckonna-app base:
    #   kubectl kustomize infra/k8s/reckonna-app | kubectl apply -f -
    # Image tag comes from $IMAGE_TAG (or set via kustomize edit set image).
    - echo "deploy:backend — write me"

# -----------------------------------------------------------------------------
# FRONTEND — React Native + Expo (web target consolidated from MUI app)
# -----------------------------------------------------------------------------
package:frontend:
  stage: package
  tags: [homelab]
  image: node:20
  script:
    # TODO(you): pick artifact strategy — `npx expo export --platform web`
    # (static bundle → nginx image or object storage) vs EAS build (cloud,
    # needs EXPO_TOKEN from Vault). npm ci first either way.
    - echo "package:frontend — write me"

deploy:frontend:
  stage: deploy
  tags: [homelab]
  image: bitnami/kubectl:1.31
  # when: manual           # optional human gate, same rule as backend
  script:
    # TODO(you): apply infra/k8s base for the frontend (create one if absent),
    # or upload exported bundle to its destination.
    - echo "deploy:frontend — write me"
```
