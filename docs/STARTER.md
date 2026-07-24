# Reckonna Starter — Vault + Tailnet PG on Kubernetes

Reusable bootstrap for any project that wants a PostgreSQL StatefulSet on
Kubernetes exposed only over Tailscale, with credentials sourced from a
HashiCorp Vault server already running in the target cluster.

Clone this tag and apply the steps below. The plan + manifests + scripts are
intentionally vendor-neutral — works on k3s, kind, EKS, GKE, AKS, or any
upstream Kubernetes.

> **Tag:** `starter/reckonna-infra-v0.1.0` — pinned, reproducible snapshot.
> Use `git clone --branch starter/reckonna-infra-v0.1.0` for a one-shot copy.

---

## What's in the box

| Layer | Path | What you get |
|-------|------|--------------|
| **Claude harness setup** | `.claude/CLAUDE.md`, `.claude/rules/{devops,secrets-vault,tdd,migrations}.md`, `.claude/skills/`, `.claude/hooks/no-secrets.sh` | Conventional Commits + `Plan: S<n>` trailer enforcement; deny-by-default on inline secrets; V-model + TDD policy; Vault-as-only-source-of-truth policy |
| **V-model planning** | `plans/02-infra-postgres-tailnet.md`, `V_MODEL_PLAN.md` | The plan that this starter executes, plus the project-wide V-model template |
| **Terraform (vendor-neutral)** | `infra/main.tf`, `infra/providers.tf`, `infra/secrets.tf`, `infra/postgres.tf`, `infra/tailscale.tf` | `vault`, `kubernetes`, `tailscale` providers; namespaces; Tailscale ACL with admin-everything safety rule |
| **Kustomize bases** | `infra/k8s/postgres/`, `infra/k8s/tailscale/` | PG StatefulSet w/ Vault Agent Injector annotations + NetworkPolicy + PDB + Service; Tailscale Operator OAuth Secret skeleton + Helm values |
| **Operator/dev scripts** | `scripts/pg-endpoint.sh`, `scripts/tailnet-smoke.sh`, `scripts/pg-probe.sh` | Resolve the tailnet hostname, run `SELECT 1` from the operator side, stage-by-stage app-side connectivity probe with DNS→TCP→TLS→auth→query classification |
| **Make targets** | `Makefile` → `pg-endpoint`, `tailnet-smoke`, `pg-probe`, `k8s-validate`, `tf-validate`, `ci` | Single-entrypoint commands; gates skip cleanly when tools are absent |
| **Runbook** | `docs/postgres-tailnet.md` | Diataxis how-to: operator one-time setup, developer per-machine setup, security model, troubleshooting, credential rotation, app integration matrix (Go pgx, Python psycopg, Node pg, JDBC, Rust sqlx) |
| **Offline tests** | `tests/*.sh` | Static + behavioural shim coverage for manifests + scripts; runs without a cluster or Vault |

---

## Prereqs (target cluster)

1. **Kubernetes cluster** with a default `StorageClass` (k3s `local-path` works) and Pod Security Admission `restricted` baseline.
2. **HashiCorp Vault** server running in the target cluster (any namespace; this guide assumes ns `vault`). Sealed=false, KV-v2 mounted at `secret/`.
3. **Tailscale account** with admin access — you can mint an OAuth client.
4. **Local tooling**: `kubectl`, `helm`, `terraform >= 1.9`, `vault`, `jq`, `tailscale`.

```bash
make tools-verify          # confirms pinned versions present
```

---

## Step 0 — Clone the starter

```bash
git clone --branch starter/reckonna-infra-v0.1.0 https://github.com/Thee5176/Reckonna.git my-project
cd my-project
```

For an existing repo, copy the reusable subtrees: `.claude/`, `infra/`, `scripts/`, `Makefile`, `plans/`, `docs/postgres-tailnet.md`.

---

## Step 1 — Seed Vault values (A1–A3)

All credentials live in Vault. Never inline. `read -rs` keeps secret strings off shell history. Every `vault kv put` call below is a single line so the no-secrets hook accepts it (the hook requires the literal word `vault` on any line that mentions a credential field).

### A1. Database creds → `secret/app/database`

```bash
DB_PW="$(openssl rand -base64 24)"
vault kv put -mount=secret app/database username='app' password="$DB_PW" dbname='accounting'
unset DB_PW
vault kv get -mount=secret app/database   # verify 3 fields present
```

> ⚠️ **Verify length, not value.** The password field length should be non-zero. Empty stdin reads silently to empty string and break the postgres pod entrypoint with `POSTGRES_PASSWORD not specified`.

```bash
vault kv get -format=json -mount=secret app/database | jq '.data.data | to_entries | map({key, len: (.value|length)})'
```

### A2. Tailscale Operator OAuth → `secret/app/tailscale/operator`

Mint at `https://login.tailscale.com/admin/settings/oauth` — scopes:
`policy_file:write`, `devices:core:write`, `auth_keys:write`; tag `tag:k8s-operator`. Copy `tskey-client-...` (the OAuth client_id) AND the opaque secret string (the OAuth client_secret).

```bash
read -rs CID    # paste tskey-client-...
read -rs CSEC   # paste the opaque secret string
vault kv put -mount=secret app/tailscale/operator client_id="$CID" client_secret="$CSEC"
unset CID CSEC

# Verify prefixes (no value exposure)
vault kv get -format=json -mount=secret app/tailscale/operator | jq '.data.data | {client_id_prefix: .client_id[0:14], client_secret_len: (.client_secret|length)}'
# expect: client_id_prefix="tskey-client-k", client_secret_len > 30
```

### A3. (Optional) CI runner ephemeral auth key

Only needed if your CI runners join the tailnet to run `make pg-probe`.

```bash
read -rs TS_EPH
vault kv put -mount=secret app/tailscale/runner ephemeral_authkey="$TS_EPH"
unset TS_EPH
```

---

## Step 2 — Wire Vault to the cluster (B1–B3)

These three pieces let workload pods log in to Vault using their
ServiceAccount token. They are out-of-band for Terraform.

### B1. Vault Kubernetes auth method

```bash
# 1. Reviewer SA + ClusterRoleBinding (idempotent)
kubectl apply -f - <<'YAML'
apiVersion: v1
kind: ServiceAccount
metadata: {name: vault-auth, namespace: vault}
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata: {name: vault-auth-tokenreview}
roleRef: {apiGroup: rbac.authorization.k8s.io, kind: ClusterRole, name: system:auth-delegator}
subjects: [{kind: ServiceAccount, name: vault-auth, namespace: vault}]
YAML

# 2. Mint a long-lived JWT for that SA (k8s 1.24+)
JWT="$(kubectl create token vault-auth -n vault --duration=8760h)"

# 3. Cluster URL + CA from kubeconfig
K8S_HOST="$(kubectl config view --raw --minify -o jsonpath='{.clusters[0].cluster.server}')"
K8S_CA="$(kubectl config view --raw --minify -o jsonpath='{.clusters[0].cluster.certificate-authority-data}' | base64 -d)"

# 4. Enable + configure (single-line vault write so the hook accepts it)
vault auth enable kubernetes 2>/dev/null || true
vault write auth/kubernetes/config token_reviewer_jwt="$JWT" kubernetes_host="$K8S_HOST" kubernetes_ca_cert="$K8S_CA" disable_iss_validation=true

unset JWT K8S_HOST K8S_CA
vault read auth/kubernetes/config   # verify
```

### B2. Policies — read-only access per path

```bash
vault policy write reckonna-postgres - <<'POL'
path "secret/data/app/database" { capabilities = ["read"] }
POL

vault policy write reckonna-tailscale-operator - <<'POL'
path "secret/data/app/tailscale/operator" { capabilities = ["read"] }
POL
```

### B3. Bind each policy to a Kubernetes ServiceAccount

```bash
vault write auth/kubernetes/role/reckonna-postgres bound_service_account_names=postgres bound_service_account_namespaces=postgres policies=reckonna-postgres ttl=1h

vault write auth/kubernetes/role/reckonna-tailscale-operator bound_service_account_names=operator bound_service_account_namespaces=tailscale policies=reckonna-tailscale-operator ttl=1h
```

---

## Step 3 — Install Vault Agent Injector (if not already present)

The Postgres StatefulSet uses Vault Agent annotations — the cluster needs the mutating webhook to honour them. Skip if `kubectl get mutatingwebhookconfiguration | grep vault` returns rows.

```bash
helm repo add hashicorp https://helm.releases.hashicorp.com
helm repo update hashicorp

helm upgrade --install vault-injector hashicorp/vault \
  --namespace vault \
  --set "server.enabled=false" \
  --set "injector.enabled=true" \
  --set "injector.externalVaultAddr=http://vault.vault.svc.cluster.local:8200" \
  --wait --timeout 3m
```

---

## Step 4 — Terraform apply (namespaces + Tailscale ACL)

```bash
cd infra
terraform init
terraform plan -input=false -out=plan02.tfplan
# expect: Plan: 3 to add, 0 to change, 0 to destroy.
terraform apply -input=false plan02.tfplan
```

> ⚠️ **Tailnet self-lockout risk.** `tailscale_acl.policy` uses `overwrite_existing_content = true` and is deny-by-default. The provided ACL keeps `autogroup:admin → *:*` so the operator does not lose kubectl/SSH reach. If you narrow this rule, replace its coverage with explicit per-device flows first or you will lock yourself out of the cluster. See `plans/02-infra-postgres-tailnet.md` "Known gaps" for the hardening follow-up.

---

## Step 5 — Pre-populate the Tailscale Operator OAuth Secret

The Operator Helm chart references an existing k8s Secret named `operator-oauth`. Create it from the vault-sourced OAuth credentials. The pattern uses `--from-literal` keys that the no-secrets hook permits because the literal `vault` appears via the source command on each producing line.

```bash
CID="$(vault kv get -mount=secret -field=client_id app/tailscale/operator)"        # vault
CSEC="$(vault kv get -mount=secret -field=client_secret app/tailscale/operator)"   # vault
kubectl create secret generic operator-oauth -n tailscale --from-literal=client_id="$CID" --from-literal=client_secret="$CSEC" --dry-run=client -o yaml | kubectl apply -f -
unset CID CSEC
```

---

## Step 6 — Install Tailscale Operator

```bash
helm repo add tailscale https://pkgs.tailscale.com/helmcharts
helm repo update tailscale

helm upgrade --install tailscale-operator tailscale/tailscale-operator \
  --namespace tailscale \
  --version 1.98.4 \
  -f infra/k8s/tailscale/operator-values.yaml \
  --wait --timeout 3m

kubectl -n tailscale get pod   # expect operator-... Running 1/1
```

---

## Step 7 — Apply the Postgres workload

```bash
kubectl apply -k infra/k8s/postgres
kubectl -n postgres rollout status statefulset/pg-postgres --timeout=4m
```

Within ~30s the Tailscale Operator picks up the Service annotations
(`tailscale.com/expose=true`, `tailscale.com/hostname=pg-reckonna`) and
publishes the device on your tailnet.

---

## Step 8 — Verify

```bash
make pg-endpoint
# hostname=pg-reckonna.<your-tailnet>.ts.net
# ip=100.x.y.z

# From inside the pod (loopback — proves DB + creds work):
kubectl -n postgres exec pg-postgres-0 -c postgres -- sh -c '. /vault/secrets/db.env && psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -tA -c "SELECT 1, current_user, current_database();"'

# From your laptop (proves tailnet hop):
make tailnet-smoke      # requires psql installed locally
# OR
make pg-probe           # app-side staged probe; uses libpq PG* env vars

# Direct TCP smoke (no psql needed):
HOST="$(./scripts/pg-endpoint.sh --hostname)"
timeout 5 bash -c "exec 3<>/dev/tcp/$HOST/5432" && echo OK
```

---

## Step 9 — Connect from any application

The endpoint is a normal TCP Postgres service once your host is on the
tailnet with `tag:dev`. Drivers pick up libpq `PG*` env vars implicitly.

```bash
export PGHOST="$(./scripts/pg-endpoint.sh --hostname)"
export PGUSER="$(vault kv get -mount=secret -field=username app/database)"        # vault
export PGPASSWORD="$(vault kv get -mount=secret -field=password app/database)"    # vault
export PGDATABASE="$(vault kv get -mount=secret -field=dbname app/database)"      # vault
export PGSSLMODE=prefer

# Your Go / Python / Node / JVM / Rust app picks these up at boot.
# See docs/postgres-tailnet.md §2A for per-stack driver examples.
```

---

## Common failure modes

| Symptom | Cause | Fix |
|---------|-------|-----|
| `Post "http://localhost/api/v1/namespaces": connection refused` on `terraform apply` | Kubernetes provider has no `config_path`; falls back to localhost | `export KUBECONFIG=~/.kube/config` or set `var.kubeconfig_path` (see commit `dd27c88`) |
| `Failed to set ACL — API token invalid (401)` | OAuth client lacks `policy_file:write` scope | Re-mint at `admin/settings/oauth` with the three documented scopes |
| `calling actor does not have enough permissions (403)` | Same as above | Same fix |
| Postgres pod `CrashLoopBackOff` with `POSTGRES_PASSWORD not specified` | A1 password field is empty in Vault | Single-line `vault kv patch -mount=secret app/database password="$(openssl rand -base64 24)"` then `kubectl -n postgres delete pod pg-postgres-0` |
| `kubectl` times out at `dial tcp <tailnet-ip>:6443: i/o timeout` immediately after `terraform apply` | New tailnet ACL dropped admin-everything rule | Paste corrected policy at `admin/acls/file` (see Step 4 warning) |
| `failed to lookup token, err=context deadline exceeded` on terraform plan | Vault server unreachable | Check `vault status`; if behind a tunnel/CDN, port-forward directly: `kubectl -n vault port-forward svc/vault 8200:8200 && export VAULT_ADDR=http://127.0.0.1:8200` |
| Pod `CrashLoopBackOff` but vault-agent-init succeeded | `/vault/secrets/db.env` rendered but values empty (A1 / A2 / A3 path or fields wrong) | Spawn debug pod with same SA + same annotations, peek at the rendered file (`grep -oE '^export [A-Z_]+=' /vault/secrets/db.env`), then re-patch the offending Vault field |
| Tailscale Operator helm install fails with `expected string, got slice` | Chart 1.98.x expects `defaultTags` as comma-separated string, not YAML list | Already fixed in this starter; see `infra/k8s/tailscale/operator-values.yaml` (commit `1441ee0`) |

---

## What this starter does NOT include

- Backups / WAL archiving (deferred)
- PG HA via Patroni or Spilo (single replica)
- pg-bouncer (single client pattern)
- PG-layer TLS / managed certificates (WireGuard already encrypts the wire)
- Public-internet exposure (intentional — see plan 02 decisions table)
- Vault dynamic database credentials (rotation is currently manual — see runbook §5)

Each gap has a tracking note in `plans/02-infra-postgres-tailnet.md` under "Known gaps".

---

## License + provenance

This starter is the Reckonna project's plan 02 deliverable, frozen at tag
`starter/reckonna-infra-v0.1.0`. Reuse it for any project under the same
repo's license. The plan-as-code methodology (`plans/<feature>.md` → V-model
phases → one-step-one-commit with `Plan: S<n>` trailer) is documented in
`.claude/CLAUDE.md` and `.claude/rules/devops.md`.
