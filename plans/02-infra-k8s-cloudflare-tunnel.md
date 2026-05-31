---
feature: 02-infra-k8s-cloudflare-tunnel
status: draft   # require-prereq.sh greps this — change to "approved" only via /plan-eng-review + human sign-off
approved_by: TBD
approved_at: TBD
domain: infra
depends_on:
  - 00-bootstrap-deps-vault
  - 01-infra-postgres-tailnet   # shares homelab k3s + Vault Agent Injector pattern
unblocks:
  - 03-backend-cqrs-core         # deploy target + public ingress for backend services
decisions:
  cluster: homelab k3s (existing) — same cluster as plan 01 Postgres
  ingress_strategy: Cloudflare Tunnel (cloudflared) — NO public LoadBalancer, NO Cloudflare proxy on origin IP
  exposure: thee5176.com/reckonna/* (path-based, NOT a subdomain)
  dns: single CNAME `thee5176.com` → `<tunnel-uuid>.cfargotunnel.com` managed by Terraform Cloudflare provider
  app_arch: minimal Go/Gin clean architecture (cmd/server + internal/{handler,service,domain}) —
            deploy harness ONLY, NOT the CQRS rewrite (that's plan 03)
  secrets: Cloudflare tunnel token + API token in Vault at `secret/app/cloudflare/tunnel`;
           rendered into cloudflared Deployment via Vault Agent Injector (same pattern as plan 01)
  iac: Terraform owns the Cloudflare Zone + DNS + Tunnel resources; kustomize owns k8s manifests.
  human_only: `terraform apply`, `kubectl apply`, `cloudflared tunnel create` — manifests + tf only in this plan
---

# Plan 02 — Minimal Go/Gin App on Homelab k3s, Exposed via Cloudflare Tunnel at thee5176.com/reckonna

Stands up a **deploy harness**: a minimal Go/Gin clean-architecture service running on the existing
homelab k3s cluster, fronted by Cloudflare Tunnel so it is reachable at `https://thee5176.com/reckonna`
without exposing the homelab's public IP. **No `terraform apply`, no `kubectl apply`, no `cloudflared
tunnel create` in this plan** — those are human-only per `devops.md`. Deliverables: manifests,
Terraform, helper scripts, docs, a Go binary with a `/healthz` and `/reckonna/hello`.

**Prerequisite:** Plan 00 (deps + Vault) landed; Plan 01 (postgres-tailnet) landed (re-uses its
Vault Agent Injector + namespace conventions). Backend CQRS (plan 03) is downstream — it lands on
top of this ingress.

## Decisions (locked at draft, 2026-05-31)

| # | Decision | Rationale |
|---|----------|-----------|
| 1 | Cloudflare Tunnel (cloudflared as a k8s Deployment) — NOT NodePort, NOT public LoadBalancer. | Homelab has no static public IP; cloudflared egresses outbound to Cloudflare edge — zero inbound holes. |
| 2 | Path-based routing on apex: `thee5176.com/reckonna/*` → in-cluster `Service reckonna-app:80`. | User decision: keep a single root domain; reckonna is one app among many on the apex. |
| 3 | Single Terraform-managed DNS CNAME: `thee5176.com` → `<tunnel-uuid>.cfargotunnel.com` (proxied). | Apex CNAME flattening handled by Cloudflare; tunnel ingress rules do the path split. |
| 4 | Minimal Go/Gin "clean architecture" app: `cmd/server` thin main; `internal/handler` (Gin) → `internal/service` (use cases) → `internal/domain` (entities). NO DB in v1 — `/healthz` + `/reckonna/hello` only. | Proves the deploy + tunnel pipeline end-to-end without coupling to plan 03 backend or to Postgres. |
| 5 | All Cloudflare secrets (tunnel token, account-scoped API token) live at `secret/app/cloudflare/tunnel` in Vault; rendered into the cloudflared Pod via Vault Agent Injector. NO literal token in any manifest, tfvars, or env file. | `secrets-vault.md`. |
| 6 | Tunnel `ingress:` config (the cloudflared YAML mapping hostnames+paths → services) is committed (no secrets in it) and mounted as a ConfigMap. | Path routing is config, not code; reviewable in PRs. |
| 7 | Container image: distroless static Go, built + pushed to `ghcr.io/thee5176/reckonna-harness:<sha>` by CI. | Same registry convention as plan 03's S17b. |
| 8 | One step = one commit = one `Plan: S<n>` trailer. Conventional Commits. No squashing across steps. | `devops.md`. |

## File structure

```
plans/02-infra-k8s-cloudflare-tunnel.md      # this file
cmd/
  server/
    main.go                                  # wires Gin router, /healthz, /reckonna/*
internal/
  handler/
    hello.go                                 # GET /reckonna/hello — calls HelloService
    health.go                                # GET /healthz — liveness/readiness
  service/
    hello.go                                 # business use case (trivial v1)
  domain/
    greeting.go                              # entity / value object
build/
  Dockerfile.server                          # multi-stage distroless static Go
infra/
  k8s/
    reckonna-app/
      namespace.yaml
      deployment.yaml                        # 2 replicas, readiness/liveness on /healthz
      service.yaml                           # ClusterIP :80 → :8080
      configmap.yaml                         # app config (non-secret)
      kustomization.yaml
    cloudflared/
      namespace.yaml
      serviceaccount.yaml                    # + Vault role binding annotations
      deployment.yaml                        # cloudflared, token from Vault-rendered file
      configmap-ingress.yaml                 # tunnel ingress rules (hostname+path → service)
      kustomization.yaml
  terraform/
    cloudflare.tf                            # cloudflare_zone (data), cloudflare_record CNAME,
                                             # cloudflare_tunnel, cloudflare_tunnel_config
    cloudflare-providers.tf                  # provider block; api_token via vault data source
scripts/
  tunnel-health.sh                           # curl https://thee5176.com/reckonna/healthz; exit non-zero on !200
  tunnel-info.sh                             # prints tunnel UUID + CNAME target from terraform output
docs/
  cloudflare-tunnel-setup.md                 # how-to: create tunnel, store token in Vault, apply
Makefile                                     # + k8s-validate, tf-validate, server-build, tunnel-health
.github/workflows/ci.yml                     # + harness image build job (ghcr.io publish on push to develop/main)
```

---

## Section 1 — Acceptance-test spec (E2E)

E2E tests in this plan are **manual until** the live tunnel + DNS are applied (human-only).
Once live, the smoke script runs from CI on a schedule.

| ID  | Given / When / Then | Domain | Test artifact |
|-----|---------------------|--------|---------------|
| AT1 | Given tunnel applied + DNS propagated / When `curl -sf https://thee5176.com/reckonna/healthz` / Then HTTP 200 with body `{"status":"ok"}`. | infra+app | `scripts/tunnel-health.sh` |
| AT2 | Given the same / When `curl -sf https://thee5176.com/reckonna/hello?name=world` / Then HTTP 200 with body `{"greeting":"hello, world"}`. | app | `tests/hello_e2e_test.sh` (uses public URL) |
| AT3 | Given a request to `thee5176.com/` (root, no `/reckonna` prefix) / When tunnel ingress evaluates / Then 404 (or fallback service if configured). | infra | manual; documented in `docs/cloudflare-tunnel-setup.md` |
| AT4 | Given the homelab origin IP / When `curl -sf http://<homelab-public-ip>/reckonna/healthz` / Then connection fails (no inbound port open). | infra | manual; documents zero-trust posture |
| AT5 | Given `kubectl rollout restart deployment/cloudflared -n cloudflared` / When restart completes / Then AT1 still passes (≤30s outage during rollout). | infra | manual; rollout uses default RollingUpdate strategy |

## Section 2 — Integration-test spec

| ID  | Condition to verify | Domain | Test artifact |
|-----|---------------------|--------|---------------|
| IT1 | Every manifest under `infra/k8s/**` passes `kubeconform -strict` against k8s 1.30. | infra | `make k8s-validate` |
| IT2 | `terraform validate` is green for `infra/terraform/cloudflare.tf` + providers. | infra | `make tf-validate` |
| IT3 | No literal secret value (Cloudflare tunnel token, API token, account ID treated as secret) appears in any committed file under `infra/**`. `gitleaks` clean against the new files. | infra | `gitleaks detect --no-git -s infra/` |
| IT4 | `configmap-ingress.yaml` declares the routing rule `hostname: thee5176.com` + `path: /reckonna/*` → `service: http://reckonna-app.reckonna-app.svc.cluster.local:80`. | infra | grep test `tests/tunnel-ingress_test.sh` |
| IT5 | `cloudflared` Deployment has Vault Agent annotations rendering `TUNNEL_TOKEN` from `secret/app/cloudflare/tunnel`. NO `env.value` literal for `TUNNEL_TOKEN` anywhere. | infra | grep test `tests/cloudflared-vault_test.sh` |
| IT6 | `cmd/server/main.go` wires the router; handlers depend on services via interfaces (no global state); domain package has zero imports from `handler` or `service` (clean architecture inversion). | app | `go test ./...` + `go vet`; an arch-test using `go/parser` walks `internal/domain` imports and fails if `handler`/`service` appear. |
| IT7 | `build/Dockerfile.server` produces an image whose `/server` binary returns 200 on `/healthz` when run with `docker run -p 8080:8080`. | app | CI job: `docker build` then container smoke. |
| IT8 | Terraform `cloudflare_record` resource sets `proxied = true` and `type = "CNAME"`; `cloudflare_tunnel_config` ingress rules match `configmap-ingress.yaml`. (Drift catch: tunnel config exists in two places — k8s ConfigMap consumed by cloudflared, and Terraform-managed remote config. This plan keeps the ConfigMap as the source of truth and Terraform mirrors it; IT8 asserts they match via a generator script.) | infra | `scripts/check-tunnel-config-parity.sh` |

## Section 3 — Implementation steps (one commit each)

Each step compiles/validates standalone. Manifest-only steps verify with static checks (kubeconform /
terraform validate / grep tests). Go steps follow TDD — failing test commit before code commit.

| ID | Commit (verbatim) | Files | Verify |
|----|-------------------|-------|--------|
| S0 | `docs(plan): infra plan 02 — minimal go/gin app on k3s via cloudflare tunnel` | `plans/02-infra-k8s-cloudflare-tunnel.md` | review only |
| S1 | `test(app): failing /healthz + /reckonna/hello handler tests` | `internal/handler/health_test.go`, `internal/handler/hello_test.go` | `go test ./internal/handler/...` RED |
| S2 | `feat(app): minimal gin server — healthz + hello (clean arch)` | `cmd/server/main.go`, `internal/handler/health.go`, `internal/handler/hello.go`, `internal/service/hello.go`, `internal/domain/greeting.go` | `go test ./...` GREEN; IT6 |
| S3 | `feat(build): multi-stage distroless dockerfile for harness server` | `build/Dockerfile.server`, `.dockerignore` | `docker build` succeeds; container smoke on `/healthz` (IT7) |
| S4 | `chore(k8s): reckonna-app namespace + deployment + service` | `infra/k8s/reckonna-app/{namespace,deployment,service,configmap,kustomization}.yaml` | `kubeconform -strict`; IT1 |
| S5 | `chore(k8s): cloudflared namespace + sa with vault injector annotations` | `infra/k8s/cloudflared/{namespace,serviceaccount,deployment,kustomization}.yaml` | `kubeconform`; IT1; IT5 grep |
| S6 | `feat(k8s): cloudflared ingress configmap — thee5176.com/reckonna routing` | `infra/k8s/cloudflared/configmap-ingress.yaml`, `tests/tunnel-ingress_test.sh` | IT4 |
| S7 | `feat(infra): terraform — cloudflare tunnel + apex CNAME + remote tunnel config` | `infra/terraform/cloudflare-providers.tf`, `infra/terraform/cloudflare.tf` | `terraform validate`; IT2; `gitleaks` IT3 |
| S8 | `feat(scripts): tunnel-health.sh + tunnel-info.sh + check-tunnel-config-parity.sh` | `scripts/tunnel-health.sh`, `scripts/tunnel-info.sh`, `scripts/check-tunnel-config-parity.sh`, `tests/scripts_test.sh` | shellcheck; IT8 |
| S9 | `chore(make): k8s-validate, tf-validate, server-build, tunnel-health targets` | `Makefile` | `make help` lists new targets; targets skip cleanly when tools missing |
| S10| `ci(workflow): build + publish harness image to ghcr.io on push to develop/main` | `.github/workflows/ci.yml` (new job `harness-image`) | dry-run via `act` (optional); IT7 in CI |
| S11| `docs(infra): how to provision the cloudflare tunnel + put token in vault` | `docs/cloudflare-tunnel-setup.md` | manual review; markdownlint optional |

### Step notes

- **S1/S2 — clean architecture.** Handlers receive a `HelloService` interface in a constructor;
  `cmd/server/main.go` wires the concrete `service.Hello{}` in. `internal/domain` has zero
  cross-imports — IT6 enforces with a `go/parser` walk. Adopted from plan 03's CQRS layout so the
  same skeleton scales when real domain code lands.
- **S5 — Vault Agent annotations on cloudflared.** Same pattern as plan 01's Postgres
  StatefulSet: `vault.hashicorp.com/agent-inject: "true"`, role `reckonna-cloudflared`, template
  renders `TUNNEL_TOKEN` from `secret/data/app/cloudflare/tunnel` into `/vault/secrets/tunnel.env`,
  which the container `sources` at start. NO `env.value` literal.
- **S6 — tunnel ingress ConfigMap.** YAML structure:
  ```yaml
  tunnel: <managed-by-token>
  ingress:
    - hostname: thee5176.com
      path: /reckonna/*
      service: http://reckonna-app.reckonna-app.svc.cluster.local:80
    - service: http_status:404
  ```
  The trailing `http_status:404` is required by cloudflared as the catch-all rule.
- **S7 — Terraform Cloudflare.** Uses `cloudflare_zero_trust_tunnel_cloudflared` +
  `cloudflare_zero_trust_tunnel_cloudflared_config` (provider v4+ naming). The CNAME record uses
  Cloudflare's apex flattening (the record name is `@`, value is `<tunnel-uuid>.cfargotunnel.com`,
  `proxied = true`). Provider auth via `data "vault_kv_secret_v2"` pulling
  `secret/app/cloudflare/tunnel:api_token` — NEVER a `.tfvars`.
- **S8 — config parity check.** `scripts/check-tunnel-config-parity.sh` parses both the k8s
  ConfigMap ingress rules and `terraform plan -out` JSON for the tunnel config; fails if they
  diverge. Prevents the "two sources of truth" drift IT8 calls out.
- **S10 — CI image.** New job `harness-image` runs after lint/test; builds + pushes
  `ghcr.io/thee5176/reckonna-harness:<sha>` + `:develop` / `:main` on those branches. Uses
  `docker/build-push-action` with provenance + SBOM attestations (matches plan 03 S17b).

## Hand-off to the heads

- **infra-engineer (HEAD):** owns S4–S11. Writes IT1–IT5, IT8 as failing/grep tests FIRST,
  then green via `iac-ops` → `tdd-implementer` → `code-reviewer`. Provisions the tunnel + DNS
  manually post-merge (`devops.md` — human-only `terraform apply`).
- **backend-engineer (HEAD):** owns S1–S3, S10 (the Go harness + Dockerfile + CI image job).
  Writes IT6, IT7 as failing tests FIRST; uses the same `internal/{handler,service,domain}`
  skeleton plan 03 adopts so the CQRS rewrite drops in without restructuring.
- **plan-tracker:** logs each landed step to `02-infra-k8s-cloudflare-tunnel.impl.md`.

**"Done" (plan 02)** = IT1–IT8 green; `make k8s-validate` + `make tf-validate` + `make server-build`
clean; `gitleaks` clean across new files; harness image published to ghcr.io on push to develop;
docs (`docs/cloudflare-tunnel-setup.md`) merged. AT1–AT5 run **manually** post-`terraform apply`
+ `kubectl apply` (human-only); they become CI smoke once the live tunnel is up.

## NOT in scope (plan 02)

- The real CQRS command/query services — those land on this ingress in plan 03. This plan ships
  ONLY the `reckonna-harness` minimal app.
- Postgres provisioning — already covered by plan 01 (postgres-tailnet). The harness has no DB.
- Multi-environment (staging / prod) tunnels — single homelab tunnel only in v1.
- Cloudflare Access (zero-trust policies on the tunnel) — public-by-default in v1; document
  follow-up if/when auth is needed at the edge (vs. in-app via Keycloak — plan 03's choice).
- Subdomain ingress (`reckonna.thee5176.com`) — user picked path-based on apex; a follow-up plan
  can add subdomain if path collisions arise.
- Observability scrape (OTel collector receiving from the harness) — `/healthz` only in v1; OTel
  pipeline is a separate infra plan.

## What already exists

- Plan 00 landed: `go.mod`, Makefile with empty-state guards, gitleaks CI gate, Vault wiring docs.
- Plan 01 landed: homelab k3s cluster has `postgres` namespace + Vault Agent Injector + Tailscale
  Operator already running. This plan re-uses the Vault Agent Injector pattern (S5).
- `infra/main.tf`, `infra/providers.tf`, `infra/secrets.tf` already exist (plan 00 / fix branch);
  this plan adds `infra/terraform/cloudflare*.tf` alongside without touching them.
- No Cloudflare resources are currently terraformed; tunnel + DNS are greenfield here.

## GSTACK REVIEW REPORT

| Review | Trigger | Why | Runs | Status | Findings |
|--------|---------|-----|------|--------|----------|
| Eng Review | `/plan-eng-review` | Architecture & tests (required before approval) | 0 | DRAFT | — |
| DX Review | `/plan-devex-review` | Developer experience gaps | 0 | — | — |
| CEO Review | `/plan-ceo-review` | Scope & strategy | 0 | — | — |

**VERDICT:** DRAFT — needs `/plan-eng-review` + human approval before any commit beyond S0 lands.
