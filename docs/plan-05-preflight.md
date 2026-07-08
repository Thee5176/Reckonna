# Plan 05 — Deployment Preflight Checklist

Hard prerequisites that must be **true** before `plans/05-infra-app-deploy-docker-k8s.md` can
proceed to live execution (build → apply → cutover). Each gate is read-only-verifiable. Plan 05's
static ITs can pass on-branch without these, but every AT (live) is gated on the chain below.

**Last validated:** 2026-07-07 — **3 / 12 PASS → 🔴 NO-GO.** All secret paths reference Vault by
path only; never inline a secret value in this file.

## Gate status

| # | Gate | Check (read-only) | 2026-07-07 |
|---|------|-------------------|-----------|
| G1 | Backend (plan 03) landed to trunk | `git ls-tree -r origin/main --name-only \| grep -E '^(cmd\|internal)/.*\.go$\|^build/Dockerfile\.(command\|query)$'` | 🔴 unmerged (feat/03-backend-cqrs-core: 53 go + 2 Dockerfiles) |
| G2 | ghcr images published + pullable | `gh api /orgs/thee5176/packages/container/reckonna-command/versions` (and `-query`) | 🔴 never built — 404 "name unknown" |
| G3 | `api/openapi.yaml` route contract present | `git show feat/03-backend-cqrs-core:api/openapi.yaml \| grep -E '/command/\|/query/\|/command/health\|/query/health'` | 🟢 prefixed routes + both health paths (no /healthz) |
| G4 | Plans 01 + 02 approved + on trunk | `grep -m1 ^status: plans/0{1,2}*.md` + `git ls-tree -r origin/main --name-only \| grep infra/k8s` | 🔴 approved, manifests only on this branch |
| G5 | `reckonna-app` namespace exists + labeled | `kubectl get ns reckonna-app -o jsonpath='{.metadata.labels.kubernetes\.io/metadata\.name}'` | 🔴 not applied to the cluster |
| G6 | Tailnet Postgres reachable :5432 | `nc -z -w5 100.87.77.59 5432` | 🟢 reachable |
| G7 | cloudflared tunnel up + public DNS published | `kubectl get pods -n cloudflare` + `dig +short reckonna.thee5176.com` (expect `*.cfargotunnel.com`) | 🔴 pod Running, DNS record NOT published |
| G8 | Secrets operator installed (matches D5) | `kubectl get crd \| grep -iE 'externalsecret\|vaultstaticsecret'` | 🔴 no VSO — cluster runs **External Secrets Operator** + Vault Agent Injector |
| G9 | tailscale operator can create egress Service | `kubectl get pods -n tailscale` + operator ClusterRole allows `services` | 🟢 cluster-scoped RBAC; can create egress in reckonna-app |
| G10 | Keycloak app server up + `reckonna-smoke` client | `kubectl get deploy,statefulset -A \| grep -i keycloak` + `curl <issuer>/.well-known/openid-configuration` | 🔴 only `keycloak-postgres` up; app server + client + issuer missing |
| G11 | Vault app-secret paths populated | `vault kv get -mount=secret app/reckonna/db` (+ `oidc`, `smoke-oidc`) | 🔴 `secret/app/reckonna/*` subtree absent |
| G12 | Vault `reckonna-app` k8s-auth role + policy | `vault read auth/kubernetes/role/reckonna-app` + `vault policy read reckonna-app` | 🔴 role + policy absent |

Green = only the contract (G3), the DB socket (G6), and egress capability (G9). Everything that must
be built, merged, applied, or provisioned is missing.

## Corrections this validation forced on plan 05

1. **D5 → External Secrets Operator, not Vault Secrets Operator.** ESO is installed; VSO is not.
   Use `ExternalSecret` + a Vault `SecretStore` (not `VaultStaticSecret`). Same outcome — Vault →
   `reckonna-app-env` Secret → `envFrom` (distroless-safe).
2. **Vault paths.** DB creds already live at `secret/app/database` `{dbname,host,password,username}`
   — NOT `secret/app/reckonna/db`. Reuse it; create `secret/app/reckonna/{oidc,smoke-oidc}` only once
   Keycloak exists.
3. **Role naming.** Existing k8s-auth roles are `reckonna-{otel-collector,postgres,redis,tailscale-operator}`.
   New roles should follow that pattern (`reckonna-command`/`reckonna-query` or `reckonna-app`).

## Unblock order (each clears the noted gates)

- [ ] **1. Merge train to trunk** — land 01 + 02 (infra) then 03 (backend) into develop → main. *(G1, G4)*
- [ ] **2. Apply infra to the cluster** — namespace + postgres + cloudflared; `terraform apply` the
      `cloudflare_record` so `reckonna.thee5176.com` resolves. *(G5, G7)*
- [ ] **3. Build + publish images** — plan 03 CI on push to main publishes
      `ghcr.io/thee5176/reckonna-{command,query}`. *(G2)*
- [ ] **4. Stand up Keycloak** — deploy the app server (only its Postgres exists), create the
      resource-server client + the `reckonna-smoke` client-credentials client; needs an owner / small plan. *(G10)*
- [ ] **5. Provision Vault** — create `secret/app/reckonna/{oidc,smoke-oidc}`, reference the existing
      `secret/app/database`; create the `reckonna-app` role + policy (read on `secret/data/app/reckonna/*`
      + `secret/data/app/database`). *(G11, G12)*
- [ ] **6. Re-spec plan-05 D5 → ESO** with the corrected paths + role names. *(G8)*
- [ ] **7. Plan 05 executable** — approve → implement S0–S13 → two-phase cutover.

## Re-running this preflight

The gates were validated by three read-only agents (repo/CI, cluster, secrets). Re-run the commands
in the table above, or re-dispatch the validators. Update the status column + the `Last validated`
date each pass. `NO-GO` until every gate is 🟢 (or explicitly waived in plan 05).
