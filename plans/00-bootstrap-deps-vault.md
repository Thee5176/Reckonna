# Plan 00 — Bootstrap Dependencies + Close Vault/Secrets Gaps

**Source of truth** for the dependency-bootstrap + secrets-enforcement feature. Every head reads
this. Derived from the dependency & vault audit (2026-05-25): repo is governance scaffold with zero
dependency manifests, and gitleaks is named in 3 docs but absent from CI.

**Scope:** bootstrap toolchain manifests (Go/RN/Terraform/Make/CI/compose) and make the secrets
policy actually enforced (harden `no-secrets.sh`, wire gitleaks). No domain/business code here —
that lands in later plans once the toolchain is green.

**Out of scope (deferred):** runtime Vault render (vault agent / k8s Vault Agent Injector) — waits
for the infra K8s plan. Tracked as a known gap, not closed here.

---

## Pinned dependencies (this plan is the single source of versions)

Reproducible toolchain. The `.devcontainer/` (S0) installs exactly these; CI uses the same pins;
`go.mod` / `package.json` carry the library pins. Bump versions ONLY by editing this plan.

### Go — runtime libraries (`go.mod`, Go 1.23)
| Module | Version | Why |
|--------|---------|-----|
| github.com/gin-gonic/gin | v1.10.x | HTTP framework |
| github.com/jackc/pgx/v5 | v5.7.x | Postgres driver/pool |
| github.com/shopspring/decimal | v1.4.x | exact money (NUMERIC) — replaces source `Double` |
| github.com/google/uuid | v1.6.x | UUIDv7 (`uuid.NewV7`) |
| go.opentelemetry.io/otel (+ otelgin, otlptracehttp) | v1.3x.x | vendor-neutral tracing |
| github.com/coreos/go-oidc/v3 | v3.11.x | Keycloak OIDC discovery + JWKS |
| github.com/golang-jwt/jwt/v5 | v5.2.x | JWT claims |
| github.com/getkin/kin-openapi | v0.128.x | request/response validation vs openapi.yaml |
| github.com/stretchr/testify | v1.10.x | assertions |
| github.com/testcontainers/testcontainers-go | v0.34.x | integration DB |

### Go / CLI tools (`.devcontainer/post-create.sh`)
| Tool | Version | Use |
|------|---------|-----|
| sqlc | v1.27.0 | DB-first codegen |
| golang-migrate | v4.18.1 | migrations (`-tags postgres`) |
| golangci-lint | v1.62.2 | lint gate |
| tbls | v1.79.0 | ERD/schema docs + `tbls diff` gate (plan 01) |
| oapi-codegen | v2.4.1 | OpenAPI types (plan 01) |
| gitleaks | 8.21.2 | secret scan (S8) |
| Vault CLI | 1.18.2 | `vault kv get` at dev time |
| @mermaid-js/mermaid-cli | 11.4.0 | render sequence diagrams (plan 01) |

### Node / frontend (recorded now; consumed by the frontend feature)
| Dep | Version |
|-----|---------|
| Node | 20 LTS |
| expo | ~54 · react-native 0.81.5 · react 19.1 |
| react-native-paper | ^5.14 · expo-router ^6 |
| react-hook-form | ^7.71 · axios ^1.13 · @react-native-async-storage/async-storage ^2.2 |
| jest + @testing-library/react-native | latest in lockfile |

### Infra / services
| Item | Version |
|------|---------|
| Terraform | 1.9.x (+ tflint) |
| kubectl | latest (vendor-neutral) |
| PostgreSQL | 17 (source RDS was 17.4) |
| Keycloak | 26.x (self-hosted OIDC — infra feature) |
| HashiCorp Vault | 1.18.x (server; CLI pinned above) |

---

## Section 1 — Acceptance-test spec (E2E)   [from business requirements]

| ID  | Given / When / Then                                                                              | Domain | Test file                         |
|-----|-------------------------------------------------------------------------------------------------|--------|-----------------------------------|
| AT1 | Given a fresh clone / When run `make generate && make test` / Then toolchain resolves, build green | infra  | scripts/smoke-bootstrap.sh        |
| AT2 | Given a planted fake secret in a tracked path / When CI runs / Then gitleaks fails the build      | infra  | .github/workflows/ci.yml (gitleaks job) |
| AT3 | Given Terraform with a Vault data source (no .tfvars) / When `terraform validate` / Then passes   | infra  | infra/validate.test (CI step)     |

## Section 2 — Integration-test spec   [from architecture / toolchain contracts]

| ID  | Condition to verify                                                                  | Domain   | Test file                          |
|-----|--------------------------------------------------------------------------------------|----------|------------------------------------|
| IT1 | `go build ./...` compiles every package; module graph resolves                        | backend  | CI: go-build job                   |
| IT2 | `no-secrets.sh` blocks BOTH quoted and unquoted inline secrets on Edit/Write          | infra    | tests/no-secrets_test.sh           |
| IT3 | CI workflow runs all 6 gates: go test -race · jest · e2e · terraform validate · Sonar · gitleaks | infra | .github/workflows/ci.yml (actionlint) |
| IT4 | `scripts/deps-check.sh` passes: every tool present at its pinned version + go.mod directive/verify + package.json — pins read from one source (.devcontainer/versions.sh) | infra | scripts/deps-check.sh (via `make tools-verify`) |

## Section 3 — Implementation steps (one commit each; unit test per step)

One step = one commit = one PR-reviewable change. Each compiles/passes on its own. TDD visible:
failing-test commit BEFORE code commit for the security steps (S6, S7).

| ID | Commit message (verbatim)                                  | Files                                                              | Domain   | Depends   | Unit test                              |
|----|------------------------------------------------------------|-------------------------------------------------------------------|----------|-----------|----------------------------------------|
| S0 | `chore(dev): reusable devcontainer + pinned tools + deps-check` | .devcontainer/devcontainer.json, .devcontainer/versions.sh, .devcontainer/post-create.sh, scripts/deps-check.sh | infra | - | devcontainer builds; `make tools-verify` (IT4) |
| S1 | `chore(build): go mod init + core backend deps`            | go.mod, go.sum                                                     | backend  | S0        | `go build ./...` (compiles empty pkgs) |
| S2 | `chore(build): sqlc config + Makefile targets`             | sqlc.yaml, Makefile                                               | backend  | S1        | `make generate` dry-run; `make` help   |
| S3 | `chore(build): docker-compose postgres for make up`        | docker-compose.yml                                                 | backend  | -         | `docker compose config` valid          |
| S4 | `chore(frontend): package.json + jest config`              | package.json, jest.config.js, app/.gitkeep, components/.gitkeep   | frontend | -         | `npm test -- --watchAll=false` (0 ok)  |
| S5 | `chore(infra): terraform skeleton + vault provider`        | infra/main.tf, infra/providers.tf, infra/secrets.tf               | infra    | -         | `terraform validate` (satisfies AT3)   |
| S6 | `test(security): failing no-secrets unquoted-secret case`  | tests/no-secrets_test.sh                                          | infra    | -         | unquoted secret bypasses (RED, IT2)    |
| S7 | `fix(security): harden no-secrets.sh regex (quoted+unquoted)` | .claude/hooks/no-secrets.sh                                     | infra    | S6        | tests/no-secrets_test.sh (GREEN, IT2)  |
| S7b| `fix(hooks): scope verify-infra.sh to k8s/infra paths`       | .claude/hooks/verify-infra.sh                                    | infra    | -         | non-manifest yaml (config/coa.yaml, sqlc.yaml) no longer fails kubeconform |
| S8 | `ci(security): gitleaks gate + config`                      | .gitleaks.toml, .github/workflows/ci.yml                          | infra    | S1,S5,S7  | gitleaks on runtime-planted secret (AT2) |
| S9 | `ci: full pipeline (go race + jest + e2e + tf + sonar)`    | .github/workflows/ci.yml                                          | infra    | S8        | actionlint; all 6 jobs present (IT3)   |

### Step notes
- **S0 devcontainer + deps-check (already scaffolded):** `.devcontainer/devcontainer.json` +
  `post-create.sh` give every contributor and CI the same toolchain — Go 1.23, Node 20, Terraform,
  kubectl, docker-in-docker via official features. **Version pins live in ONE file**,
  `.devcontainer/versions.sh`, sourced by BOTH `post-create.sh` (install) and `scripts/deps-check.sh`
  (validate) — no second list to drift. `scripts/deps-check.sh` checks every tool is present at its
  pinned version, plus `go.mod` go-directive + `go mod verify` + `package.json` presence; exits non-zero
  on any miss (IT4). `post-create.sh` runs it (`--tools-only`) at the end. **No secrets baked** —
  `VAULT_ADDR` from host env (`${localEnv:VAULT_ADDR}`), auth at runtime. S0 still owes the
  `make tools-verify` target wrapping `scripts/deps-check.sh` (S2 Makefile).
- **S1 deps (pinned — see Pinned dependencies table):** gin, pgx/v5, shopspring/decimal, google/uuid
  (UUIDv7), otel + otlphttp, coreos/go-oidc + golang-jwt/v5, getkin/kin-openapi, testify,
  testcontainers-go. Pin Go 1.23 in `go.mod`.
- **S6/S7 (TDD):** S6 commits a test proving `password=supersecret` (unquoted, no `vault`) currently
  passes the hook — RED. S7 widens the regex to make `["']?` optional and re-tests — GREEN. Test
  writes the planted secret to a `mktemp` file at runtime; never commit a secret string.
- **S7b (real defect found this session):** the PostToolUse `verify-infra.sh` runs `kubeconform` on
  EVERY `.yaml`, so non-manifest yaml (`config/coa.yaml`, `sqlc.yaml`, `docker-compose.yml`) fails
  validation. Scope it to `kubernetes/**` + `infra/**` (or by `kind:`/`apiVersion:` presence) so only
  real k8s manifests are checked. Until fixed, bootstrap yaml writes trip a false-positive block.
- **S8 gitleaks:** fixture is generated at runtime in CI (not committed), so neither `no-secrets.sh`
  nor the gitleaks scan trips on a tracked file. `.gitleaks.toml` carries the allowlist for the
  test-fixture path only.
- **S5/S8 Vault:** Terraform pulls secrets via `vault` provider **data sources** — no `.tfvars`,
  consistent with secrets-vault.md and iac-ops/SKILL.md.

### Known gap (NOT closed by this plan)
Runtime env rendering from Vault (vault agent / k8s Vault Agent Injector) is unimplemented. The
secrets policy is enforced at author-time (hook) and commit-time (gitleaks) but not yet at deploy-time.
Close in the infra K8s plan.

## Hand-off to the heads
- **infra-engineer (HEAD):** owns S0, S2, S3, S5, S6, S7, S8, S9. Write AT1–AT3 + IT2/IT3/IT4 as
  failing checks first, then green via iac-ops → code-reviewer. (S0 devcontainer scaffold already
  landed; finish `scripts/tools-verify.sh` + `make tools-verify`.)
- **backend-engineer (HEAD):** owns S1 (+ verifies IT1 `go build ./...`).
- **frontend-engineer (HEAD):** owns S4.
"Done" = AT1–AT3 + IT1–IT4 green, devcontainer builds, `make tools-verify` reports all pins.
plan-tracker logs each landed step to `00-bootstrap-deps-vault.impl.md`.
