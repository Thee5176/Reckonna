# Plan 00 — Implementation tracker

Mirrors `plans/00-bootstrap-deps-vault.md`. Logs landed steps + verification status.

## Landed steps (commits)

| Step | Commit  | Subject |
|------|---------|---------|
| S0–S9 | `cade037` | `chore: initial project import — V-model scaffold + plan 00 bootstrap` |
| (CI)  | `bf8e2e3` | `docs(ci): staged CI fixes (empty-module guard, gitleaks token, sonar gate) + applied branch-protection note` |
| (CI)  | `5ff60a8` | `ci: guard empty module + gitleaks token + gate sonar` |
| (CI)  | `fba7196` | `ci: grant gitleaks pull-requests:read (fix 403)` |
| (chore) | `86d1080` | `chore: remove empty .gitkeep files from e2e and infra directories` |
| (AT1 fix) | _pending_ | `chore(make): empty-state guards + AT1 smoke-bootstrap script` |

S0–S9 collapsed into one initial-import commit instead of one-per-step. Departs from plan
"one step = one commit"; preserved here for traceability rather than re-splitting.

## Verification status (2026-05-28, executed from `chore/00-bootstrap`)

| ID  | Check                                                | Status | Notes |
|-----|------------------------------------------------------|--------|-------|
| AT1 | `make generate && make test` on fresh-clone proxy    | GREEN  | After Makefile empty-state guards + `scripts/smoke-bootstrap.sh`. |
| AT2 | gitleaks gate fails on planted secret                | GREEN-BY-CONFIG | `.gitleaks.toml` present, CI job wired. Live red/green needs CI run; `gitleaks` not on workstation. |
| AT3 | `terraform validate` on `infra/`                     | GREEN-BY-CONFIG | Files present, vault data sources only. `terraform` not on workstation; CI job wired. |
| IT1 | `go build ./...`                                     | GREEN  | Empty module, warning-only. |
| IT2 | `tests/no-secrets_test.sh`                           | GREEN  | 5/5 cases pass (quoted, unquoted, vault ref, .env path, clean). |
| IT3 | CI workflow has all 6 gates                          | GREEN  | backend, frontend, e2e, terraform, gitleaks, sonar — present in `.github/workflows/ci.yml`. |
| IT4 | `make tools-verify`                                  | RED-ON-HOST / GREEN-IN-DEVCONTAINER | Bare host (Fedora) has Go 1.26, Node 22, OpenBao, no terraform/tbls/oapi-codegen/gitleaks/mmdc. Pins target the devcontainer + CI runners. `go.mod` directive OK, `go mod verify` OK. |

## Gaps closed this run
- `scripts/smoke-bootstrap.sh` — AT1 test file the plan named but didn't ship.
- `Makefile` `generate`/`test`/`build` — empty-state guards mirror the CI `has_go` pattern, so AT1 passes on a fresh clone before plan 01 lands queries/packages.

## Known gaps (NOT closed by plan 00)
- Runtime Vault rendering (vault agent / k8s Vault Agent Injector) — deferred to the infra K8s plan.
- IT4 strict pass requires running inside the devcontainer or CI; bare workstation pins drift expected.
