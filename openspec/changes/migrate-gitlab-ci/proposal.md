# Proposal

## Why

The hybrid CI/CD split puts all CI (tests, lint, SonarQube, gitleaks) on GitHub Actions and CD (package + deploy) on GitLab. The GitLab pipeline has no static-analysis coverage of its own, and every GitLab-side secret need (kubeconfig, registry creds) was designed around Vault — which is currently down. GitLab's native SAST needs zero secrets and runs where the deploy happens, so it takes over the analysis role for the GitLab side, while GitLab CI/CD variables (masked + protected) provide a Vault-free secret path.

## What Changes

- **`.gitlab-ci.yml`**: add a `test` stage with GitLab native SAST (via the `Security/SAST.gitlab-ci.yml` template) that runs on every `main` mirror push before `package`. SAST findings are uploaded as job artifacts (`reports: sast`) and surface in GitLab's security dashboard. Analyzer coverage targets Go backend, TypeScript/JS frontend, and k8s manifests (kubesec).
- **`.gitlab-ci.yml`**: replace all Vault-based secret sourcing with GitLab CI/CD variables — masked + protected project variables, file-type variable for kubeconfig. Registry auth needs no variables at all: package jobs publish to the GitLab-native container registry (`$CI_REGISTRY`) with `CI_JOB_TOKEN`. No secrets in the repo file. The deviation from the Vault-only rule is recorded in `docs/hybrid-ci-github-cd-gitlab.md`; `.claude/rules/secrets-vault.md` itself is not amended.
- **`.github/workflows/ci.yml`**: unchanged. The `sonar` + `sonar-gate` jobs stay as-is (PR-time analysis on GitHub remains SonarQube).
- **`docs/hybrid-ci-github-cd-gitlab.md`**: update to describe the SAST stage and the Vault-deviation decision.

Out of scope: adding the `mirror-gitlab` job to `ci.yml` (GitHub-side edit, separate change), retiring GitHub-side SonarQube, and the remaining `package:*` build-strategy TODOs.

## Capabilities

### New Capabilities

- `gitlab-cd`: GitLab-side pipeline behavior — native SAST on the `main` mirror, secret handling without Vault, and the artifact/report contracts those jobs rely on.

### Modified Capabilities

None — no existing capabilities.

## Impact

- **Files**: `.gitlab-ci.yml` (edit), `docs/hybrid-ci-github-cd-gitlab.md` (edit).
- **Systems**: GitLab project settings — CI/CD variables (masked/protected) configured manually; `homelab`-tagged runner must be able to run SAST analyzer container images (docker or kubernetes executor).
- **No impact**: Go/frontend code, Terraform, GitHub Actions workflows, k8s manifests.
