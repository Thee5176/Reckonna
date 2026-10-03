# Design

## Context

The hybrid split (CI on GitHub Actions, CD on GitLab, see
`docs/hybrid-ci-github-cd-gitlab.md`) validated every commit twice and
depended on homelab services that are now gone — the self-hosted runner,
Vault, and SonarQube all went down with the homelab. `sonar` + `sonar-gate`
in `ci.yml` are therefore already unrunnable. Decided with the human:
consolidate everything on gitlab.com — CI migrates from `ci.yml`, CD + SAST
stay as previously designed, the push mirror becomes a gitlab.com pull mirror,
and the development flow moves to GitLab MRs. The Vault-deviation (CI/CD
variables) and GitLab container registry decisions carry over unchanged.

## Goals / Non-Goals

**Goals:**

- Single GitLab pipeline: the 7 CI jobs from `ci.yml` migrate (backend,
  frontend, e2e, terraform, infra-manifests, gitleaks→Secret Detection, sonar
  dropped) and run on main + MR pipelines before `package`.
- Native SAST in `test` (advisory) on main + MR pipelines; findings reach the
  Security Dashboard.
- Zero Vault dependency in the GitLab pipeline; all secrets via masked +
  protected CI/CD variables.
- Backend/frontend artifacts land in the GitLab container registry — no
  external registry.
- `.github/workflows/ci.yml` deleted; gitlab.com pull mirror replaces the push
  job; `GITLAB_MIRROR_TOKEN` secret eliminated.
- MR-based flow: protected `main`, green pipeline = merge condition.
- Pipeline scoping + caching keep CI inside the 400-minute free pool.

**Non-Goals:**

- No SAST hardening to blocking (later, human-approved step).
- No quality-gate replacement (SonarCloud / re-hosted SonarQube) in this
  change.
- Backend k8s manifests, web-harness rework, and the `package:*` build-strategy
  TODOs stay open.
- The GitHub repo itself is not deleted — only its workflows.

## Decisions

### D1 — SAST via the `Security/SAST.gitlab-ci.yml` template, new `test` stage

Include the official template, which provides the `sast` job with the right
analyzer set, `allow_failure` semantics, and `artifacts: reports: sast`
wiring. Stages become `test → package → deploy`. Override the job only where
the template cannot know local facts: `tags: [homelab]`.

- Alternative: hand-rolled `semgrep` job — rejected; more config to maintain,
  loses the report-contract and dashboard integration for free.
- Alternative: IaC-only template (`Security/SAST-IaC`) — rejected; covers
  kubesec only, misses Go/TS analyzers. The main SAST template already runs
  kubesec against k8s manifests.

### D2 — SAST is advisory (`allow_failure: true`) — human decision

SAST findings are surfaced, not gating. Rationale: with `sonar-gate` gone
(D8) there is no blocking static-analysis gate anywhere; a false positive
must not block delivery. Advisory mode until hardening is explicitly approved
(task 6.1).

- Alternative (considered, rejected by human): `allow_failure: false` for
  parity with the old `sonar-gate`. Listed as a future hardening step once
  false-positive rate is observed.

### D3 — Secrets: masked+protected CI/CD variables replace Vault

- `KUBECONFIG`: **file-type** protected+masked variable — the deploy job
  consumes `$KUBECONFIG` (a path), no base64 dance in the script.
- Registry authentication needs **no** CI/CD variable: the GitLab container
  registry accepts `CI_JOB_TOKEN` out of the box for the project's own images,
  so the Vault-free surface is just one variable total.
- Variables live in GitLab project settings, configured manually (human-gated
  task). `.gitlab-ci.yml` references names only.
- The deviation from the Vault-only rule is recorded in
  `docs/gitlab-pipeline.md`, not by editing `.claude/rules/secrets-vault.md`.

- Alternative: Kubernetes secrets fetched by jobs — rejected; per-job kubectl
  plumbing and the kubeconfig bootstrap problem.
- Alternative: GitLab External Secrets / Vault agent — rejected; both depend
  on Vault, which is down.
- Alternative: wait for Vault recovery — rejected; blocks delivery now.
  Variable names are chosen so a later Vault-backed switchover is a drop-in.

### D4 — Registry: GitLab-native container registry (`$CI_REGISTRY`) — human decision

Images publish to the GitLab project's built-in container registry
(`registry.gitlab.com/thee51761/reckonna/...`) and authenticate with
`CI_JOB_TOKEN` — zero external credentials, zero additional variables, works
whether Vault is up or down.

- Alternative: ghcr.io via `REGISTRY_USER`/`REGISTRY_PASSWORD` — rejected for
  the extra PAT rotation burden; keeping a GitHub PAT alive in GitLab vars
  buys nothing now that GitHub CI is leaving too.

### D5 — Pipeline host: gitlab.com (`gl` remote) — human decision

Pipeline runs on gitlab.com (project `thee51761/Reckonna`) — now the sole
CI/CD host. The `homelab`-tagged runner is registered there and must reach
the internet for analyzer pulls (registry.gitlab.com). Self-hosted
`gitlab.thee5176.com` stays unused by this change.

### D6 — SAST needs no secrets; analyzer images come from GitLab's registry

SAST analyzers are public images and require no credentials. Runner executor
must support container jobs (docker or kubernetes executor).

### D7 — CI job migration map (GitHub `ci.yml` → GitLab)

| GitHub `ci.yml` job | GitLab implementation | Delta |
| --- | --- | --- |
| backend (build, `go test -race`, lint) | one job, `image: golang:1.23`; lint via `golangci/golangci-lint:v1.62.2` image directly | marketplace action → direct container |
| frontend (Jest) | `image: node:20`, `npm ci && npm test` | as-is |
| e2e (testcontainers) | `image: golang:1.23` + `services: [docker:24-dind]` + `DOCKER_HOST` | runs via dind service |
| terraform (validate ×2 modules) | `image: hashicorp/terraform:1.9.8`, keep `-backend=false` | as-is |
| infra-manifests (kubeconform, shellcheck, IT) | `image: golang:1.23` + `go install kubeconform` + apt | validation only — works without a deploy target |
| gitleaks | nothing to add — native Secret Detection replaces it | planned in the August comparison |
| sonar + sonar-gate | removed — self-hosted runner, Vault, and SonarQube are gone with the homelab; unrunnable | quality-gate gap noted |

### D8 — sonar + sonar-gate removed; quality-gate gap recorded

The jobs are already unrunnable (self-hosted runner, Vault, SonarQube all
gone). Removing them leaves the pipeline with no PR-time static-analysis gate;
SAST is advisory. The gap is recorded in `docs/gitlab-pipeline.md`; options
(SonarCloud, re-hosted SonarQube) are a separate decision, out of scope.

### D9 — gitlab.com pull mirror replaces the GitHub push job

A gitlab.com pull mirror (Free) fetches GitHub `main`; the planned
`mirror-gitlab` job and `GITLAB_MIRROR_TOKEN` secret are never created.
Risk: Free-tier pull mirroring of private repos may require a paid tier —
verify before cutover; fallback is a scheduled pipeline job with a read-only
token.

### D10 — Development flow moves to GitLab MRs

Branch protection on `main` (gitlab.com) with green pipeline as the merge
condition. GitHub PR checks disappear with the workflow; MRs become the
review + gate surface.

### D11 — CI minutes budget: scoping + caching are operating conditions

~15–20 min/pipeline × ~20/month ≈ 300 min; SAST + package push toward the
400-minute free pool. `workflow.rules` (main + MR only) and Go/npm caching
are therefore hard requirements, not optimizations. Exhausting the pool stops
CI entirely — a single point of failure accepted and recorded as a risk.

## Risks / Trade-offs

- [400-minute pool exhaustion stops all CI] → scoping + caching (D11); monitor
  usage; escalation path is paid CI minutes.
- [Pull mirror tier limits on private repos] → verify before cutover; fallback
  scheduled job + read-only token (D9).
- [No static-analysis gate anywhere in the pipeline] → quality-gate gap
  recorded (D8); SAST advisory; hardening task 6.1 exists.
- [e2e dind needs docker executor on the homelab runner] → validate in the
  first MR pipeline (task 5.1); fallback is a dedicated e2e runner config.
- [Runner cannot pull analyzer images from the homelab network position] →
  validate in the first pipeline run (task 5.2); fallback is
  `SAST_ANALYZER_IMAGE_PREFIX`/`SECURE_ANALYZERS_PREFIX` pointed at a
  reachable mirror.
- [Two secret sources of truth while Vault is down (Vault files vs GitLab
  vars)] → single documented source for GitLab jobs (the vars); re-migration
  to Vault is an explicit future change, not drift.
- [CI_JOB_TOKEN is job-scoped and short-lived; a later job needing
  cross-project pulls would need a deploy token] → out of scope today (single
  project pulls its own images); documented as the escalation path.
- [SAST false positives flood the dashboard post-merge] → advisory mode means
  no delivery impact; hardening task exists to make it blocking once tuned.
- [Secret value leakage in job logs if a variable is not masked] → all
  variables created masked; task 2.2 verifies masking in the settings UI/API
  before first run.

## Migration Plan

1. **Expand**: rewrite `.gitlab-ci.yml` — CI jobs (D7), Secret Detection
   include, `workflow.rules` main + MR, caching; keep SAST, registry, and
   `$KUBECONFIG` plumbing.
2. **Configure**: gitlab.com settings — pull mirror, branch protection,
   `KUBECONFIG` file variable (human-gated).
3. **Remove**: delete `.github/workflows/ci.yml`.
4. **Document**: rewrite `docs/hybrid-ci-github-cd-gitlab.md` →
   `docs/gitlab-pipeline.md`; update README + dev-workflow docs.
5. **Verify**: MR pipeline (CI + SAST + SD), main pipeline
   (`test → package → deploy`, registry image, dashboard), GitHub workflow
   gone.
6. **Rollback**: restore `ci.yml` and revert `.gitlab-ci.yml` to the
   pre-change hybrid state (single-file revert each). No data loss — pipeline
   config only. Variables can stay dormant in settings.

## Open Questions

None that change specs, approach, or task breakdown.
