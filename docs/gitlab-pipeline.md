# GitLab Pipeline — CI + CD + SAST on one platform

Reckonna's entire pipeline lives on GitLab (gitlab.com): CI migrated from
GitHub Actions, CD (package + deploy to the homelab k3s cluster), and native
SAST + Secret Detection. GitHub Actions is retired; development moves to
GitLab merge requests.

```
GitHub repo (source of truth; no workflows)
  │
  └─ gitlab.com pull mirror fetches main (Free tier)
                          │
               GitLab repo (gitlab.com/thee51761/Reckonna)
                 .gitlab-ci.yml → single pipeline
                 stages: test (CI + SAST + Secret Detection) → package → deploy
                 runner: homelab (inside cluster network)
                 targets: k3s homelab + GitLab container registry
```

**The seam contract:** the mirror preserves history, so GitLab's
`$CI_COMMIT_SHORT_SHA` equals the GitHub commit SHA. The image tag is
traceable with zero extra plumbing.

---

## Why one platform

The hybrid split validated every commit twice — GitHub CI first, then the
mirrored GitLab pipeline — for the same artifact, and carried two sets of
secrets, runners, and rules. The homelab decommission removed the self-hosted
runner, Vault, and SonarQube that GitHub CI depended on: `sonar` and
`sonar-gate` were already unrunnable. Consolidation removes the double
validation, the `mirror-gitlab` push job, and the `GITLAB_MIRROR_TOKEN`
secret; gitlab.com's pull mirror replaces the push job.

## Pipeline shape

| Stage | Jobs |
|-------|------|
| `test` | CI jobs migrated from GitHub (backend, frontend, e2e, terraform, infra-manifests), SAST, Secret Detection |
| `package` | `package:backend` (buildah → command + query images), `package:frontend-export` + `package:frontend-image` (expo export → nginx image) |
| `deploy` | `deploy:backend`, `deploy:frontend` — kubeconfig from the `KUBECONFIG` file variable |

CI job migration map (GitHub `ci.yml` → GitLab):

| GitHub job | GitLab implementation | Delta |
| --- | --- | --- |
| backend (build, `go test -race`, lint) | one job, `image: golang:1.23`; lint via `golangci/golangci-lint:v1.62.2` image directly | marketplace action → direct container |
| frontend (Jest) | `image: node:20`, `npm ci && npm test` | as-is |
| e2e (testcontainers) | `image: golang:1.23` + `services: [docker:24-dind]` + `DOCKER_HOST` | runs via dind service |
| terraform (validate ×2 modules) | `image: hashicorp/terraform:1.9.8`, `-backend=false` | as-is |
| infra-manifests (kubeconform, shellcheck, IT) | `image: golang:1.23` + `go install kubeconform` + apt | validation only — no deploy target needed |
| gitleaks | native Secret Detection (`Security/Secret-Detection.gitlab-ci.yml`) | planned since the August comparison |
| sonar + sonar-gate | removed — self-hosted runner, Vault, and SonarQube are gone with the homelab | quality-gate gap, see below |

`workflow.rules` admits exactly two pipeline kinds: `main` pushes (the pull
mirror lands here) and merge-request pipelines. Tags, schedules, and stray
branch pushes create nothing — also the budget lever, see below.

## SAST and Secret Detection

Native SAST (`Security/SAST.gitlab-ci.yml` include) runs in `test` on every
main and MR pipeline: semgrep covers Go + TypeScript/JavaScript, kubesec
covers k8s manifests. Findings upload as a `sast` report artifact and surface
in the Security Dashboard for the default branch. SAST is **advisory**
(`allow_failure: true`): a red analyzer never blocks package or deploy.
Hardening to blocking is a separate, human-approved step.

Secret Detection (`Security/Secret-Detection.gitlab-ci.yml`) replaces the
GitHub gitleaks job — same gitleaks analyzer, native report + dashboard
integration.

## Secrets without Vault (recorded deviation)

Repo-wide rule `.claude/rules/secrets-vault.md` says Vault is the only secret
source. **Vault is down**; the GitLab side deviates, recorded here (the rule
file itself is not amended):

- GitLab CI/CD variables are the GitLab-side secret source: masked +
  protected, created in project settings.
- Exactly **one** variable exists: `KUBECONFIG` (file type). File-type
  variables are written to a temp file by GitLab; `$KUBECONFIG` expands to
  that path — deploy jobs run `export KUBECONFIG="$KUBECONFIG"`, no base64,
  no fetch step.
- Registry auth needs **no** variable: `CI_JOB_TOKEN` is accepted by the
  project's own container registry out of the box.
- Re-migration to Vault later is a drop-in (swap variable references), an
  explicit future change, not drift.

## Container registry

Images publish to the GitLab-native container registry
(`$CI_REGISTRY` = `registry.gitlab.com/thee51761/reckonna/...`), tagged with
`$CI_COMMIT_SHORT_SHA`, authenticated with `CI_JOB_TOKEN` — zero external
credentials. Escalation path: `CI_JOB_TOKEN` is job-scoped and short-lived; a
future job pulling across projects would need a deploy token.

## Merge-request flow

Development moves to GitLab MRs. `main` is protected on gitlab.com with a
green pipeline as the merge condition; a red MR pipeline cannot merge.
GitHub PR checks no longer exist. Branch protection / required checks live on
GitLab only now.

## CI-minutes budget

The free tier pools 400 CI minutes/month. Estimate: 15–20 min per pipeline ×
~20/month ≈ 300 min for CI alone; SAST + package push toward the cap.
Therefore `workflow.rules` scoping (main + MR only) and Go/npm caching are
**operating conditions, not optimizations**. Exhausting the pool stops all CI
— a single point of failure; the escalation path is paid CI minutes.

## Quality-gate gap

With sonar removed and SAST advisory, the pipeline has no blocking
static-analysis gate. Recorded gap; options (SonarCloud, re-hosted SonarQube,
accept the gap) are a separate human decision.

## Operations notes

- **GitHub is the source of truth.** No commits land directly on gitlab.com;
  the pull mirror carries `main` down.
- **Pull mirror tier check:** Free-tier pull mirroring of private repos may
  require a paid tier — verify before cutover; fallback is a scheduled
  pipeline job with a read-only token.
- **Deploy gaps:** backend Deployment/Service manifests don't exist under
  `infra/k8s/` yet, and the web-harness mounts configmap content — both
  deploy jobs wait on those (TODO comments in `.gitlab-ci.yml`).
- **Runner single point of failure:** the homelab runner down = no CI and no
  deploys.
