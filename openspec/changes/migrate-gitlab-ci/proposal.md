# Proposal

## Why

The hybrid split (CI on GitHub Actions, CD on GitLab) validates every commit
twice — once on GitHub, once mirrored to GitLab — for the same artifact. It
also carries two platform taxonomies of secrets, runners, and rules. The
homelab decommission removed the self-hosted runner, Vault, and SonarQube that
GitHub CI depended on (`sonar`, `sonar-gate` are already unrunnable), so the
GitHub side no longer delivers its original gate value. Consolidation: retire
`.github/workflows/ci.yml` entirely, run CI + CD + SAST in one GitLab pipeline,
and replace the GitHub→GitLab push mirror with a gitlab.com pull mirror (Free).

## What Changes

- **`.gitlab-ci.yml`**: grows from CD-only to the single pipeline — the 7 CI
  jobs from `ci.yml` migrate per the job map in design D7 (backend, frontend,
  e2e, terraform, infra-manifests, gitleaks→native Secret Detection, sonar
  dropped). Stages become `test → package → deploy`. `workflow.rules` admits
  `main` pushes AND merge-request pipelines. Caching (Go modules, npm) becomes
  a hard requirement under the 400-minute free CI pool.
- **`.github/workflows/ci.yml`**: deleted. The `mirror-gitlab` push job never
  lands; gitlab.com pull mirror replaces it and the `GITLAB_MIRROR_TOKEN`
  secret disappears.
- **GitLab project settings**: pull mirror (GitHub → gitlab.com), branch
  protection on `main` with green pipeline as merge condition, CI/CD variables
  unchanged (`KUBECONFIG` file-type only).
- **`docs/hybrid-ci-github-cd-gitlab.md`**: rewritten and renamed to
  `docs/gitlab-pipeline.md` — no longer a hybrid split; documents the MR
  workflow, the sonar quality-gate gap, and the CI-minutes budget.
- **`README.md` / dev-workflow docs**: updated for the MR-based flow.

Quality-gate note: SonarQube analysis leaves the pipeline with no PR-time
static-analysis gate. Recorded as a known gap; options (SonarCloud, re-hosted
SonarQube) are out of scope.

Out of scope: SAST hardening to blocking, backend k8s manifests, web-harness
rework, re-adding any external quality gate.

## Capabilities

### New Capabilities

- `gitlab-pipeline`: the single GitLab pipeline — CI jobs migrated from
  GitHub, native SAST, Vault-free secrets, GitLab container registry, and the
  MR-based development flow.

## Impact

- **Files**: `.gitlab-ci.yml` (rewrite), `.github/workflows/ci.yml` (delete),
  `docs/hybrid-ci-github-cd-gitlab.md` → `docs/gitlab-pipeline.md` (rewrite +
  rename), `README.md` (edit), dev-workflow docs (edit).
- **Systems**: gitlab.com project settings — pull mirror, branch protection,
  CI/CD variables (`KUBECONFIG`). GitHub repo keeps existing contents;
  workflows removed.
- **CI budget**: ~15–20 min/pipeline × ~20/month ≈ 300 min; SAST + package on
  top approaches the 400-minute free pool — `workflow.rules` scoping and
  caching are conditions for operation, not optimizations.
