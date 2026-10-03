# Spec Delta

## Purpose

Defines the single GitLab pipeline (CI + CD + SAST) after GitHub Actions
retirement: migrated CI jobs validate every MR and main pipeline, native SAST
runs before packaging, findings land in the security dashboard, secrets come
exclusively from GitLab CI/CD variables, and images publish to the GitLab
container registry.

## ADDED Requirements

### Requirement: SAST runs on every main and MR pipeline before package

The GitLab pipeline SHALL run GitLab native SAST in a `test` stage that
executes before the `package` stage on every pipeline for the `main` branch
and every merge-request pipeline. SAST SHALL analyze the Go backend, the
TypeScript/JavaScript frontend, and Kubernetes manifests. A SAST analyzer
failure SHALL NOT block the package or deploy stages (advisory mode) until
hardening is explicitly approved.

#### Scenario: Main push triggers SAST

- **WHEN** a push to `main` (via the pull mirror or directly) starts a pipeline
- **THEN** the `test` stage runs the SAST job before any `package` job
- **AND** the SAST job executes analyzers covering Go, TypeScript/JavaScript, and Kubernetes manifests

#### Scenario: Merge request pipeline triggers SAST

- **WHEN** a merge request pipeline runs
- **THEN** the SAST job executes in the `test` stage

#### Scenario: SAST analyzer failure does not block delivery

- **WHEN** a SAST analyzer exits non-zero
- **THEN** the SAST job is marked failed with allow-failure semantics
- **AND** the `package` and `deploy` stages still run

### Requirement: SAST findings are reported to the security dashboard

The pipeline SHALL upload SAST results as a `sast` report artifact so findings
for the default branch SHALL appear in GitLab's Security Dashboard after the
pipeline completes.

#### Scenario: Findings visible after pipeline completion

- **WHEN** the SAST job completes and its pipeline finishes
- **THEN** the pipeline stores a `sast` report artifact
- **AND** the Security Dashboard shows findings attributed to the default branch

### Requirement: Migrated CI jobs run on every pipeline

The pipeline SHALL run the CI jobs migrated from GitHub — backend (`go test
-race` + golangci-lint), frontend (Jest), e2e (testcontainers), terraform
(validate), infra-manifests (kubeconform + shellcheck + IT) — on every `main`
pipeline and every merge-request pipeline, before the `package` stage.

#### Scenario: Merge request pipeline runs the CI suite

- **WHEN** a merge request pipeline runs
- **THEN** all migrated CI jobs execute and a failure blocks the merge

#### Scenario: Main pipeline runs the CI suite before packaging

- **WHEN** a `main` pipeline runs
- **THEN** all migrated CI jobs pass before any `package` job starts

### Requirement: Secret Detection replaces gitleaks

The pipeline SHALL run GitLab native Secret Detection (which uses the gitleaks
analyzer) in the `test` stage; the standalone GitHub gitleaks job is retired
with `ci.yml`.

#### Scenario: Secret Detection runs on main and MR pipelines

- **WHEN** a `main` or merge-request pipeline runs
- **THEN** the Secret Detection job executes and reports findings to the Security Dashboard

### Requirement: GitHub CI is removed

The change SHALL delete `.github/workflows/ci.yml`; no GitHub Actions workflow
remains in the repository. The GitHub→GitLab mirror SHALL be a gitlab.com pull
mirror (Free tier), not a GitHub-side push job, and the `GITLAB_MIRROR_TOKEN`
secret SHALL no longer be required.

#### Scenario: Repository has no GitHub workflow

- **WHEN** the change's commits are compared against their base
- **THEN** `.github/workflows/ci.yml` is deleted and no workflow file under `.github/` remains

#### Scenario: Mirror needs no GitHub-side secret

- **WHEN** the pull mirror is configured on gitlab.com
- **THEN** `main` reaches gitlab.com without any GitHub Actions job or token

### Requirement: MR flow with protected main

Development SHALL move to GitLab merge requests: `main` SHALL be protected on
gitlab.com with a green pipeline as the merge condition.

#### Scenario: Red pipeline blocks merge

- **WHEN** a merge request's pipeline is not green
- **THEN** the MR cannot merge into `main`

### Requirement: Pipeline secrets come only from GitLab CI/CD variables

The GitLab pipeline SHALL NOT depend on Vault for any secret. All secrets
consumed by GitLab jobs (kubeconfig, registry credentials) SHALL be delivered
through masked, protected GitLab CI/CD variables — file-type variables for
file-shaped secrets — and SHALL be referenced by name only, never inlined in
repository files.

#### Scenario: Pipeline runs with Vault unreachable

- **WHEN** the GitLab pipeline executes while Vault is down
- **THEN** every job completes its secret provisioning using CI/CD variables alone
- **AND** no job step attempts to contact Vault

#### Scenario: Repository contains no secret values

- **WHEN** `.gitlab-ci.yml` and the repository are scanned for credentials
- **THEN** no secret values are present — only variable references such as `$KUBECONFIG` or `$CI_REGISTRY_PASSWORD`

### Requirement: Packaged artifacts are published to the GitLab container registry

The pipeline SHALL publish backend container images to the GitLab project's
built-in container registry (GitLab-native, authenticated with `CI_JOB_TOKEN`)
— not to an external registry. Deploy jobs SHALL reference images from that
same registry.

#### Scenario: Package job publishes to the GitLab registry

- **WHEN** a `package` job completes on a `main` pipeline
- **THEN** the backend image exists at `registry.gitlab.com/thee51761/reckonna/<image>:<commit-sha>`
- **AND** no external registry credentials were required to push

#### Scenario: Deploy references the GitLab registry image

- **WHEN** a deploy job runs
- **THEN** the image it deploys is pulled from the GitLab project container registry

### Requirement: Pipeline scope and caching fit the free CI budget

`workflow.rules` SHALL admit only `main` pushes and merge-request events (no
tags, schedules, or duplicate pipelines), and the pipeline SHALL configure Go
module and npm caching — these are conditions for staying inside the
400-minute free CI pool, not optimizations.

#### Scenario: Rules admit only main and MR

- **WHEN** a tag push or schedule triggers
- **THEN** no pipeline is created

#### Scenario: Cache keys exist

- **WHEN** the pipeline config is inspected
- **THEN** Go module and npm cache entries are configured
