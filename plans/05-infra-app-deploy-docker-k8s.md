---
feature: 05-infra-app-deploy-docker-k8s
status: draft   # require-prereq.sh greps this — flip to approved via /plan-eng-review + human sign-off
approved_by:
approved_at:
domain: infra
depends_on:
  - 00-bootstrap-deps-vault          # go.mod, Makefile, docker-compose, gitleaks, Vault wiring
  - 01-infra-postgres-tailnet        # tailnet-exposed Postgres (pg-reckonna) — the migrate Job + services' DATABASE_URL target
  - 02-infra-k8s-cloudflare-tunnel   # reckonna-app namespace + cloudflared tunnel + remote-managed ingress (this plan repoints it)
  - 03-backend-cqrs-core             # PUBLISHES ghcr images + OWNS the route contract (api/openapi.yaml); this plan DEPLOYS them
soft_depends_on:
  - 04-design-component-implementation  # supplies the RN-Web bundle + the web IMAGE BUILD (Dockerfile.web + CI job) — deferred there
unblocks:
  - 06-infra-otel-telemetry          # OTel scrape/spans need the app actually running in-cluster
decisions:
  surface: three tiers — reckonna-command (write), reckonna-query (read), reckonna-web (RN-Web static). Single evolved reckonna-app base. Web TOPOLOGY lands here; web IMAGE BUILD is deferred to plan 04 (no RN-Web source yet).
  images: consume plan 03 ghcr command/query images (SHA-pinned, never :latest); plan 05 builds the reckonna-migrate image; reckonna-web image is built by plan 04.
  registry_auth: PUBLIC ghcr — packages are world-readable, SHA-pinned, NO imagePullSecret. The images carry only the compiled distroless Go binary (no secrets baked in; runtime secrets come from Vault), so public is acceptable and keeps cluster wiring simple.
  secret_mechanism: runtime app env synced Vault to k8s by the Vault Secrets Operator (VaultStaticSecret CR renders reckonna-app-env; envFrom consumes it — distroless-safe, no shell needed). VSO operator install + the reckonna-app VaultAuth role are documented human-only prereqs. NOT committed.
  ingress: path-based on the SINGLE reckonna.thee5176.com CNAME, matching plan 03 api/openapi.yaml — /command/* to command:8080, /query/* to query:8080, /* to reckonna-web:80, http_status:404 catch-all. Health is /command/health + /query/health (NO /healthz). No new DNS.
  cutover: TWO-PHASE — stand up command/query/web Services alongside the plan-02 harness, repoint the tunnel (Terraform), THEN remove the harness. The tunnel target always exists so there is no 502 window.
  migrations: BAKED digest-pinned migrations image (build/Dockerfile.migrate COPYs db/migration; FROM migrate/migrate@sha256) run by a Job. NOT ConfigMap-mounted SQL (ConfigMap 1MB ceiling + SQL-as-config). Policy — expand/contract only, NO automatic down migrations; app rollback = redeploy prior image (schema is additive so it is always safe).
  db_access: DATABASE_URL points at plan-01 tailnet-exposed Postgres (pg-reckonna) via a tailscale EGRESS Service (operator-created) — the connection originates from the tailscale namespace, which plan-01 netpol ALREADY allow-lists, so plan-01 policy is NEVER edited/reapplied and no additive policy is needed (sidesteps the n8n-clobber risk entirely).
  init_ordering: command/query initContainer polls schema_migrations for the expected version (not just DB TCP) over the tailnet DATABASE_URL using a psql-capable image (postgres:17-alpine), so the app never starts against an unmigrated schema regardless of apply order.
  local_stack: docker-compose FULL stack — postgres + migrate + command + query + a reverse proxy (caddy) mirroring the tunnel path routes (true parity); web behind a full profile.
  smoke_auth: AT4/AT5/AT10 mint a JWT via a dedicated Keycloak reckonna-smoke client (client-credentials grant); creds in Vault secret/app/reckonna/smoke-oidc, read at CI time. No human token in CI.
  deploy_mechanism: raw kustomize + human kubectl apply (devops.md — kubectl apply/delete + terraform apply are human-only). No GitOps CD controller in v1 (VSO is a secrets operator, not a CD controller).
  human_only: terraform apply, kubectl apply/delete, kubectl delete job/reckonna-migrate before redeploy, VSO operator install + reckonna-app VaultAuth role — manifests + tf + scripts only in this plan.
review_log:
  - draft authored 2026-07-04 via /plan; 4 developer decisions locked (surface=backend+web, local=full-stack, registry, ingress).
  - /plan-eng-review 2026-07-06 (mode SCOPE_REDUCED); web image build deferred to plan 04 (D2). Codex outside voice ran (gpt-5.5, ready). 8 findings folded — (A1 verified) ingress reconciled to /command·/query·/* per api/openapi.yaml + health /command|/query/health; (A3) init waits schema version; (A4) migrate delete-before-apply; (CQ2) conservative .dockerignore + real build test; (auth) unauth to 401 AT; (codex) two-phase cutover, baked migrations image + expand/contract rollback, busybox to postgres init image, label-pollution fix, JWT-for-smoke client, compose reverse-proxy parity; (netpol) connectivity gate.
  - decision-by-decision confirmation 2026-07-06 — 10 KEEP, 3 CHANGE — D4 to PUBLIC ghcr (drop pull secret); D5 to Vault Secrets Operator (auto-sync reckonna-app-env); D9 to tailnet PG endpoint via a tailscale egress Service (drop the additive NetworkPolicy). Pending human approval.
---

# Plan 05 — Reckonna App Deployment (Docker + Kubernetes): command · query · web on homelab k3s

Turns the plan-02 **throwaway nginx harness** into the **real running application**. Plan 03
builds the command/query images (`build/Dockerfile.{command,query}` to `ghcr.io`) and OWNS the
route contract (`api/openapi.yaml`), but explicitly defers ALL `infra/k8s` app deployment; this
plan is that cutover:

1. **Docker** — a full local stack (`docker compose`): postgres + a one-shot golang-migrate +
   `command` + `query` + a `caddy` reverse proxy that mirrors the tunnel path routes, with the
   RN-Web tier behind a `full` profile. `make up` runs the whole app on the SAME path surface as prod.
2. **Kubernetes** — three Deployments (`reckonna-command`, `reckonna-query`, `reckonna-web`) + a
   baked-image migrate `Job`, reaching plan-01 Postgres over the tailnet, pulling SHA-pinned images
   from a **public** ghcr, with runtime config synced from **Vault** by the Vault Secrets Operator.
3. **Ingress cutover** — the remote-managed Cloudflare Tunnel config (plan 02, Terraform-owned) is
   repointed to **path-based** routing matching `api/openapi.yaml`, via a **two-phase** cutover so
   the tunnel target always exists (no 502 window).

**No `terraform apply`, no `kubectl apply`, no VSO/operator install in this plan** — those are
human-only per `devops.md`. Deliverables: Dockerfile(s), compose, k8s manifests, Terraform edit,
helper scripts, grep/validate tests, docs.

**Prerequisites:** Plans 00 + 01 + 02 landed (Vault, tailnet-exposed Postgres, tunnel +
`reckonna-app` namespace). Plan 03 image-publish pipeline landed (command/query images in ghcr) and
`api/openapi.yaml` is the route contract this plan ingress mirrors. The RN-Web bundle + its image
build (plan 04) are a **soft** dependency — the web manifests + placeholder land now; the web IMAGE
lands with plan 04. The Vault Secrets Operator + the `reckonna-app` VaultAuth role are a human-only
cluster prereq (documented in S13).

## Decisions (locked at draft 2026-07-04; hardened by /plan-eng-review + confirmation 2026-07-06)

| # | Decision | Rationale |
|---|----------|-----------|
| D1 | **Three tiers in ONE evolved base** (`infra/k8s/reckonna-app/`): the plan-02 nginx harness becomes `reckonna-web`; add `reckonna-command`, `reckonna-query`, `reckonna-migrate`. | Least churn; the Makefile `k8s-validate` loop + cloudflared ingress already know `reckonna-app`. |
| D2 | **Web TOPOLOGY here; web IMAGE BUILD deferred to plan 04.** `reckonna-web` Deployment/Service/placeholder + its ingress route land now; `build/Dockerfile.web` + the CI web-image job move to plan 04. | Building an image pipeline that skips itself is speculative infra. The topology CAN be finished now; the bundle build is hard-blocked. |
| D3 | **Consume plan 03 ghcr images; build the migrate image; SHA-pinned, never `:latest`.** kustomize `images:` pins command/query/web/migrate. | Immutable, reproducible, rollback = repin. |
| D4 | **PUBLIC ghcr.** Packages are world-readable, SHA-pinned; NO `imagePullSecret`. | The images hold only the compiled distroless Go binary — no secrets are baked in (runtime secrets arrive from Vault). Public keeps cluster wiring simple and removes a rotating pull credential. Developer confirmation 2026-07-06 (changed from private). |
| D5 | **Runtime app env via the Vault Secrets Operator.** A `VaultStaticSecret` CR syncs `secret/app/reckonna/*` (DB URL, OIDC issuer/JWKS/audience) into the `reckonna-app-env` Secret; pods `envFrom` it. | Auto-sync beats a human `kubectl create secret`; VSO produces a normal k8s Secret so `distroless/static:nonroot` (no shell) consumes it via `envFrom` without any injector/env-file contract. Developer confirmation 2026-07-06 (changed from human-applied). Operator install + the `reckonna-app` VaultAuth role are human-only prereqs. |
| D6 | **Ingress matches `api/openapi.yaml` exactly:** `/command/*` to `reckonna-command:8080`, `/query/*` to `reckonna-query:8080`, `/*` to `reckonna-web:80`, `http_status:404` last. Health = `/command/health` + `/query/health` (there is NO `/healthz`). | The website owns root; each API answers only its own prefix. Verified against plan 03 OpenAPI during eng-review; confirmed 2026-07-06. |
| D7 | **TWO-PHASE cutover:** (1) apply command/query/web Services alongside the live harness Service; (2) `terraform apply` repoints the tunnel; (3) remove the harness Service. | The tunnel target must never point at a deleted Service. Two-phase removes the 502 window. |
| D8 | **Migrations = baked digest-pinned image + expand/contract policy.** `build/Dockerfile.migrate` run by a Job. Every migration backward-compatible; **NO automatic down migrations**; app rollback = redeploy prior image. | ConfigMap-mounted SQL hits the 1MB ceiling + makes schema mutable cluster config. Expand/contract makes app rollback safe after a schema change. |
| D9 | **DB reach via the tailnet.** `DATABASE_URL` targets plan-01 tailnet-exposed `pg-reckonna` through a **tailscale egress Service** (operator-created); the connection originates from the tailscale namespace, which plan-01 netpol ALREADY allow-lists. | No edit/reapply of plan-01 NetworkPolicy (memory: a blind reapply once cut n8n DB access) and no additive policy. Developer confirmation 2026-07-06 (changed from an additive netpol). ⚠ Verify the operator can create the egress Service in `reckonna-app`; fall back to an additive netpol if not. |
| D10 | **initContainer waits for the schema version** over the tailnet `DATABASE_URL`, using `postgres:17-alpine` (has `psql`; busybox has no `pg_isready`). | App never boots against an unmigrated schema regardless of apply order. |
| D11 | **Local parity via a compose reverse proxy.** The `full` profile adds `caddy` mirroring `/command`,`/query`,`/`. | Hitting `localhost:8080` direct does not prove Cloudflare path routing. |
| D12 | **Smoke JWT via a dedicated Keycloak `reckonna-smoke` client** (client-credentials); creds in Vault `secret/app/reckonna/smoke-oidc`, read at CI time. | AT4/AT5/AT10 need a token with no human in the loop. |
| D13 | **Raw kustomize + human `kubectl apply`.** No ArgoCD/Flux CD controller in v1 (VSO is a secrets operator, not CD). One step = one commit + `Plan: S<n>`. | `devops.md`. GitOps CD is a later infra plan. |

## File structure

```
plans/05-infra-app-deploy-docker-k8s.md      # this file
build/
  Dockerfile.migrate                         # NEW — FROM migrate/migrate@sha256 + COPY db/migration /migrations (baked, digest-pinned)
  # Dockerfile.command / Dockerfile.query    # from plan 03 — NOT re-authored
  # Dockerfile.web                            # DEFERRED to plan 04 (needs app/ source)
.dockerignore                                # NEW — CONSERVATIVE denylist (node_modules, .git, .claude, graphify-out, *.md, tests) — NEVER cmd/ internal/ config/ locales/ go.*
docker-compose.yml                           # UPDATED — add migrate + command + query + caddy proxy (full profile); web under full profile
infra/k8s/reckonna-app/                       # EVOLVED base (was the nginx harness)
  namespace.yaml                             # keep
  command-deployment.yaml                    # NEW — reckonna-command; public ghcr image; envFrom reckonna-app-env; init schema-wait; probes /command/health:8080
  command-service.yaml                       # NEW — ClusterIP reckonna-command:8080
  query-deployment.yaml                      # NEW — reckonna-query; probes /query/health:8080
  query-service.yaml                         # NEW — ClusterIP reckonna-query:8080
  migrate-job.yaml                           # NEW — Job runs the baked reckonna-migrate image; envFrom reckonna-app-env
  web-deployment.yaml                        # NEW — reckonna-web (nginx + placeholder now; RN-Web bundle post plan 04); supersedes harness deployment.yaml
  web-service.yaml                           # NEW — ClusterIP reckonna-web:80
  web-content-configmap.yaml                 # placeholder index until plan 04 bundle (evolved from harness configmap.yaml)
  web-nginx-configmap.yaml                   # SPA-fallback nginx conf (evolved from configmap-nginx.yaml)
  configmap-app.yaml                         # NEW — NON-secret app env: OTEL endpoint, GIN_MODE=release, log level, RECKONNA_LOCALES_DIR
  vault-secret.yaml                          # NEW — VSO VaultAuth + VaultStaticSecret syncing secret/app/reckonna/* to the reckonna-app-env Secret (name refs only)
  pg-egress-service.yaml                     # NEW — tailscale egress Service to plan-01 tailnet Postgres (the DATABASE_URL host)
  kustomization.yaml                         # UPDATED — new resources, images: SHA pins (command/query/web/migrate), PER-WORKLOAD component labels (no web-harness pollution)
  # DELETED (phase 3 of cutover): deployment.yaml (nginx), service.yaml (:80), configmap.yaml (static), configmap-nginx.yaml
  # NOT committed: the reckonna-app-env Secret is materialized by VSO from Vault, never in-repo
# infra/k8s/postgres/ — UNTOUCHED. Plan-01 NetworkPolicy is NOT edited; DB reach is via the tailnet egress Service above.
infra/terraform/
  cloudflare.tf                              # UPDATED — tunnel ingress: /command/* to command:8080 ; /query/* to query:8080 ; /* to web:80 ; http_status:404
scripts/
  smoke-token.sh                             # NEW — mints a JWT via the reckonna-smoke client-credentials grant (creds from Vault)
  app-health.sh                              # NEW — curl /command/health + /query/health + / (web) via the tunnel
  compose-smoke.sh                           # NEW — compose up (full) to wait healthy to drive AT4/AT5 through the caddy proxy to down
tests/
  image-pins_test.sh                         # every app image (command/query/web/migrate) SHA-pinned; NO :latest; kustomize images: present
  app-secrets-wiring_test.sh                 # command/query/migrate envFrom secretRef reckonna-app-env (name only); NO imagePullSecrets needed (public ghcr)
  vault-secret_test.sh                       # vault-secret.yaml VaultStaticSecret targets reckonna-app-env by name; no literal secret value
  app-probes_test.sh                         # command probes /command/health:8080; query probes /query/health:8080; web /:80
  migrate-job_test.sh                        # Job runs the baked migrate image with up; NO ConfigMap SQL; NO literal DATABASE_URL
  init-schemawait_test.sh                    # command/query initContainer polls schema_migrations version (NOT busybox/pg_isready-only)
  tunnel-routes_test.sh                      # tf ingress: /command/ , /query/ rules, then /* to web default, 404 LAST; correct svc FQDNs
  pg-egress_test.sh                          # pg-egress-service.yaml is a tailscale egress Service; manifests reference it (NOT postgres.postgres.svc); plan-01 netpol not referenced/edited
  labels_test.sh                             # each workload carries a distinct app.kubernetes.io/component; no shared web-harness label across selectors
  dockerignore-safety_test.sh                # .dockerignore never excludes cmd/ internal/ config/ locales/ go.mod go.sum
  compose-config_test.sh                     # docker compose config valid; command/query build from build/Dockerfile.*; depends_on migrate + postgres; caddy in full profile
  no-app-secrets_test.sh                     # gitleaks/grep: no literal DB URL, OIDC/smoke secret in infra/**, compose, tests, scripts
docs/
  app-deploy.md                              # NEW — VSO+role bootstrap, two-phase cutover order, expand/contract rollback, image repin, smoke JWT
Makefile                                     # + compose-smoke, app-health, smoke-token targets; + docker-build check for command/query
.github/workflows/ci.yml                     # + reckonna-migrate image build+push job (baked migrations image to ghcr)
```

**Evolved from the plan-02 harness (not greenfield):** `deployment.yaml`(nginx) to `web-deployment.yaml`;
`service.yaml`(:80) to `web-service.yaml`; `configmap.yaml`(static) to `web-content-configmap.yaml`;
`configmap-nginx.yaml` to `web-nginx-configmap.yaml` (SPA fallback). Namespace unchanged.

---

## Section 1 — Acceptance-test spec (E2E)

E2E tests are **manual until** images are pushed and the human applies the VSO role + manifests +
Terraform (`kubectl apply` human-only). Once live, the smoke scripts run from CI on a schedule and
mint their JWT via the `reckonna-smoke` client (D12). AT4/AT5/AT6 exercise the **domain invariant
through the fully deployed stack** — the deploy is not "done" until an unbalanced entry is rejected.

| ID  | Given / When / Then | Domain | Test artifact |
|-----|---------------------|--------|---------------|
| AT1 | Given the VSO-synced `reckonna-app-env` Secret present + the tailscale egress Service up + SHA-pinned public images pushed / When `kubectl apply -k infra/k8s/reckonna-app` / Then `reckonna-command` + `reckonna-query` pods reach Ready (probes green, initContainer clears). | infra | manual; `kubectl get pods -n reckonna-app` |
| AT2 | Given a fresh/empty `accounting` schema / When the `reckonna-migrate` Job runs (baked image) / Then it completes `Succeeded` and every `db/migration/*.up.sql` is applied (no dirty version). | infra | manual; `kubectl logs job/reckonna-migrate -n reckonna-app` |
| AT3 | Given the stack deployed + tunnel live / When `curl -sf https://reckonna.thee5176.com/command/health` AND `.../query/health` / Then both HTTP 200. | infra+app | `scripts/app-health.sh` |
| AT4 | **Happy path (借方=貸方):** Given a JWT from reckonna-smoke / When `POST https://reckonna.thee5176.com/command/journal-entries` with a balanced entry (debit 1000 + credit 1000) / Then 201, entry recorded. | app (e2e) | `e2e/deploy-journal-entry.e2e` |
| AT5 | **Invalid (借方≠貸方):** Given the same JWT / When `POST /command/journal-entries` with an unbalanced entry / Then HTTP 422 `application/problem+json`, `code: unbalanced_entry` — rejected end-to-end. | app (e2e) | `e2e/deploy-journal-entry-invalid.e2e` |
| AT6 | **CQRS write to read across the two deployed pods:** Given AT4 recorded id `X` / When `GET https://reckonna.thee5176.com/query/journal-entries/X` (routed to `reckonna-query`) / Then it returns id `X` with `SUM(debit) == SUM(credit)`. | app (e2e) | `e2e/deploy-cqrs-roundtrip.e2e` |
| AT7 | Given the stack deployed / When `kubectl rollout restart deploy/reckonna-query -n reckonna-app` / Then AT3 `/query/health` passes again within 60s (RollingUpdate, ≥2 replicas). | infra | manual |
| AT8 | **Local parity:** Given `docker compose --profile full up -d` (postgres + migrate + command + query + caddy) / When migrate completes + services healthy / Then AT4 + AT5 pass against `http://localhost/command/journal-entries` **through the caddy proxy** (same path surface as the tunnel). | app | `scripts/compose-smoke.sh` |
| AT9 | **Two-phase cutover, no outage:** Given the plan-02 harness serving via the tunnel / When phase-1 (new Services applied) to phase-2 (`terraform apply` repoints ingress to `/command`,`/query`,`/*`) to phase-3 (harness removed) / Then `/command/*` to command, `/query/*` to query, `/` to web throughout, and NO request 502s at any phase. | infra | manual; documented in `docs/app-deploy.md` |
| AT10| **Auth gate on the deployed stack:** Given NO JWT (and separately an invalid JWT) / When `POST https://reckonna.thee5176.com/command/journal-entries` / Then HTTP 401 — proves the VSO-synced OIDC issuer/JWKS are wired on the live image. | app (e2e) | `e2e/deploy-auth-gate.e2e` |
| AT11| **Web route:** Given the web tier deployed / When `GET https://reckonna.thee5176.com/` / Then HTTP 200 served by `reckonna-web` (placeholder pre-plan-04; the RN-Web SPA post-plan-04). | infra+app | `scripts/app-health.sh` |

## Section 2 — Integration-test spec

Static/structural checks — run in CI + `make` without a live cluster.

| ID  | Condition to verify | Domain | Test artifact |
|-----|---------------------|--------|---------------|
| IT1 | `kubectl kustomize infra/k8s/reckonna-app` renders + passes `kubeconform -strict` (k8s 1.30). | infra | `make k8s-validate` |
| IT2 | `terraform validate` green for `infra/terraform/cloudflare.tf` (3-way ingress) + providers. | infra | `make tf-validate` |
| IT3 | No literal secret (DB URL/password, OIDC/smoke client secret) in any committed file under `infra/**`, `docker-compose.yml`, `tests/**`, `scripts/**`. | infra | `gitleaks`; `tests/no-app-secrets_test.sh` |
| IT4 | Every app image (command/query/web/migrate) SHA-pinned (`@sha256:` or full-SHA tag) via the kustomize `images:` block. NO `:latest`/floating. | infra | `tests/image-pins_test.sh` |
| IT5 | `command` + `query` + `migrate` use `envFrom: [{secretRef: {name: reckonna-app-env}}]` (name only — the Secret is materialized by VSO, not committed). NO `imagePullSecrets` (public ghcr). | infra | `tests/app-secrets-wiring_test.sh` |
| IT6 | `reckonna-command` probes `path: /command/health, port: 8080`; `reckonna-query` probes `path: /query/health, port: 8080`; `reckonna-web` probes `/` on 80. All HTTP, readiness+liveness. | infra | `tests/app-probes_test.sh` |
| IT7 | Tunnel ingress: rule `path ~ ^/command/` to `reckonna-command...:8080`, rule `path ~ ^/query/` to `reckonna-query...:8080`, then a default (no-path) rule to `reckonna-web...:80`, then `service = "http_status:404"` LAST. Order asserted. | infra | `tests/tunnel-routes_test.sh` |
| IT8 | `reckonna-migrate` Job runs the **baked** `ghcr.io/thee5176/reckonna-migrate@sha256` image with args `… up`; NO `configMapGenerator` of SQL; `DATABASE_URL` via `envFrom` (no literal). | infra | `tests/migrate-job_test.sh` |
| IT9 | `command` + `query` `initContainer` polls `schema_migrations` for the expected version over the tailnet `DATABASE_URL` using `postgres:17-alpine`, NOT busybox/`pg_isready`-only. | infra | `tests/init-schemawait_test.sh` |
| IT10| `docker compose config` valid; command/query build from `build/Dockerfile.command|query`; both `depends_on` `migrate` (completed) + `postgres` (healthy); `caddy` present under the `full` profile mirroring `/command`,`/query`,`/`. | infra | `tests/compose-config_test.sh` |
| IT11| `build/Dockerfile.migrate` is `FROM migrate/migrate@sha256:…` + `COPY db/migration /migrations`, and passes `hadolint`. `command` + `query` still `docker build` green under the new `.dockerignore` (real build, not just `compose config`). | infra | `hadolint`; `make docker-build-check` |
| IT12| `.dockerignore` NEVER excludes `cmd/`, `internal/`, `config/`, `locales/`, `go.mod`, `go.sum` (guards plan-03 image builds). | infra | `tests/dockerignore-safety_test.sh` |
| IT13| `pg-egress-service.yaml` is a tailscale egress Service (annotation `tailscale.com/tailnet-fqdn`); the app manifests' `DATABASE_URL`/host reference it, NOT `postgres.postgres.svc` in-cluster; plan-01 `networkpolicy.yaml` is neither referenced nor edited by this base. | infra | `tests/pg-egress_test.sh` |
| IT14| Each workload (command/query/web/migrate) carries a DISTINCT `app.kubernetes.io/component` label; no single `web-harness` component label is applied across all selectors (no selector pollution from `includeSelectors`). | infra | `tests/labels_test.sh` |
| IT15| `vault-secret.yaml` declares a VSO `VaultStaticSecret` whose destination is the `reckonna-app-env` Secret and whose mount/path is `secret/app/reckonna/*` (references only — no literal secret value). | infra | `tests/vault-secret_test.sh` |

## Section 3 — Implementation steps (one commit each)

Each step compiles/validates standalone (`kubeconform` / `terraform validate` / `hadolint` / grep).
Add a `Plan: S<n>` trailer to every commit.

| ID | Commit (verbatim) | Files | Verify |
|----|-------------------|-------|--------|
| S0 | `docs(plan): infra plan 05 — reckonna app deploy (command+query+web) on k3s + docker` | `plans/05-infra-app-deploy-docker-k8s.md` | review only |
| S1 | `chore(k8s): reckonna-command deployment + service (public image, schema-wait init, /command/health probes)` | `infra/k8s/reckonna-app/{command-deployment,command-service}.yaml` | `kubeconform`; IT1; IT6; IT9 grep |
| S2 | `chore(k8s): reckonna-query deployment + service (read-only, /query/health probes)` | `infra/k8s/reckonna-app/{query-deployment,query-service}.yaml` | `kubeconform`; IT1; IT6; IT9 grep |
| S3 | `feat(build): baked migrations image + reckonna-migrate Job` | `build/Dockerfile.migrate`, `infra/k8s/reckonna-app/migrate-job.yaml` | `hadolint`; `kubeconform`; IT8 + IT11 grep |
| S4 | `chore(k8s): evolve nginx harness into reckonna-web tier (deployment/service/configmaps)` | `infra/k8s/reckonna-app/{web-deployment,web-service,web-content-configmap,web-nginx-configmap}.yaml` | `kubeconform`; IT1; IT6 grep |
| S5 | `chore(k8s): image SHA-pins + VSO vault-secret + per-workload labels + app configmap` | `infra/k8s/reckonna-app/{kustomization,configmap-app,vault-secret}.yaml` | IT4 + IT5 + IT14 + IT15 grep; IT1 |
| S6 | `chore(build): conservative .dockerignore + docker-build check for command/query` | `.dockerignore`, `Makefile` (docker-build-check) | IT12; IT11 (real build) |
| S7 | `feat(infra): tunnel ingress — path-based command/query/web routing (two-phase cutover)` | `infra/terraform/cloudflare.tf` | `terraform validate`; gitleaks IT3; IT7 grep |
| S8 | `feat(k8s): tailscale egress service for tailnet postgres` | `infra/k8s/reckonna-app/pg-egress-service.yaml` | `kubeconform`; IT13 grep (plan-01 netpol untouched) |
| S9 | `feat(docker): full local stack — migrate + command + query + caddy path-proxy (web under full profile)` | `docker-compose.yml` | `docker compose config`; IT10 grep |
| S10| `feat(scripts): smoke-token + app-health + compose-smoke` | `scripts/{smoke-token,app-health,compose-smoke}.sh`, `tests/*_test.sh` | `shellcheck`; each `bash tests/<name>_test.sh` exits 0 |
| S11| `chore(make): compose-smoke, app-health, smoke-token targets` | `Makefile` | `make help` lists targets; skip cleanly when tools absent |
| S12| `ci(images): reckonna-migrate baked image build+push job` | `.github/workflows/ci.yml` | workflow lints; job builds + pushes on push-to-main |
| S13| `docs(infra): app deploy — VSO+role bootstrap, two-phase cutover, expand/contract rollback, repin` | `docs/app-deploy.md` | manual review |

### Step notes

- **S1/S2 — command/query pods.** `containerPort: 8080`. **No `imagePullSecrets`** (public ghcr, D4).
  `envFrom: [secretRef: reckonna-app-env]` (VSO-synced — DB URL, OIDC issuer/JWKS/audience) + `envFrom:
  [configMapRef: reckonna-app-config]` (OTEL endpoint, `GIN_MODE=release`, log level,
  `RECKONNA_LOCALES_DIR=/locales`). **initContainer `schema-wait`** = `postgres:17-alpine`, loops
  `psql "$DATABASE_URL" -tAc "select 1 from schema_migrations where version >= <expected> and not dirty"`
  over the tailnet `DATABASE_URL` until it returns a row (D10). Command probes `/command/health:8080`;
  query `/query/health:8080`. `replicas: 2`, RollingUpdate. **Pin CPU/mem requests AND limits** so a
  runaway pod cannot starve the shared homelab node.
- **S3 — baked migrations.** `build/Dockerfile.migrate`: `FROM migrate/migrate@sha256:<digest>` then
  `COPY db/migration /migrations`. Job runs it with `args: ["-path","/migrations","-database","$(DATABASE_URL)","up"]`,
  `DATABASE_URL` via `envFrom reckonna-app-env`, `backoffLimit: 1`, `restartPolicy: Never`,
  `ttlSecondsAfterFinished` set. **Re-run:** the Job name is fixed; a redeploy requires
  `kubectl delete job/reckonna-migrate` first (documented in S13). NO ConfigMap SQL (D8).
- **S4 — web tier.** The plan-02 harness Deployment becomes `reckonna-web`, still `nginx:alpine`
  PRE-plan-04 serving `web-content-configmap` (placeholder index at `/`); POST-plan-04 it serves the
  RN-Web `dist/` from `ghcr.io/thee5176/reckonna-web` (image built in plan 04). `web-nginx-configmap`
  gets an SPA `try_files $uri /index.html` fallback. Harness files are deleted in **phase 3** of the
  cutover (S13), not here.
- **S5 — pins + VSO + labels.** kustomize `images:` SHA-pins command/query/web/migrate. `envFrom` refs on
  each pod (no imagePullSecrets). `vault-secret.yaml` = a VSO `VaultAuth` (role `reckonna-app`) +
  `VaultStaticSecret` syncing `secret/app/reckonna/*` into `reckonna-app-env` (names/paths only, no
  values). **Per-workload `app.kubernetes.io/component`** (`command`/`query`/`web`/`migrate`) — drop
  the shared `web-harness` component label from `includeSelectors` (codex). `configmap-app` holds ONLY
  non-secret values.
- **S7 — ingress (Terraform, two-phase).** Edit plan 02
  `cloudflare_zero_trust_tunnel_cloudflared_config` ingress list to: `{path="^/command/", service=
  "http://reckonna-command.reckonna-app.svc.cluster.local:8080"}`, `{path="^/query/", service=
  "http://reckonna-query.reckonna-app.svc.cluster.local:8080"}`, `{service=
  "http://reckonna-web.reckonna-app.svc.cluster.local:80"}` (default), `{service="http_status:404"}`.
  Applied in **phase 2**. No new `cloudflare_record`.
- **S8 — tailscale egress Service.** A Service in the `reckonna-app` namespace with
  `annotations: tailscale.com/tailnet-fqdn: <pg-reckonna tailnet fqdn>` (operator creates an egress
  proxy pod). Pods set `DATABASE_URL` host to this Service; egress originates from the tailscale
  namespace, which plan-01 netpol already admits — so plan-01 policy is untouched (D9). ⚠ Verify the
  operator can create egress in `reckonna-app`; if not, fall back to an additive netpol.
- **S9 — compose full stack + proxy.** `migrate` (`depends_on postgres: service_healthy`),
  `command`/`query` (`build: build/Dockerfile.command|query`, `depends_on: {postgres: service_healthy,
  migrate: service_completed_successfully}`, env from the developer Vault-rendered shell — NEVER
  inlined), `caddy` under `profiles: [full]` reverse-proxying `/command/*` to command, `/query/*` to
  query, `/*` to web (D11). Compose postgres is LOCAL (not the tailnet) so the inner loop needs no VPN.
- **S10 — scripts.** `smoke-token.sh` does a client-credentials grant against Keycloak using
  `secret/app/reckonna/smoke-oidc`. `app-health.sh` hits `/command/health` + `/query/health` + `/`.
  `compose-smoke.sh` drives AT8 through caddy. (No ghcr-pull / app-env scripts — public ghcr + VSO.)
- **S13 — docs apply order (two-phase).** (1) install the Vault Secrets Operator + create the
  `reckonna-app` VaultAuth role + `vault kv put` `secret/app/reckonna/*` and `.../smoke-oidc` (human,
  once); (2) `kubectl apply` the `vault-secret.yaml` (VSO materializes `reckonna-app-env`) + the
  tailscale egress Service; (3) `kubectl delete job/reckonna-migrate` if present, then `kubectl apply`
  the migrate Job, await `Complete`; (4) `kubectl apply -k` the command/query/web Deployments +
  Services **alongside the live harness**; (5) `terraform apply` the ingress repoint; (6) remove the
  harness. **Rollback:** app = redeploy the prior SHA (safe — expand/contract, NO down migrations);
  ingress = `terraform apply` the prior config.

---

## Failure modes

| Codepath | Realistic failure | Test? | Error handling? | User visibility |
|----------|-------------------|-------|-----------------|-----------------|
| DB reachability | tailscale egress proxy / tailnet down | IT13 (egress Service present) | Connection to `DATABASE_URL` fails; initContainer blocks (visible, not silent) | `kubectl get pods` Init:0/1 stuck; `kubectl logs -c schema-wait` |
| VSO sync | `reckonna-app-env` not yet materialized / VaultAuth role wrong | AT1 (live); IT15 (CR present) | Pods stay pending on the missing Secret; VSO logs the auth error | `kubectl describe pod` (Secret not found); VSO operator logs |
| Public image pull | ghcr package unreachable / wrong SHA | AT1 (live); IT4 asserts the pin | Pod `ImagePullBackOff`; readiness never passes | `kubectl describe pod`; tunnel 502 for that route |
| App boots pre-migration | Deployments applied same time as Job | IT9 (schema-wait) | initContainer blocks on schema version, not just TCP | Init container waits; no crash against missing tables |
| Migrate Job re-run | Fixed-name Job immutable on 2nd apply | AT2 (live) | Documented `kubectl delete job/reckonna-migrate` before redeploy (S13) | 2nd apply errors `field is immutable` if the delete is skipped |
| Rollback after schema change | Old app image vs new schema | N/A (policy) | Expand/contract: migrations backward-compatible, NO auto-down so the prior image runs against the new schema | Rollback is safe by construction |
| Cutover outage | Tunnel points at a deleted Service | AT9 (two-phase) | Two-phase: new Services exist before repoint; harness removed after | No 502 if the phase order holds |
| Ingress rule mis-ordered | `/` default before `/command/` or `/query/` | IT7 asserts order; AT9 live | grep blocks at PR; cloudflared first-match wins | `/command/*` would wrongly hit web to 404 |
| `.dockerignore` over-excludes | Hides `cmd/`/`locales/` from the Go build | IT12 + IT11 real build | Denylist guards the paths; a real `docker build` fails CI if broken | Build fails in the plan-05 PR, not prod |
| Smoke has no JWT | AT4/AT5/AT10 cannot authenticate | D12 (`reckonna-smoke` client) | `smoke-token.sh` mints via client-credentials from Vault | Smoke fails loudly if the client/creds are missing |
| Label pollution | Shared `web-harness` component across selectors | IT14 | Per-workload component labels; no shared includeSelectors | Would surface as mis-selected Endpoints; caught by grep |

**No silent failures flagged.** Every mode has an observable symptom + a documented recovery path.

---

## Worktree parallelization strategy

| Step | Module | Depends on |
|------|--------|------------|
| S1 | infra/k8s/reckonna-app (command) | — |
| S2 | infra/k8s/reckonna-app (query) | — |
| S3 | build/ + infra/k8s/reckonna-app (migrate) | — |
| S4 | infra/k8s/reckonna-app (web) | — |
| S5 | infra/k8s/reckonna-app (kustomization/configmap/vault-secret) | S1–S4 |
| S6 | .dockerignore + Makefile | — |
| S7 | infra/terraform | — |
| S8 | infra/k8s/reckonna-app (pg egress) | — |
| S9 | docker-compose.yml | S3 (migrate image) |
| S10| scripts/, tests/ | S1–S9 |
| S11| Makefile | S6, S10 |
| S12| .github/workflows | S3 |
| S13| docs/ | all |

**Lanes:** S1, S2, S3, S4, S6, S7, S8 independent (parallel). S5 barrier-joins S1–S4 (shared
`kustomization.yaml`). S9 depends on S3, S12 depends on S3. S10 depends on S1–S9. S11 depends on
S6+S10. S13 last. `kustomization.yaml` (S3 + S5 + S8 all add resources) is the shared file — sequence
S3 and S8 before S5, or have S5 own all kustomization edits.

---

## Hand-off to the heads

- **infra-engineer (HEAD):** owns S0–S13. Writes IT1–IT15 + the AT smoke scripts as failing/grep
  tests FIRST, then green via `iac-ops` to `tdd-implementer` to `code-reviewer`. Installs the VSO +
  `reckonna-app` VaultAuth role, applies the egress Service + manifests + Terraform manually post-merge
  (human-only), following the two-phase cutover order in S13.
- **backend-engineer (HEAD):** NOT re-invoked — plan 03 ships command/query images + owns
  `api/openapi.yaml`. **Contract this plan mirrors (verified 2026-07-06):** command serves
  `/command/*`, query serves `/query/*`, health is `/command/health` + `/query/health`, 401 on any
  non-`/health` path. If plan 03 routes change, IT6/IT7 + the ingress must follow.
- **frontend-engineer (HEAD):** delivers the RN-Web bundle AND the web image build (`build/Dockerfile.web`
  + CI job) in plan 04, which flips `reckonna-web` from placeholder to the real SPA.
- **plan-tracker:** logs each landed step to `05-infra-app-deploy-docker-k8s.impl.md`.

**"Done" (plan 05)** = IT1–IT15 green; `make k8s-validate` + `make tf-validate` clean; `gitleaks`
clean; `docker compose config` valid; command/query `docker build` green under the new
`.dockerignore`. AT1–AT11 run manually post-apply (human-only); AT3/AT4/AT5/AT8/AT10/AT11 become CI
smoke once live. AT5 (借方≠貸方 rejected end-to-end) + AT10 (auth gate) are the gating checks.

## NOT in scope (plan 05)

- **command/query image builds** — plan 03 S17b owns `build/Dockerfile.{command,query}` + their ghcr
  jobs. Plan 05 pins + deploys them and builds only the migrate image.
- **RN-Web app source + web image build** — plan 04. Plan 05 lands the web topology + placeholder;
  `build/Dockerfile.web` + the CI web-image job are plan 04.
- **OTel collector / dashboards / scrape** — plan 06. Plan 05 only sets the OTLP endpoint env.
- **A reckonna-app NetworkPolicy or edits to plan-01's policy** — DB reach is via the tailnet egress
  Service (D9); no netpol is touched. App-side egress lockdown is a later plan.
- **GitOps CD (ArgoCD/Flux), HPA/autoscaling, PodDisruptionBudgets for the app tiers** — later infra.
  (VSO ships here as a secrets operator, not a CD controller.)
- **Keycloak provisioning** — plan 03 external prereq; plan 05 consumes issuer/JWKS + the
  `reckonna-smoke` client, it does not stand up Keycloak.
- **Cloudflare Access / edge auth** — in-app OIDC only (plan 03).

## What already exists

- Plan 00: `go.mod`, `Makefile` (empty-state guards + `k8s-validate`/`tf-validate`),
  `docker-compose.yml` (postgres only), gitleaks CI gate, Vault docs.
- Plan 01: tailnet-exposed Postgres (`pg-reckonna`) + its NetworkPolicy (admits tailscale namespace) —
  plan 05 reaches it via a tailscale egress Service and does NOT edit that policy.
- Plan 02: `reckonna-app` namespace + nginx harness (evolved here) + `cloudflared` + Terraform
  remote-managed tunnel (ingress edited here).
- Plan 03: `build/Dockerfile.{command,query}` (distroless, `EXPOSE 8080`) + ghcr publish, AND
  `api/openapi.yaml` — the authoritative route contract this plan ingress/probes mirror.
- `Makefile k8s-validate` already renders + `kubeconform`s the `reckonna-app` base.

---

## Implementation Tasks
Synthesized from this review findings. Each derives from a specific finding above.

- [ ] **T1 (P1, human: ~1h / CC: ~15min)** — ingress/health — Reconcile Terraform ingress + probes to `api/openapi.yaml` (`/command/*`,`/query/*`,`/*` to web; `/command/health`+`/query/health`)
  - Surfaced by: A1 cross-model + codex health-path finding
  - Files: `infra/terraform/cloudflare.tf`, `infra/k8s/reckonna-app/{command,query}-deployment.yaml`, `scripts/app-health.sh`
  - Verify: IT6 + IT7 grep; `terraform validate`
- [ ] **T2 (P1, human: ~45min / CC: ~10min)** — db-access — Tailscale egress Service to tailnet Postgres; DATABASE_URL targets it (plan-01 netpol untouched)
  - Surfaced by: eng-review connectivity finding + D9 confirmation (tailnet endpoint)
  - Files: `infra/k8s/reckonna-app/pg-egress-service.yaml`, `tests/pg-egress_test.sh`
  - Verify: IT13; `kubeconform`
- [ ] **T3 (P1, human: ~1h / CC: ~15min)** — cutover — Two-phase cutover structure + S13 apply order
  - Surfaced by: codex outage-window finding
  - Files: `docs/app-deploy.md`, `infra/terraform/cloudflare.tf`
  - Verify: AT9 procedure; manual
- [ ] **T4 (P1, human: ~1h / CC: ~15min)** — migrations — Baked migrate image + expand/contract policy + schema-wait init
  - Surfaced by: codex ConfigMap-fragility + rollback-safety; A3 init-ordering
  - Files: `build/Dockerfile.migrate`, `infra/k8s/reckonna-app/migrate-job.yaml`, `command/query` initContainers
  - Verify: IT8 + IT9 + IT11
- [ ] **T5 (P1, human: ~45min / CC: ~10min)** — secrets — VSO VaultStaticSecret for reckonna-app-env (public ghcr drops the pull secret)
  - Surfaced by: D4 (public ghcr) + D5 (Vault Secrets Operator) confirmation
  - Files: `infra/k8s/reckonna-app/vault-secret.yaml`, `tests/vault-secret_test.sh`, `tests/app-secrets-wiring_test.sh`
  - Verify: IT5 + IT15
- [ ] **T6 (P2, human: ~30min / CC: ~10min)** — dockerignore — Conservative denylist + real command/query build check
  - Surfaced by: CQ2
  - Files: `.dockerignore`, `Makefile` (docker-build-check), `tests/dockerignore-safety_test.sh`
  - Verify: IT11 + IT12
- [ ] **T7 (P2, human: ~30min / CC: ~10min)** — tests — Auth-gate AT + web-route AT + labels + smoke-token + caddy parity
  - Surfaced by: test-review auth gap; codex label pollution + JWT-for-smoke + local-parity
  - Files: `e2e/deploy-auth-gate.e2e`, `scripts/smoke-token.sh`, `tests/labels_test.sh`, `docker-compose.yml`, `scripts/compose-smoke.sh`
  - Verify: AT10/AT11; IT14; IT10/AT8

---

## GSTACK REVIEW REPORT

| Review | Trigger | Why | Runs | Status | Findings |
|--------|---------|-----|------|--------|----------|
| CEO Review | `/plan-ceo-review` | Scope & strategy | 0 | — | — |
| Codex Review | `/codex review` | Independent 2nd opinion | 1 | issues_found | 12 raised; 8 folded (ingress/health, /query prefix, cutover, busybox to postgres, baked-migrations, rollback policy, JWT-for-smoke, compose-proxy, label pollution), rest folded as notes |
| Eng Review | `/plan-eng-review` | Architecture & tests (required) | 1 | cleared | 9 findings folded; then a decision-by-decision confirmation set 10 KEEP + 3 CHANGE (D4 public ghcr, D5 VSO, D9 tailnet PG) |
| Design Review | `/plan-design-review` | UI/UX gaps | 0 | N/A | infra plan — web tier renders plan 04 approved design system |
| DX Review | `/plan-devex-review` | Developer experience gaps | 0 | — | — |

**CODEX:** gpt-5.5 (ready) found a contract mismatch the review missed — plan 05 used `/healthz` and `/*` to query, but `api/openapi.yaml` defines `/command/health`+`/query/health` and prefixed routes. Also flagged a guaranteed cutover outage, ConfigMap-SQL fragility, unsafe schema rollback, and the busybox/`pg_isready` impossibility. All folded.
**CROSS-MODEL:** Both reviewers independently concluded the draft `/*` to query ingress was wrong; reconciled against the OpenAPI spec (authoritative).
**VERDICT:** ENG CLEARED (pending human approval) — 9 review + 8 codex findings folded; 13 decisions confirmed (D4/D5/D9 changed). Contract matches `api/openapi.yaml`; DB reach via tailnet egress; secrets via VSO; public ghcr. Flip `status: draft` to `approved` + `approved_by` + `approved_at` before S0 commits. STEP 2 (design-system HTML) waived — infra plan; the web tier renders plan 04 approved `design/` system.

NO UNRESOLVED DECISIONS
