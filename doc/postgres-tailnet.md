# Postgres over Tailscale — Operator Setup + Client Usage

This document is the operator+developer how-to for the Postgres deployment in
plan `plans/02-infra-postgres-tailnet.md`. It assumes:

- An existing Kubernetes cluster with a default `StorageClass` and the
  HashiCorp **Vault Agent Injector** already running in namespace `vault`.
- An existing **Tailscale** account with admin access (an OAuth client can be
  minted).
- **Vault** server reachable from the cluster, with a KV-v2 mount at `secret/`.
- `kubectl`, `helm`, `vault`, `tailscale`, `psql`, `jq` available locally for
  the operator. (Developers only need `tailscale` and `psql`.)

The deployment runs the Postgres `StatefulSet` in namespace `postgres`. The
Tailscale Operator publishes the `Service` as MagicDNS device `pg-reckonna`,
reachable **only by machines joined to the tailnet**. There is no public
internet exposure.

---

## 1. One-time setup (operator)

### 1.1 Mint a Tailscale OAuth client

In the Tailscale admin console:

1. **Settings → OAuth clients → Generate OAuth client**.
2. Scopes: `Devices: Core (write)` and `Auth Keys (write)`.
3. Tags: `tag:k8s-operator`.
4. Copy the **Client ID** and **Client secret**. They are shown once.

Store them in Vault. Each value is read from stdin so it never appears in
shell history, then pushed in a single `vault kv put` call:

```bash
read -rs CID  ; read -rs CSEC ; read -rs TS_APIKEY
vault kv put -mount=secret app/tailscale/operator client_id="$CID" client_secret="$CSEC" api_key="$TS_APIKEY"
unset CID CSEC TS_APIKEY
```

### 1.2 Configure the Vault role + policy

The Vault Agent Injector reads its data via Kubernetes auth. Create the policy
and role that the `postgres` and `tailscale` ServiceAccounts will use:

```bash
# Policy that allows reading the database creds.
vault policy write reckonna-postgres - <<'POL'
path "secret/data/app/database" { capabilities = ["read"] }
POL

# Policy that allows reading the operator OAuth creds.
vault policy write reckonna-tailscale-operator - <<'POL'
path "secret/data/app/tailscale/operator" { capabilities = ["read"] }
POL

# Bind each policy to a Kubernetes SA.
vault write auth/kubernetes/role/reckonna-postgres \
  bound_service_account_names=postgres \
  bound_service_account_namespaces=postgres \
  policies=reckonna-postgres ttl=1h

vault write auth/kubernetes/role/reckonna-tailscale-operator \
  bound_service_account_names=tailscale-operator \
  bound_service_account_namespaces=tailscale \
  policies=reckonna-tailscale-operator ttl=1h
```

### 1.3 Seed the database credentials in Vault

```bash
PW="$(openssl rand -base64 24)"  # local-only; vault is the persistent store
vault kv put -mount=secret app/database username='app' password="$PW" dbname='accounting'
unset PW
```

(Use the corporate password manager / `pwgen` of your choice — never check
the password into any file.)

### 1.4 Apply the Kubernetes namespaces + manifests

```bash
# Namespaces are managed by Terraform.
( cd infra && terraform init && terraform apply )

# Postgres workload manifests.
kubectl apply -k infra/k8s/postgres
```

### 1.5 Install the Tailscale Operator

```bash
helm repo add tailscale https://pkgs.tailscale.com/helmcharts
helm repo update
helm install tailscale-operator tailscale/tailscale-operator \
  --namespace tailscale \
  --create-namespace \
  -f infra/k8s/tailscale/operator-values.yaml

# Apply the operator-oauth Secret manifest (Vault Agent Injector populates it).
kubectl apply -f infra/k8s/tailscale/namespace.yaml
kubectl apply -f infra/k8s/tailscale/operator-oauth-secret.yaml
```

Within ~30 seconds the operator picks up the annotations on
`service/pg-postgres` in namespace `postgres`, registers a tailnet device
named `pg-reckonna`, and starts proxying TCP/5432.

Verify:

```bash
kubectl -n tailscale get pods           # operator + a proxy pod
tailscale status | grep pg-reckonna     # the device appears on your tailnet
```

---

## 2. Developer setup (per machine)

### 2.1 Install + join the tailnet

macOS / Windows: install the Tailscale app, log in to the corporate tailnet.

Linux:

```bash
curl -fsSL https://tailscale.com/install.sh | sh
sudo tailscale up --accept-routes --accept-dns
tailscale status      # should list this device + 'pg-reckonna'
```

The device is auto-tagged `tag:dev` (per `infra/tailscale.tf` ACL). Without
that tag the tailnet ACL refuses port 5432.

### 2.2 Resolve the endpoint

```bash
make pg-endpoint
# hostname=pg-reckonna.<your-tailnet>.ts.net
# ip=100.x.y.z
```

Or, for scripting:

```bash
HOST="$(make -s pg-endpoint | awk -F= '/^hostname/ {print $2}')"
URL="$(scripts/pg-endpoint.sh --url)"
```

### 2.3 Smoke-test the connection

```bash
make tailnet-smoke
# tailnet-smoke: OK (pg-reckonna.<your-tailnet>.ts.net returned 1)
```

`tailnet-smoke.sh` pulls the credentials from Vault at `secret/app/database`
and runs `SELECT 1`. It never prints the password.

### 2.4 Connect with `psql`

```bash
# Each value comes from Vault at use-time. Nothing persists on disk.
export PGPASSWORD="$(vault kv get -mount=secret -field=password app/database)"
psql \
  -h "$(scripts/pg-endpoint.sh --hostname)" \
  -U "$(vault kv get -mount=secret -field=username app/database)" \
  -d "$(vault kv get -mount=secret -field=dbname app/database)"
unset PGPASSWORD
```

### 2.5 Connect with `migrate` (golang-migrate)

```bash
# vault-sourced inputs assembled into the URL at runtime; not stored anywhere.
DB_USER="$(vault kv get -mount=secret -field=username app/database)"
DB_PASS="$(vault kv get -mount=secret -field=password app/database)"
DB_NAME="$(vault kv get -mount=secret -field=dbname app/database)"
DB_HOST="$(scripts/pg-endpoint.sh --hostname)"
export DATABASE_URL="postgres://${DB_USER}:${DB_PASS}@${DB_HOST}:5432/${DB_NAME}?sslmode=require"
make migrate
unset DATABASE_URL DB_USER DB_PASS DB_NAME DB_HOST
```

### 2.6 Connect with GoLand / DataGrip / Beekeeper Studio

- Host: output of `scripts/pg-endpoint.sh --hostname` (e.g. `pg-reckonna.tail-foo123.ts.net`)
- Port: `5432`
- DB: `accounting`
- User / Password: pulled at the moment from `vault kv get -mount=secret app/database`
- SSL: prefer or require (Tailscale already encrypts; PG TLS adds defence-in-depth)

Do **not** save the password in the IDE's password vault unless that vault is
itself a managed enterprise secret store.

---

## 3. Security model

| Threat | Mitigation |
|--------|------------|
| Attacker on the public internet scans the cluster's IPs | No public LB; PG `Service` is `ClusterIP`. No tunnel exposes TCP/5432 outside the tailnet. |
| Attacker brute-forces the PG password via DNS lookup | The `pg-reckonna.*.ts.net` hostname does not resolve off the tailnet; even with the hostname they need a tailnet route. |
| Tailnet user with `tag:dev` is compromised | Tailnet ACL grants `tag:dev → tag:k8s:5432` only. The user still needs the Vault-rotated password. Rotate password in Vault → pods pick up on next restart (or use Vault dynamic creds in a later plan). |
| Pod inside cluster (different ns) tries to reach PG | `NetworkPolicy` `pg-postgres` allows ingress only from the `tailscale` namespace. |
| Secret leaks into a committed file | Pre-commit `no-secrets.sh` + CI `gitleaks` reject inline secrets. All secrets flow from Vault. |

---

## 4. Troubleshooting

| Symptom | Likely cause + check |
|---------|----------------------|
| `pg-endpoint: device 'pg-reckonna' not visible yet — operator may not have published it. Retry in 30s.` | The operator hasn't reconciled the Service annotations yet. Wait, or `kubectl -n tailscale logs deploy/operator`. |
| `psql: error: connection to server ... timed out` | You're off the tailnet. Run `tailscale status` to confirm. Without `tag:dev`, the ACL rejects you too. |
| `tailnet-smoke: vault CLI missing` | Install Vault CLI (`brew install vault` / `apt-get install vault`). |
| `pg_isready` failing inside the pod | The Vault Agent didn't render `/vault/secrets/db.env`. `kubectl -n postgres logs <pod> -c vault-agent-init`. |
| Connection succeeds but `SELECT 1` returns no rows | You hit a stale pgBouncer or another DB — check `\conninfo` in `psql`. |

---

## 5. Rotating credentials

```bash
NEW_PW="$(openssl rand -base64 24)"
vault kv put -mount=secret app/database username='app' password="$NEW_PW" dbname='accounting'
unset NEW_PW
kubectl -n postgres rollout restart statefulset/pg-postgres
```

The injector re-renders `/vault/secrets/db.env` on pod start; the entrypoint
sources it before `exec docker-entrypoint.sh postgres`.

For zero-downtime rotation use Vault dynamic database credentials (deferred
to a later plan).

---

## 6. References

- Plan: `plans/02-infra-postgres-tailnet.md`
- Tailscale Operator chart: <https://pkgs.tailscale.com/helmcharts>
- Vault Agent Injector: <https://developer.hashicorp.com/vault/docs/platform/k8s/injector>
- Repo rules: `.claude/rules/secrets-vault.md`, `.claude/rules/devops.md`
