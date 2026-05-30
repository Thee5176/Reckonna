# Plan 02 — Postgres on Kubernetes, Exposed via Tailscale (Tailnet-Only)

status: draft  <!-- flip to `status: approved` after human review; require-prereq.sh greps this -->

**Source of truth** for deploying Postgres on the existing remote Kubernetes cluster and exposing
its endpoint only to machines on our Tailscale tailnet. Closes the runtime-Vault gap deferred from
plan 00 by wiring the Vault Agent Injector and the Tailscale Operator side-by-side. **No `terraform
apply`, no `kubectl apply`, no `helm install` in this plan** — those are human-only per
`devops.md`. The deliverables here are: manifests, Terraform, helper script, docs.

## Decisions (locked, 2026-05-28)

| # | Decision | Rationale |
|---|----------|-----------|
| 1 | Postgres runs as a `StatefulSet` in an existing remote k8s cluster (not docker-compose, not managed). | Single source of truth with the production topology; docker-compose stays as the local dev stack. |
| 2 | Exposure is **tailnet-only** via the Tailscale Kubernetes Operator. | Tailscale Funnel does not expose TCP/5432; the cluster has no public LB; raw public Postgres is a security non-starter. |
| 3 | The original "make endpoint available via public internet" framing is dropped. | TCP/5432 + zero-trust auth needs Cloudflare Tunnel TCP; user opted for tailnet-only. Add a follow-up plan if public access is later required. |
| 4 | Vault Agent Injector renders `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_DB` from `secret/app/database` at pod start. No plaintext in any manifest. | Enforces `secrets-vault.md`. |
| 5 | Tailscale OAuth client + tags live in Vault at `secret/app/tailscale/operator`. | Same policy. |
| 6 | One step = one commit = one `Plan: S<n>` trailer. Conventional Commits. | `devops.md`. |

## File structure

```
plans/02-infra-postgres-tailnet.md      # this file
infra/
  k8s/
    postgres/
      namespace.yaml                    # ns + vault-injector label
      serviceaccount.yaml               # SA + Vault role binding annotations
      service.yaml                      # ClusterIP + tailscale exposure annotations
      statefulset.yaml                  # pg17, vault-agent sidecar/init, PVC template
      pdb.yaml                          # PodDisruptionBudget minAvailable=1
      networkpolicy.yaml                # deny all + allow tailscale-operator ns
      kustomization.yaml                # kustomize base
    tailscale/
      operator-values.yaml              # Helm values stub (oauth from Vault path, not literal)
      kustomization.yaml                # references upstream operator manifest by version
  terraform/
    postgres.tf                         # k8s_namespace + storageclass binding
    tailscale.tf                        # tailscale provider stub + vault data sources
scripts/
  pg-endpoint.sh                        # resolve tailnet hostname/IP for the PG service
  tailnet-smoke.sh                      # non-destructive reachability probe (psql -c 'SELECT 1')
docs/
  postgres-tailnet.md                   # how-to: setup + connect (Diataxis how-to)
Makefile                                # + k8s-validate, tf-validate, tailnet-smoke, pg-endpoint
```

---

## Section 1 — Acceptance-test spec (E2E)

Acceptance tests are **manual** in this plan — they require a live cluster and a tailnet-joined
host, which the plan does not provision. They become CI once a non-prod cluster lands.

| ID  | Given / When / Then | Domain | Test artifact |
|-----|---------------------|--------|---------------|
| AT1 | Given a host on the tailnet with `tailscale` up / When run `scripts/pg-endpoint.sh` / Then it prints a `*.ts.net` MagicDNS hostname **and** a `100.x.y.z` tailnet IP, exit 0. | infra | `scripts/pg-endpoint.sh` + `tests/pg-endpoint_test.sh` (dry-run with a fake `tailscale` shim) |
| AT2 | Given the same host / When run `scripts/tailnet-smoke.sh` / Then `psql -h <hostname> -U app -d accounting -c 'SELECT 1'` returns `1`. | infra | `scripts/tailnet-smoke.sh` |
| AT3 | Given a host **not** on the tailnet / When `psql -h <hostname>` is attempted / Then the connection fails (no DNS / no route). | infra | manual; documented in `docs/postgres-tailnet.md` |

## Section 2 — Integration-test spec

| ID  | Condition to verify | Domain | Test artifact |
|-----|---------------------|--------|---------------|
| IT1 | Every manifest under `infra/k8s/**` passes `kubeconform -strict` against k8s 1.30. | infra | `make k8s-validate` |
| IT2 | `terraform validate` is green for `infra/` + new `infra/terraform/*.tf`. | infra | `make tf-validate` |
| IT3 | `networkpolicy.yaml` denies pods outside the `tailscale` and `vault` namespaces from reaching `app/component=postgres`. | infra | manifest review + `kube-score`/`polaris` (optional) |
| IT4 | No literal secret value appears in any committed file under `infra/k8s/**` or `infra/terraform/**`. `gitleaks` clean. | infra | `gitleaks detect --no-git -s infra/` |
| IT5 | The PG `Service` has annotations `tailscale.com/expose: "true"` and `tailscale.com/hostname: pg-reckonna`. | infra | grep over `service.yaml` (`tests/service-annotations_test.sh`) |
| IT6 | The PG `StatefulSet` declares Vault Agent annotations rendering `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_DB` from `secret/app/database`. | infra | grep over `statefulset.yaml` (`tests/vault-injector_test.sh`) |
| IT7 | `scripts/pg-endpoint.sh` is idempotent and exits non-zero with a clear message when `tailscale` is missing or the operator hostname is not yet propagated. | infra | `tests/pg-endpoint_test.sh` |

## Section 3 — Implementation steps (one commit each)

Each step compiles/validates on its own. RED tests are committed before the manifest that satisfies them where TDD applies. For pure manifest steps without behavioural code, the verifier is the static check (kubeconform / terraform validate / grep test).

| ID | Commit (verbatim) | Files | Verify |
|----|-------------------|-------|--------|
| S0 | `docs(plan): infra plan 02 — postgres on k8s via tailscale (tailnet-only)` | `plans/02-infra-postgres-tailnet.md` | review only |
| S1 | `chore(k8s): postgres namespace + kustomization base` | `infra/k8s/postgres/namespace.yaml`, `infra/k8s/postgres/kustomization.yaml` | `kubeconform -strict` |
| S2 | `feat(k8s): postgres service + statefulset with vault-injector annotations` | `infra/k8s/postgres/service.yaml`, `infra/k8s/postgres/statefulset.yaml`, `infra/k8s/postgres/serviceaccount.yaml`, `infra/k8s/postgres/pdb.yaml` | `kubeconform`; grep tests IT5/IT6 |
| S3 | `feat(k8s): networkpolicy — deny-all + allow tailscale-operator + vault` | `infra/k8s/postgres/networkpolicy.yaml`, `tests/networkpolicy_test.sh` | `kubeconform`; IT3 |
| S4 | `feat(k8s): tailscale operator helm values + service annotations` | `infra/k8s/tailscale/operator-values.yaml`, `infra/k8s/tailscale/kustomization.yaml` | `kubeconform`; IT5 |
| S5 | `feat(infra): terraform stubs — k8s namespace + tailscale provider` | `infra/terraform/postgres.tf`, `infra/terraform/tailscale.tf` | `terraform validate` |
| S6 | `feat(scripts): pg-endpoint.sh + tailnet-smoke.sh` | `scripts/pg-endpoint.sh`, `scripts/tailnet-smoke.sh`, `tests/pg-endpoint_test.sh` | shellcheck; `tests/pg-endpoint_test.sh` |
| S7 | `chore(make): k8s-validate, tf-validate, tailnet-smoke, pg-endpoint` | `Makefile` | `make help` lists targets; `make k8s-validate` skips cleanly when `kubeconform` missing |
| S8 | `docs(infra): how to set up tailscale + connect to postgres` | `docs/postgres-tailnet.md` | manual review; markdownlint optional |
| S9 | `feat(scripts): pg-probe.sh + docs app-integration section + make pg-probe` | `scripts/pg-probe.sh`, `tests/pg-probe_test.sh`, `docs/postgres-tailnet.md`, `Makefile` | `tests/pg-probe_test.sh`; `make help` lists `pg-probe`; shellcheck |

### Step notes

- **S2 — Vault Agent Injector annotations.** Use `vault.hashicorp.com/agent-inject: "true"`, `vault.hashicorp.com/role: reckonna-postgres`, and per-template annotations rendering `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_DB` from `secret/data/app/database`. The injector mounts them at `/vault/secrets/db.env` and the container `sources` that into the entrypoint. **No secret value is committed** — only Vault paths.
- **S3 — NetworkPolicy.** Default-deny ingress to the `postgres` namespace. Allow ingress from pods labelled `app.kubernetes.io/name=tailscale-operator` in namespace `tailscale` on TCP 5432 (so the operator's proxy pod can reach PG). Allow egress to the Vault namespace on 8200.
- **S4 — Tailscale Operator.** This plan does **not** install the operator (helm install is human-only). It commits the values file that points the operator at OAuth client + secret read from Vault at `secret/app/tailscale/operator`. The PG `Service` annotation `tailscale.com/hostname: pg-reckonna` controls MagicDNS naming.
- **S5 — Terraform.** Adds the `tailscale` and `kubernetes` providers and a `kubernetes_namespace` resource for the `postgres` ns. Secrets via `data "vault_kv_secret_v2"` only; no `.tfvars`.
- **S6 — Endpoint script.** `scripts/pg-endpoint.sh` resolves the PG endpoint two ways: (a) `tailscale status --json | jq` for the `pg-reckonna` device (works from any tailnet host) and (b) `kubectl get service pg-postgres -n postgres -o jsonpath='{.metadata.annotations.tailscale\.com/hostname}'` (works from any kubeconfig). Prints both hostname (`pg-reckonna.<tailnet>.ts.net`) and IP. Tests use a fake `tailscale` shim in `PATH`.
- **S7 — Makefile.** Targets skip cleanly when their tool is absent (CI installs them; dev boxes may not).
- **S8 — Docs.** Diataxis how-to: prerequisites, mint Tailscale OAuth, store in Vault, install operator (operator side), client setup (tailscale up, MagicDNS), connect (psql, sqlc, migrate, GoLand, Beekeeper), troubleshooting, security model, off-tailnet behaviour.
- **S9 — App-side verification.** Adds `scripts/pg-probe.sh` for application teams who already have credentials in their runtime env (no Vault CLI dependency). Probe reads libpq env vars (`PGHOST`, `PGUSER`, `PGPASSWORD`, `PGDATABASE`, `PGSSLMODE`) and walks DNS → TCP → query, exiting with a stage-specific code (3 DNS / 4 TCP / 5 TLS / 6 AUTH / 7 DB / 8 query). Resolves DNS once and TCP-probes the resolved IP so the stage layers cleanly. Docs §2A documents per-stack driver call (pgx/psycopg/pg/pgjdbc/sqlx), headless tailnet onboarding via ephemeral auth keys, and the exit-code → fix table. `make pg-probe` wires it into the local CLI. Offline test stubs `psql` + `getent` and spins a throwaway TCP listener — no real PG and no network egress.

## Hand-off to the heads

- **infra-engineer (HEAD):** owns every step. Writes IT1–IT7 as failing checks first where applicable (S2/S3/S5/S6), then green via `iac-ops` → `code-reviewer`.
- **Human:** runs `helm install`, `kubectl apply -k infra/k8s/postgres`, `kubectl apply -k infra/k8s/tailscale`, `terraform apply`. Not in this plan.
- "Done" = AT1–AT3 documented; IT1–IT7 green; `docs/postgres-tailnet.md` reviewed; plan-tracker logs to `02-infra-postgres-tailnet.impl.md`.

## Known gaps (deferred)

- No public-internet exposure of PG (intentional — see Decision 3). If later required, add Plan 03 for Cloudflare Tunnel TCP + Cloudflare Access SSO.
- No PG backups / WAL archiving. Deferred to Plan 04 (backup/DR).
- No PG HA (single replica). Deferred to Plan 05 (Patroni or Spilo).
- No managed-cert / pg-bouncer. Deferred.
