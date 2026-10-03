# Tasks

## 1. GitLab SAST configuration

- [ ] 1.1 Add `test` stage and `Security/SAST.gitlab-ci.yml` include to `.gitlab-ci.yml`, with `sast` job override (`tags: [homelab]`, `allow_failure: true`) — verify the file parses with `yamllint .gitlab-ci.yml` and stages read `test → package → deploy`
- [ ] 1.2 Keep `workflow.rules` scoped to `main` pushes only (no pipeline on feature branches or tags) — verify the rules block is unchanged by `git diff`
- [ ] 1.3 Confirm default analyzer set covers Go, TypeScript/JavaScript, and k8s manifests (semgrep + kubesec) — verify analyzer list appears in the first pipeline's SAST job log (with task 3.1)

## 2. Secrets without Vault

- [ ] 2.1 Replace every Vault reference in `.gitlab-ci.yml` with CI/CD variable references: `$KUBECONFIG` (file-type) for deploy jobs, `$CI_REGISTRY` image tags with `CI_JOB_TOKEN` auth for package jobs — verify `grep -in vault .gitlab-ci.yml` returns only the header doc-note and no functional Vault step remains
- [ ] 2.2 HUMAN: create the masked + protected CI/CD variable `KUBECONFIG` (file-type) in the gitlab.com project settings — verify it shows masked in Settings → CI/CD → Variables and no registry credential variable exists
- [ ] 2.3 Update `docs/hybrid-ci-github-cd-gitlab.md`: SAST stage description, Vault-deviation note (GitLab CI/CD vars are the GitLab-side secret source while Vault is down), GitLab container registry choice with `CI_JOB_TOKEN` auth — verify the new sections exist and the doc still matches the file in the repo

## 3. Validation

- [ ] 3.1 Run a `main` pipeline on gitlab.com (mirror or direct push) — verify SAST runs in `test` before `package`, a `sast` report artifact is stored, and the Security Dashboard shows default-branch findings
- [ ] 3.2 Confirm GitHub is untouched — verify `git diff --stat <base>..HEAD -- .github/` is empty
- [ ] 3.3 Confirm the packaged image exists in the GitLab container registry at `registry.gitlab.com/thee51761/reckonna/<image>:<commit-sha>` after the package job — verify via the project's Container Registry page or `docker manifest inspect`

## 4. Follow-up (deferred, human-gated)

- [ ] 4.1 Hardening: flip SAST to blocking (`allow_failure: false`) after false-positive tuning — requires explicit human approval; do not fold into the initial delivery
