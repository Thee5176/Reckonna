# Tasks

## 1. GitLab SAST configuration

- [x] 1.1 Add `test` stage and `Security/SAST.gitlab-ci.yml` include to `.gitlab-ci.yml`, with `sast` job override (`tags: [homelab]`, `allow_failure: true`) — verify the file parses with `yamllint .gitlab-ci.yml` and stages read `test → package → deploy`
- [x] 1.2 Extend `workflow.rules` to admit `main` pushes AND merge-request pipelines (no tags, schedules, or duplicate pipelines) — verify with `yamllint .gitlab-ci.yml` and that the rules admit exactly those two event kinds
- [ ] 1.3 Confirm default analyzer set covers Go, TypeScript/JavaScript, and k8s manifests (semgrep + kubesec) — verify analyzer list appears in a pipeline's SAST job log (with task 5.2)

## 2. Secrets without Vault

- [x] 2.1 Replace every Vault reference in `.gitlab-ci.yml` with CI/CD variable references: `$KUBECONFIG` (file-type) for deploy jobs, `$CI_REGISTRY` image tags with `CI_JOB_TOKEN` auth for package jobs — verify `grep -in vault .gitlab-ci.yml` returns only the header doc-note and no functional Vault step remains
- [ ] 2.2 HUMAN: create the masked + protected CI/CD variable `KUBECONFIG` (file-type) in the gitlab.com project settings — verify it shows masked in Settings → CI/CD → Variables and no registry credential variable exists
- [x] 2.3 Rewrite `docs/hybrid-ci-github-cd-gitlab.md` as `docs/gitlab-pipeline.md`: SAST stage, migrated CI jobs, Vault-deviation note (GitLab CI/CD vars are the secret source while Vault is down), GitLab container registry with `CI_JOB_TOKEN`, MR flow, CI-minutes budget — verify the new doc exists and matches the repo

## 3. CI migration from GitHub

- [ ] 3.1 Add `backend` job (`image: golang:1.23`; `go test -race ./...`; lint via `golangci/golangci-lint:v1.62.2` image directly) — verify `yamllint .gitlab-ci.yml` and the job is present in the pipeline
- [ ] 3.2 Add `frontend` job (`image: node:20`; `npm ci && npm test`) — verify `yamllint .gitlab-ci.yml`
- [ ] 3.3 Add `e2e` job (`image: golang:1.23` + `services: [docker:24-dind]` + `DOCKER_HOST` env) — verify `yamllint .gitlab-ci.yml`
- [ ] 3.4 Add `terraform` validate job (`image: hashicorp/terraform:1.9.8`; `-backend=false`; ×2 modules) — verify `yamllint .gitlab-ci.yml`
- [ ] 3.5 Add `infra-manifests` job (`image: golang:1.23`; `go install kubeconform` + shellcheck + IT) — verify `yamllint .gitlab-ci.yml`
- [ ] 3.6 Add `Security/Secret-Detection.gitlab-ci.yml` include (replaces the retired gitleaks job) — verify the template include is present
- [ ] 3.7 Add Go module + npm cache configuration — verify cache keys present in `.gitlab-ci.yml`
- [ ] 3.8 Delete `.github/workflows/ci.yml` — verify `git rm` and no workflow file under `.github/` remains

## 4. GitLab project setup (human)

- [ ] 4.1 HUMAN: configure the gitlab.com pull mirror (GitHub → gitlab.com) — verify the mirror syncs `main`
- [ ] 4.2 HUMAN: enable branch protection on `main` with green pipeline as the merge condition — verify a red-pipeline MR cannot merge

## 5. Validation

- [ ] 5.1 Run a merge-request pipeline on gitlab.com — verify CI jobs + SAST + Secret Detection run and a `sast` report artifact is stored
- [ ] 5.2 Run a `main` pipeline on gitlab.com (mirror or direct push) — verify stage order `test → package → deploy`, the packaged image exists in the GitLab container registry at `registry.gitlab.com/thee51761/reckonna/<image>:<commit-sha>`, and the Security Dashboard shows default-branch findings
- [ ] 5.3 Confirm GitHub CI is gone — verify no `.github/workflows/` remains in the repo
- [ ] 5.4 Update `README.md` + dev-workflow docs for the MR-based flow — verify they describe GitLab MRs, not GitHub PR checks

## 6. Follow-up (deferred, human-gated)

- [ ] 6.1 Hardening: flip SAST to blocking (`allow_failure: false`) after false-positive tuning — requires explicit human approval; do not fold into the initial delivery
- [ ] 6.2 Quality-gate decision: SonarCloud vs re-hosted SonarQube vs accept the gap — separate change, human decision
