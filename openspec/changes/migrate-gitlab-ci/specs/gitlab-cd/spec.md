# Spec Delta

## Purpose

Defines the GitLab-side delivery pipeline: native SAST runs on every main-branch pipeline before packaging, findings land in the security dashboard, and the pipeline consumes secrets exclusively from GitLab CI/CD variables — never Vault.

## ADDED Requirements

### Requirement: SAST runs on every main pipeline before package

The GitLab pipeline SHALL run GitLab native SAST in a `test` stage that executes before the `package` stage on every pipeline for the `main` branch. SAST SHALL analyze the Go backend, the TypeScript/JavaScript frontend, and Kubernetes manifests. A SAST analyzer failure SHALL NOT block the package or deploy stages (advisory mode) until hardening is explicitly approved.

#### Scenario: Main mirror push triggers SAST

- **WHEN** a push to `main` (via the GitHub mirror or directly) starts a pipeline
- **THEN** the `test` stage runs the SAST job before any `package` job
- **AND** the SAST job executes analyzers covering Go, TypeScript/JavaScript, and Kubernetes manifests

#### Scenario: SAST analyzer failure does not block delivery

- **WHEN** a SAST analyzer exits non-zero
- **THEN** the SAST job is marked failed with allow-failure semantics
- **AND** the `package` and `deploy` stages still run

### Requirement: SAST findings are reported to the security dashboard

The pipeline SHALL upload SAST results as a `sast` report artifact so findings for the default branch SHALL appear in GitLab's Security Dashboard after the pipeline completes.

#### Scenario: Findings visible after pipeline completion

- **WHEN** the SAST job completes and its pipeline finishes
- **THEN** the pipeline stores a `sast` report artifact
- **AND** the Security Dashboard shows findings attributed to the default branch

### Requirement: Pipeline secrets come only from GitLab CI/CD variables

The GitLab pipeline SHALL NOT depend on Vault for any secret. All secrets consumed by GitLab jobs (kubeconfig, registry credentials) SHALL be delivered through masked, protected GitLab CI/CD variables — file-type variables for file-shaped secrets — and SHALL be referenced by name only, never inlined in repository files.

#### Scenario: Pipeline runs with Vault unreachable

- **WHEN** the GitLab pipeline executes while Vault is down
- **THEN** every job completes its secret provisioning using CI/CD variables alone
- **AND** no job step attempts to contact Vault

#### Scenario: Repository contains no secret values

- **WHEN** `.gitlab-ci.yml` and the repository are scanned for credentials
- **THEN** no secret values are present — only variable references such as `$KUBECONFIG` or `$CI_REGISTRY_PASSWORD`

### Requirement: Packaged artifacts are published to the GitLab container registry

The pipeline SHALL publish backend container images to the GitLab project's built-in container registry (GitLab-native, authenticated with `CI_JOB_TOKEN`) — not to an external registry. Deploy jobs SHALL reference images from that same registry.

#### Scenario: Package job publishes to the GitLab registry

- **WHEN** a `package` job completes on a `main` pipeline
- **THEN** the backend image exists at `registry.gitlab.com/thee51761/reckonna/<image>:<commit-sha>`
- **AND** no external registry credentials were required to push

#### Scenario: Deploy references the GitLab registry image

- **WHEN** a deploy job runs
- **THEN** the image it deploys is pulled from the GitLab project container registry

### Requirement: GitHub CI is untouched by this change

The change SHALL NOT modify `.github/workflows/ci.yml`; the GitHub `sonar` and `sonar-gate` jobs SHALL continue to run unchanged.

#### Scenario: GitHub workflow diff is empty

- **WHEN** the change's commits are compared against their base
- **THEN** no file under `.github/workflows/` differs
