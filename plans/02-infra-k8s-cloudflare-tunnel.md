---
feature: 02-infra-k8s-cloudflare-tunnel
status: approved   # require-prereq.sh greps this — approved via /plan-eng-review + human sign-off
approved_by: thee5176
approved_at: 2026-06-29
domain: infra
depends_on:
  - 00-bootstrap-deps-vault
  - 01-infra-postgres-tailnet   # shares homelab k3s + Vault Agent Injector pattern
unblocks:
  - 03-backend-cqrs-core         # deploy target + public ingress for backend services
decisions:
  cluster: homelab k3s (existing) — same cluster as plan 01 Postgres
  ingress_strategy: Cloudflare Tunnel (cloudflared) — NO public LoadBalancer, NO Cloudflare proxy on origin IP
  exposure: reckonna.thee5176.com (subdomain — NOT path-based on apex; apex untouched)
  dns: CNAME `reckonna.thee5176.com` → `<tunnel-uuid>.cfargotunnel.com` managed by Terraform Cloudflare provider; apex thee5176.com NOT modified
  tunnel_config: remote-managed via Terraform `cloudflare_zero_trust_tunnel_cloudflared_config`; cloudflared pod authenticates via tunnel token and pulls config from Cloudflare API at startup (NO k8s ConfigMap routing, NO parity script)
  app_arch: nginx:alpine serving static content from a k8s ConfigMap — deploy harness ONLY (NO Go app; plan 03 establishes Go build pipeline)
  secrets: Cloudflare tunnel token + API token in Vault at `secret/app/cloudflare/tunnel`; rendered into cloudflared Deployment via Vault Agent Injector (same pattern as plan 01)
  iac: Terraform owns Cloudflare Zone + DNS + Tunnel + tunnel ingress config; kustomize owns k8s manifests for reckonna-app + cloudflared
  human_only: `terraform apply`, `kubectl apply`, `cloudflared tunnel create` — manifests + tf only in this plan
review_log:
  - /plan-eng-review on 2026-06-08: 3 scope reductions accepted (D1=A drop Go harness, D2=C remote-managed config, D3=B subdomain over apex); 4 test gaps added; AT6 apex-regression added
---

# Plan 02 — Minimal nginx Harness on Homelab k3s, Exposed via Cloudflare Tunnel at reckonna.thee5176.com

Stands up a **deploy harness**: a stock `nginx:alpine` Pod serving static content
from a k8s ConfigMap on the existing homelab k3s cluster, fronted by Cloudflare
Tunnel so it is reachable at `https://reckonna.thee5176.com` without exposing the
homelab's public IP. **No `terraform apply`, no `kubectl apply`, no `cloudflared
tunnel create` in this plan** — those are human-only per `devops.md`. Deliverables:
manifests, Terraform, helper scripts, docs.

**Prerequisite:** Plan 00 (deps + Vault) landed; Plan 01 (postgres-tailnet) landed
(re-uses its Vault Agent Injector + namespace conventions). Backend CQRS (plan 03)
is downstream — it lands on top of this ingress.

## Decisions (locked at draft, 2026-06-08 — post `/plan-eng-review`)

| # | Decision | Rationale |
|---|----------|-----------|
| 1 | Cloudflare Tunnel (cloudflared as a k8s Deployment) — NOT NodePort, NOT public LoadBalancer. | Homelab has no static public IP; cloudflared egresses outbound to Cloudflare edge — zero inbound holes. |
| 2 | **Subdomain ingress: `reckonna.thee5176.com` (NOT path-based on apex).** Apex `thee5176.com` is NOT modified. | Apex DNS untouched — existing apex content (whatever lives there) keeps working. Each future app gets its own subdomain. Reversible by deleting one CNAME. `/plan-eng-review` D3=B. |
| 3 | Single Terraform-managed DNS CNAME: `reckonna.thee5176.com` → `<tunnel-uuid>.cfargotunnel.com` (proxied). | Subdomain CNAME, no apex flattening needed. |
| 4 | **Remote-managed tunnel config via Terraform `cloudflare_zero_trust_tunnel_cloudflared_config`.** cloudflared pod authenticates with tunnel token and pulls the ingress config from the Cloudflare API at startup. NO k8s ConfigMap routing rules. NO parity script. | Single source of truth at Cloudflare; zero drift class. Pod manifest is minimal (token + args). `/plan-eng-review` D2=C. |
| 5 | **`nginx:alpine` upstream image** serving static `/healthz` + `/reckonna/hello` content mounted from a k8s ConfigMap. **NO Go harness app, NO custom container build.** | The harness exists to prove the deploy + tunnel pipeline; nginx is rock-solid Layer-1 boring tech. Plan 03 establishes the Go build → distroless → ghcr.io pipeline on real backend code. `/plan-eng-review` D1=A. |
| 6 | All Cloudflare secrets (tunnel token, account-scoped API token) live at `secret/app/cloudflare/tunnel` in Vault; rendered into the cloudflared Pod via Vault Agent Injector. NO literal token in any manifest, tfvars, or env file. | `secrets-vault.md`. |
| 7 | One step = one commit = one `Plan: S<n>` trailer. Conventional Commits. No squashing across steps. | `devops.md`. |

## File structure

```
plans/02-infra-k8s-cloudflare-tunnel.md      # this file
infra/
  k8s/
    reckonna-app/
      namespace.yaml
      configmap.yaml                         # nginx index/healthz/reckonna/hello static content
      deployment.yaml                        # nginx:alpine, 2 replicas, readiness/liveness on /healthz, mounts configmap
      service.yaml                           # ClusterIP :80 → :80
      kustomization.yaml
    cloudflared/
      namespace.yaml
      serviceaccount.yaml                    # + Vault role binding annotations
      deployment.yaml                        # cloudflared run --token from Vault-rendered file; NO --config flag (remote-managed)
      kustomization.yaml
  terraform/
    cloudflare.tf                            # cloudflare_zone (data), cloudflare_record CNAME (reckonna subdomain),
                                             # cloudflare_zero_trust_tunnel_cloudflared,
                                             # cloudflare_zero_trust_tunnel_cloudflared_config (ingress rules)
    cloudflare-providers.tf                  # provider block; api_token via vault data source
scripts/
  tunnel-health.sh                           # curl https://reckonna.thee5176.com/healthz; exit non-zero on !200
  tunnel-info.sh                             # prints tunnel UUID + CNAME target from terraform output
  tunnel-dns-check.sh                        # dig +short reckonna.thee5176.com; expect substring cfargotunnel.com
docs/
  cloudflare-tunnel-setup.md                 # how-to: create tunnel, store token in Vault, apply, rollback
Makefile                                     # + k8s-validate, tf-validate, tunnel-health, tunnel-dns-check
.github/workflows/ci.yml                     # no new harness image job — nginx:alpine is upstream
```

**Removed from earlier draft (post `/plan-eng-review`):**
- `cmd/server/main.go`, `internal/handler/*`, `internal/service/*`, `internal/domain/*` — dropped with the Go harness (D1=A)
- `build/Dockerfile.server`, `.dockerignore` — no custom image build
- `infra/k8s/cloudflared/configmap-ingress.yaml` — dropped with locally-managed config (D2=C)
- `scripts/check-tunnel-config-parity.sh` — no parity check needed with a single source of truth (D2=C)
- `.github/workflows/ci.yml` `harness-image` job — no image to publish

---

## Section 1 — Acceptance-test spec (E2E)

E2E tests in this plan are **manual until** the live tunnel + DNS are applied (human-only).
Once live, the smoke scripts run from CI on a schedule.

| ID  | Given / When / Then | Domain | Test artifact |
|-----|---------------------|--------|---------------|
| AT1 | Given tunnel applied + DNS propagated / When `curl -sf https://reckonna.thee5176.com/healthz` / Then HTTP 200 with body `{"status":"ok"}`. | infra+app | `scripts/tunnel-health.sh` |
| AT2 | Given the same / When `curl -sf https://reckonna.thee5176.com/reckonna/hello` / Then HTTP 200 with body containing `hello`. | app | `tests/hello_e2e_test.sh` (uses public URL) |
| AT3 | Given the homelab origin IP / When `curl -sf http://<homelab-public-ip>/` / Then connection fails (no inbound port open). | infra | manual; documents zero-trust posture |
| AT4 | Given `kubectl rollout restart deployment/cloudflared -n cloudflared` / When restart completes / Then AT1 still passes within 60 seconds. | infra | manual; default RollingUpdate strategy |
| AT5 | Given DNS propagated / When `dig +short reckonna.thee5176.com` / Then output contains substring `.cfargotunnel.com`. | infra | `scripts/tunnel-dns-check.sh` |
| AT6 | **Apex regression:** Given pre-apply baseline `curl -sf https://thee5176.com/` / When `terraform apply` for plan 02 completes / Then post-apply `curl -sf https://thee5176.com/` returns the SAME response (apex unchanged). | infra | manual; documented in `docs/cloudflare-tunnel-setup.md` rollback section |

## Section 2 — Integration-test spec

| ID  | Condition to verify | Domain | Test artifact |
|-----|---------------------|--------|---------------|
| IT1 | Every manifest under `infra/k8s/**` passes `kubeconform -strict` against k8s 1.30. | infra | `make k8s-validate` |
| IT2 | `terraform validate` is green for `infra/terraform/cloudflare.tf` + providers. | infra | `make tf-validate` |
| IT3 | No literal secret value (Cloudflare tunnel token, API token, account ID treated as secret) appears in any committed file under `infra/**`. `gitleaks` clean against the new files. | infra | `gitleaks detect --no-git -s infra/` |
| IT4 | `infra/terraform/cloudflare.tf` declares a `cloudflare_zero_trust_tunnel_cloudflared_config` resource whose ingress block contains `hostname = "reckonna.thee5176.com"` → `service = "http://reckonna-app.reckonna-app.svc.cluster.local:80"` followed by a `service = "http_status:404"` catch-all. | infra | grep test `tests/tunnel-config_test.sh` |
| IT5 | `cloudflared` Deployment has Vault Agent annotations rendering `TUNNEL_TOKEN` from `secret/app/cloudflare/tunnel`. NO `env.value` literal for `TUNNEL_TOKEN` anywhere. | infra | grep test `tests/cloudflared-vault_test.sh` |
| IT6 | `reckonna-app` ConfigMap declares two files: `healthz` with content `{"status":"ok"}` and `reckonna_hello` with content containing the substring `hello`. nginx Deployment mounts the ConfigMap into `/usr/share/nginx/html`. | infra+app | grep test `tests/nginx-content_test.sh` |
| IT7 | `reckonna-app` Deployment `readinessProbe` and `livenessProbe` both target `path: /healthz`, `port: 80`, scheme HTTP. | infra | grep test `tests/probes_test.sh` |
| IT8 | `cloudflared` Deployment args use `tunnel --no-autoupdate run --token $(TUNNEL_TOKEN)`. NO `--config` flag present (remote-managed tunnel config). | infra | grep test `tests/cloudflared-args_test.sh` |
| IT9 | `cloudflare_record` in `cloudflare.tf` has `name = "reckonna"` (subdomain — NOT `@`), `type = "CNAME"`, `proxied = true`. No second `cloudflare_record` with `name = "@"` or `name = "thee5176.com"` exists (apex untouched). | infra | grep test `tests/tf-dns_test.sh` |

## Section 3 — Implementation steps (one commit each)

Each step compiles/validates standalone. Manifest-only steps verify with static checks
(kubeconform / terraform validate / grep tests).

| ID | Commit (verbatim) | Files | Verify |
|----|-------------------|-------|--------|
| S0 | `docs(plan): infra plan 02 — nginx harness on k3s via cloudflare tunnel (subdomain + remote-managed)` | `plans/02-infra-k8s-cloudflare-tunnel.md` (this rewrite) | review only |
| S1 | `chore(k8s): reckonna-app namespace + configmap (nginx static content) + deployment + service` | `infra/k8s/reckonna-app/{namespace,configmap,deployment,service,kustomization}.yaml` | `kubeconform -strict`; IT1; IT6 grep; IT7 grep |
| S2 | `chore(k8s): cloudflared namespace + sa with vault injector + remote-managed deployment` | `infra/k8s/cloudflared/{namespace,serviceaccount,deployment,kustomization}.yaml` | `kubeconform`; IT1; IT5; IT8 grep |
| S3 | `feat(infra): terraform — cloudflare subdomain CNAME + tunnel + remote-managed ingress config` | `infra/terraform/cloudflare-providers.tf`, `infra/terraform/cloudflare.tf` | `terraform validate`; IT2; gitleaks IT3; IT4 grep; IT9 grep |
| S4 | `feat(scripts): tunnel-health.sh + tunnel-info.sh + tunnel-dns-check.sh` | `scripts/tunnel-health.sh`, `scripts/tunnel-info.sh`, `scripts/tunnel-dns-check.sh`, `tests/scripts_test.sh` | shellcheck |
| S5 | `chore(make): k8s-validate, tf-validate, tunnel-health, tunnel-dns-check targets` | `Makefile` | `make help` lists new targets; targets skip cleanly when tools missing |
| S6 | `docs(infra): how to provision the cloudflare tunnel + vault token + rollback procedure` | `docs/cloudflare-tunnel-setup.md` | manual review; markdownlint optional |

### Step notes

- **S1 — nginx ConfigMap.** Two keys: `healthz` and `reckonna_hello`. Mounted at
  `/usr/share/nginx/html/` (filenames as URL paths). Default nginx config serves static
  files; `try_files` fallback handles `/reckonna/hello` → `reckonna_hello`. Probes
  curl `/healthz` on port 80.
- **S2 — cloudflared remote-managed.** Deployment `args`:
  `["tunnel", "--no-autoupdate", "run", "--token", "$(TUNNEL_TOKEN)"]`. `TUNNEL_TOKEN`
  is sourced from `/vault/secrets/cloudflared.env` via an `envFrom` script wrapper or
  a `command` that `source`s the file then `exec`s cloudflared. NO `--config` flag —
  cloudflared fetches the ingress rules from the Cloudflare API on startup. Vault Agent
  annotations re-use plan 01's Vault Agent Injector PATTERN but a NEW, dedicated role
  `reckonna-cloudflared` + policy (read-only on `secret/data/app/cloudflare/tunnel`) —
  this is NOT plan 01's `reckonna-postgres` role; the new role + policy must be
  provisioned in Vault first (human-only, documented in S6). Template renders
  `TUNNEL_TOKEN` from `secret/data/app/cloudflare/tunnel:token`.
- **S3 — Terraform Cloudflare.** Uses provider v4+ resource names
  `cloudflare_zero_trust_tunnel_cloudflared` + `cloudflare_zero_trust_tunnel_cloudflared_config`.
  The CNAME `cloudflare_record` has `name = "reckonna"` (subdomain) with the zone
  resolving to `thee5176.com` via the `cloudflare_zone` data source. `proxied = true`.
  Provider auth via `data "vault_kv_secret_v2"` pulling
  `secret/app/cloudflare/tunnel:api_token` — NEVER a `.tfvars`. **NO** `cloudflare_record`
  for the apex; AT6 verifies the apex is unchanged.
- **S4 — tunnel-dns-check.sh.** `dig +short reckonna.thee5176.com | grep -q
  '\.cfargotunnel\.com$' || exit 1`. Pairs with AT5.
- **S6 — docs include rollback.** Step-by-step: `terraform destroy
  -target=cloudflare_record.reckonna` removes only the subdomain CNAME; apex untouched
  throughout. Vault token rotation procedure documented separately.

---

## Failure modes

| Codepath | Realistic failure | Test? | Error handling? | User visibility |
|----------|-------------------|-------|-----------------|-----------------|
| cloudflared startup config fetch from CF API | Cloudflare API throttled or rate-limited; pod CrashLoop | No (live-only) | cloudflared default exponential backoff retries on its own | Visible in `kubectl logs -n cloudflared`; AT1 returns 502 until pod recovers |
| Tunnel token compromised | Bad actor replays token | N/A (preventive) | Rotate with `vault kv patch -mount=secret app/cloudflare/tunnel token=...` (PATCH, not PUT — the secret also holds `api_token`; `put` would wipe it). cloudflared does NOT hot-reload: the Vault Agent re-renders the file but the pod must be restarted (`kubectl rollout restart deploy/cloudflared -n cloudflared`) to read the new token — documented v1 behavior, no auto-reload. Then revoke the old tunnel in the CF dashboard. | None until rotated + restarted |
| Subdomain CNAME collision | `reckonna` already exists in Cloudflare zone | N/A | `terraform apply` errors at plan stage with explicit conflict | Visible in tf output before apply |
| nginx ConfigMap malformed | Bad index/healthz content | IT6 grep catches structure; readinessProbe catches runtime | Pod NotReady; Service has no endpoints; tunnel returns 502 | Visible immediately in `kubectl get pods -n reckonna-app` |
| cloudflared Deployment rollout | Brief outage during RollingUpdate | AT4 enforces ≤60s recovery | Default RollingUpdate strategy; 2 replicas | AT1 may fail intermittently for ≤60s |
| Apex content regression | `terraform apply` accidentally touches apex | IT9 grep blocks at PR; AT6 verifies post-apply | Plan structure forbids apex `cloudflare_record` | Visible in `curl https://thee5176.com/` diff |

**No silent failures flagged.** All failure modes have observable symptoms + a recovery path.

---

## Worktree parallelization strategy

| Step | Modules touched | Depends on |
|------|----------------|------------|
| S0 | plans/ | — |
| S1 | infra/k8s/reckonna-app/ | — |
| S2 | infra/k8s/cloudflared/ | — |
| S3 | infra/terraform/ | — |
| S4 | scripts/, tests/ | — |
| S5 | Makefile | S4 (references new scripts) |
| S6 | docs/ | — |

**Lanes:**
- Lane A: S1 (reckonna-app manifests) — independent
- Lane B: S2 (cloudflared manifests) — independent
- Lane C: S3 (terraform) — independent
- Lane D: S4 → S5 (scripts → Makefile, sequential within lane)
- Lane E: S6 (docs) — independent

**Execution order:** Launch A + B + C + D + E in parallel worktrees. Merge in any order.
No shared-module conflicts (each lane owns its own directory; only S5's Makefile edit
sits in a shared file but lands last in lane D after lane D itself is done).

---

## Hand-off to the heads

- **infra-engineer (HEAD):** owns S0–S6. Writes IT1–IT9 + AT1–AT6 as failing/grep tests
  FIRST, then green via `iac-ops` → `tdd-implementer` → `code-reviewer`. Provisions
  the tunnel + DNS manually post-merge (`devops.md` — human-only `terraform apply`).
- **backend-engineer (HEAD):** NOT involved in plan 02 (no Go code). Picks up at plan 03.
- **plan-tracker:** logs each landed step to `02-infra-k8s-cloudflare-tunnel.impl.md`.

**"Done" (plan 02)** = IT1–IT9 green; `make k8s-validate` + `make tf-validate` clean;
`gitleaks` clean across new files; docs (`docs/cloudflare-tunnel-setup.md`) merged.
AT1–AT6 run **manually** post-`terraform apply` + `kubectl apply` (human-only); AT1,
AT2, AT5 become CI smoke once the live tunnel is up; AT6 is a one-shot regression check
captured in the rollout doc.

## NOT in scope (plan 02)

- The real CQRS command/query services — those land on this ingress in plan 03. This
  plan ships ONLY the `reckonna-app` nginx harness.
- The Go build → distroless → ghcr.io image publish pipeline — plan 03 establishes it
  on real backend code (D1=A defers from plan 02).
- Locally-managed tunnel config (k8s ConfigMap routing rules) — D2=C chose
  remote-managed only; locally-managed deferred indefinitely.
- Apex `thee5176.com` routing changes — D3=B preserves apex untouched; any future
  routing changes for the apex are out of plan 02 scope.
- Postgres provisioning — already covered by plan 01 (postgres-tailnet). The harness
  has no DB.
- Multi-environment (staging / prod) tunnels — single homelab tunnel only in v1.
- Cloudflare Access (zero-trust policies on the tunnel) — public-by-default in v1;
  follow-up if/when auth is needed at the edge (vs. in-app via Keycloak — plan 03's
  choice).
- **OTel / observability — APPROVED EXCEPTION to devops.md** ("new endpoints/screens emit
  OpenTelemetry spans; observability is part of done"). Deliberately waived for plan 02: the
  `reckonna-app` harness is throwaway nginx with NO business logic to trace; cloudflared/tunnel
  health is observed via `kubectl logs -n cloudflared` + the Cloudflare dashboard. Real OTel
  spans arrive with plan 03's Go services on this same ingress. This is a scoped, approved
  deviation — recorded here explicitly, not an oversight. (OTel scrape of nginx access logs is
  itself a later infra plan.)

## What already exists

- Plan 00 landed: `go.mod`, Makefile with empty-state guards, gitleaks CI gate,
  Vault wiring docs.
- Plan 01 landed: homelab k3s cluster has `postgres` namespace + Vault Agent Injector
  + Tailscale Operator already running. This plan re-uses the Vault Agent Injector
  pattern (S2).
- `infra/main.tf`, `infra/providers.tf`, `infra/secrets.tf` already exist (plan 00 /
  fix branch); this plan adds `infra/terraform/cloudflare*.tf` alongside without
  touching them.
- No Cloudflare resources are currently terraformed; tunnel + DNS are greenfield here.

---

## Implementation Tasks

Synthesized from this review's findings. Each task derives from a specific finding
above. Run with Claude Code or Codex; checkbox as you ship.

- [ ] **T1 (P1, human: ~30min / CC: ~5min)** — k8s/reckonna-app — Write reckonna-app manifests for nginx:alpine + ConfigMap static content
  - Surfaced by: Step 0 D1=A (drop Go harness)
  - Files: `infra/k8s/reckonna-app/{namespace,configmap,deployment,service,kustomization}.yaml`
  - Verify: `kubeconform -strict`; IT6 grep; IT7 grep
- [ ] **T2 (P1, human: ~20min / CC: ~5min)** — k8s/cloudflared — Write cloudflared deployment with `tunnel --no-autoupdate run --token`, no `--config` flag, Vault Agent annotations
  - Surfaced by: Step 0 D2=C (remote-managed config only)
  - Files: `infra/k8s/cloudflared/{namespace,serviceaccount,deployment,kustomization}.yaml`
  - Verify: IT5 + IT8 grep
- [ ] **T3 (P1, human: ~30min / CC: ~10min)** — terraform — Write `cloudflare_record` with `name = "reckonna"` (subdomain), `cloudflare_zero_trust_tunnel_cloudflared`, `cloudflare_zero_trust_tunnel_cloudflared_config` with ingress rule
  - Surfaced by: Step 0 D3=B (subdomain) + D2=C (remote-managed)
  - Files: `infra/terraform/cloudflare.tf`, `infra/terraform/cloudflare-providers.tf`
  - Verify: `terraform validate`; IT4 + IT9 grep
- [ ] **T4 (P2, human: ~15min / CC: ~3min)** — scripts — Add `tunnel-dns-check.sh`; write `tunnel-health.sh` against subdomain URL
  - Surfaced by: Section 3 new test gap (DNS smoke) + D3=B
  - Files: `scripts/tunnel-health.sh`, `scripts/tunnel-info.sh`, `scripts/tunnel-dns-check.sh`
  - Verify: shellcheck clean; `bash -n` clean
- [ ] **T5 (P2, human: ~15min / CC: ~3min)** — tests — Write IT4/IT6/IT7/IT8/IT9 grep tests + AT5/AT6 manual procedure docs
  - Surfaced by: Section 3 (4 test gaps + apex regression AT6)
  - Files: `tests/{tunnel-config,nginx-content,probes,cloudflared-args,tf-dns}_test.sh`
  - Verify: each `bash tests/<name>_test.sh` exits 0 against the new manifests
- [ ] **T6 (P3, human: ~10min / CC: ~2min)** — Makefile — Add `k8s-validate`, `tf-validate`, `tunnel-health`, `tunnel-dns-check` targets; DO NOT add `server-build`
  - Surfaced by: Step 0 D1=A (no Go harness build target)
  - Files: `Makefile`
  - Verify: `make help` lists new targets and omits `server-build`
- [ ] **T7 (P3, human: ~20min / CC: ~5min)** — docs — Write cloudflare-tunnel-setup.md with apply procedure, Vault token rotation, rollback procedure (especially apex-preservation guarantee)
  - Surfaced by: Step 0 D2=C + D3=B + apex preservation guarantee (AT6)
  - Files: `docs/cloudflare-tunnel-setup.md`
  - Verify: markdownlint optional; manual review

---

## GSTACK REVIEW REPORT

| Review | Trigger | Why | Runs | Status | Findings |
|--------|---------|-----|------|--------|----------|
| Eng Review | `/plan-eng-review` | Architecture & tests (required) | 1 | CLEAR (PLAN) | 3 scope reductions accepted (D1=A drop Go harness, D2=C remote-managed config, D3=B subdomain over apex); 4 test gaps added (IT6 nginx content, IT7 probes, IT8 cloudflared args, IT9 subdomain DNS); AT6 apex-regression added; plan reduced 12 steps → 7 steps, ~25 files → ~14 files |
| CEO Review | `/plan-ceo-review` | Scope & strategy | 0 | — | — |
| Codex Review | `/codex review` | Independent 2nd opinion | 0 | skipped | D4=B — user opted to skip outside voice |
| Design Review | `/plan-design-review` | UI/UX gaps | 0 | — | N/A — infra plan |
| DX Review | `/plan-devex-review` | Developer experience gaps | 0 | — | — |

**UNRESOLVED:** 0
**VERDICT:** ENG CLEARED — drift class (two-source tunnel config) eliminated; apex blast radius eliminated; throwaway Go code eliminated. Awaiting human flip of `status: draft → approved` + `approved_by` + `approved_at` before S0 commits.
