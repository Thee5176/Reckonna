# Design

## Context

The hybrid split (see `docs/hybrid-ci-github-cd-gitlab.md`): GitHub Actions is the CI quality gate; GitLab receives a push-mirror of `main` after CI is green and runs CD only (stages `package` → `deploy`). Today the GitLab pipeline (`.gitlab-ci.yml`, scaffold with 4 open TODOs) assumes Vault supplies kubeconfig and registry credentials at job runtime. Vault is currently down. GitHub keeps its SonarQube PR-time analysis — this change does not touch `ci.yml`; it adds GitLab-native SAST to the GitLab pipeline and switches GitLab-side secret sourcing to CI/CD variables.

Decided with the human: SAST is **advisory**, the pipeline runs on **gitlab.com** (the `gl` remote), and package jobs publish to the **GitLab-native container registry** (`$CI_REGISTRY`) — superseding an earlier ghcr.io choice.

## Goals / Non-Goals

**Goals:**

- GitLab pipeline gains a `test` stage with native SAST (Go, TS/JS, k8s manifests) running on every `main` pipeline before `package`.
- SAST findings reach GitLab's Security Dashboard via `sast` report artifacts; SAST never blocks delivery.
- Zero Vault dependency in the GitLab pipeline; all secrets via masked+protected CI/CD variables.
- Backend/frontend artifacts land in the GitLab container registry — no external registry.
- GitHub workflows byte-for-byte untouched.

**Non-Goals:**

- No GitHub-side edits (no `mirror-gitlab` job, no `sonar` retirement).
- No amendment to `.claude/rules/secrets-vault.md` (deviation documented in the hybrid doc only).
- Not filling the `package:*` build-strategy TODOs beyond registry plumbing (dind vs buildah vs kaniko stays open in the scaffold).
- Not making SAST a blocking gate (hardening is a later, human-approved step).

## Decisions

### D1 — SAST via the `Security/SAST.gitlab-ci.yml` template, new `test` stage

Include the official template, which provides the `sast` job with the right analyzer set, `allow_failure` semantics, and `artifacts: reports: sast` wiring. Stages become `test → package → deploy`. Override the job only where the template cannot know local facts: `tags: [homelab]`.

- Alternative: hand-rolled `semgrep` job — rejected; more config to maintain, loses the report-contract and dashboard integration for free.
- Alternative: IaC-only template (`Security/SAST-IaC`) — rejected; covers kubesec only, misses Go/TS analyzers. The main SAST template already runs kubesec against k8s manifests.

### D2 — SAST is advisory (`allow_failure: true`) — human decision

SAST findings are surfaced, not gating. Rationale: the GitLab pipeline is post-merge delivery; a false positive must not block delivery of code that already passed GitHub CI. GitHub's `sonar-gate` keeps its PR-time gate role.

- Alternative (considered, rejected by human): `allow_failure: false` for parity with `sonar-gate`. Listed as a future hardening step once false-positive rate is observed.

### D3 — Secrets: masked+protected CI/CD variables replace Vault

- `KUBECONFIG`: **file-type** protected+masked variable — the deploy job consumes `$KUBECONFIG` (a path), no base64 dance in the script.
- Registry authentication needs **no** CI/CD variable: the GitLab container registry accepts `CI_JOB_TOKEN` out of the box for the project's own images, so the Vault-free surface is just one variable total.
- Variables live in GitLab project settings, configured manually (human-gated task). `.gitlab-ci.yml` references names only.
- The deviation from the Vault-only rule is recorded in `docs/hybrid-ci-github-cd-gitlab.md`, not by editing `.claude/rules/secrets-vault.md`.

- Alternative: Kubernetes secrets fetched by jobs — rejected; per-job kubectl plumbing and the kubeconfig bootstrap problem.
- Alternative: GitLab External Secrets / Vault agent — rejected; both depend on Vault, which is down.
- Alternative: wait for Vault recovery — rejected; blocks delivery now. Variable names are chosen so a later Vault-backed switchover is a drop-in.

### D4 — Registry: GitLab-native container registry (`$CI_REGISTRY`) — human decision

Images publish to the GitLab project's built-in container registry (`registry.gitlab.com/thee51761/reckonna/...`) and authenticate with `CI_JOB_TOKEN` — zero external credentials, zero additional variables, works whether Vault is up or down. This supersedes the earlier ghcr.io choice: keeping a GitHub PAT alive in GitLab vars for registry pushes was an extra secret-rotation surface that buys nothing while Vault is down.

- Alternative (earlier human choice, now reversed): ghcr.io via `REGISTRY_USER`/`REGISTRY_PASSWORD` — rejected for the extra PAT rotation burden.

### D5 — Pipeline host: gitlab.com (`gl` remote) — human decision

Pipeline runs on gitlab.com (project `thee51761/Reckonna`). The `homelab`-tagged runner is registered there and must reach the internet for analyzer pulls (registry.gitlab.com) and ghcr.io pushes. Self-hosted `gitlab.thee5176.com` stays unused by this change.

### D6 — SAST needs no secrets; analyzer images come from GitLab's registry

SAST analyzers are public images and require no credentials. Runner executor must support container jobs (docker or kubernetes executor).

## Risks / Trade-offs

- [Runner cannot pull analyzer images or reach ghcr.io from the homelab network position] → Validate in the first pipeline run (task 3.1); fallback is `SAST_ANALYZER_IMAGE_PREFIX`/`SECURE_ANALYZERS_PREFIX` pointed at a reachable mirror.
- [Two secret sources of truth while Vault is down (Vault files vs GitLab vars)] → Single documented source for GitLab jobs (the vars); hybrid doc updated in task 2.3; re-migration to Vault is an explicit future change, not drift.
- [CI_JOB_TOKEN is job-scoped and short-lived; a later job needing cross-project pulls would need a deploy token] → Out of scope today (single project pulls its own images); documented in the hybrid doc as the escalation path.
- [SAST false positives flood the dashboard post-merge] → Advisory mode means no delivery impact; hardening task exists to make it blocking once tuned.
- [Secret value leakage in job logs if a variable is not masked] → All variables created masked; task 2.2 verifies masking in the settings UI/API before first run.

## Migration Plan

1. **Expand**: edit `.gitlab-ci.yml` (additive: new stage + include + overrides; replace Vault references with variable references; point registry at `$CI_REGISTRY`).
2. **Configure**: create one CI/CD variable in the gitlab.com project settings (manual, human-gated): `KUBECONFIG` (file).
3. **Verify**: next `main` pipeline runs SAST before `package`, report artifacts upload, dashboard populates; GitHub diff for `.github/` is empty; the GitLab container registry receives the image.
4. **Rollback**: revert `.gitlab-ci.yml` (single-file revert; remove include + stage). No data loss — pipeline config only. Variables can stay dormant in settings.

## Open Questions

None that change specs, approach, or task breakdown.
