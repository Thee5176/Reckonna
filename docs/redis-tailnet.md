# Redis over Tailscale — Operator Setup + Client Usage

This document is the operator+developer how-to for the Redis cache deployment
in plan `plans/03-cache-otel-sidecar.md`. It mirrors the Postgres how-to in
`docs/postgres-tailnet.md` — read that first if you have not deployed Postgres
yet, as it covers Vault Agent Injector, Kubernetes auth wiring, and Tailscale
Operator install (Redis reuses all three).

Assumptions:

- An existing Kubernetes cluster with a default `StorageClass`, the HashiCorp
  **Vault Agent Injector** in namespace `vault`, and the **Tailscale Operator**
  in namespace `tailscale` (set up per `docs/postgres-tailnet.md` §1).
- A working **Vault** server reachable from the cluster, with a KV-v2 mount at
  `secret/`.
- `kubectl`, `vault`, `tailscale`, `redis-cli`, `jq` available locally for the
  operator. (Developers only need `tailscale` and `redis-cli`.)

The deployment runs the Redis `StatefulSet` in namespace `redis`. The Tailscale
Operator publishes the `Service` as MagicDNS device `redis-reckonna`, reachable
**only by machines joined to the tailnet**. There is no public exposure.

---

## 1. One-time setup (operator)

### 1.1 Seed the Redis password in Vault

```bash
PW="$(openssl rand -base64 24)"
vault kv put -mount=secret app/redis password="$PW"
unset PW
vault kv get -mount=secret app/redis   # verify
```

Verify the field is non-empty without echoing the value:

```bash
vault kv get -format=json -mount=secret app/redis \
  | jq '.data.data | to_entries | map({key, len: (.value|length)})'
```

### 1.2 Vault policy + role for the Redis pod

```bash
vault policy write reckonna-redis - <<'POL'
path "secret/data/app/redis" { capabilities = ["read"] }
POL

vault write auth/kubernetes/role/reckonna-redis \
  bound_service_account_names=redis \
  bound_service_account_namespaces=redis \
  policies=reckonna-redis ttl=1h
```

The role name (`reckonna-redis`) matches the annotation on
`infra/k8s/redis/serviceaccount.yaml` and `infra/k8s/redis/statefulset.yaml`.
Do not change one without the other.

### 1.3 Apply the namespace + manifests

```bash
cd infra
terraform apply              # creates the `redis` namespace
cd ..

kubectl apply -k infra/k8s/redis
kubectl -n redis rollout status statefulset/redis --timeout=2m
```

Within ~30s the Tailscale Operator picks up the `Service` annotations
(`tailscale.com/expose=true`, `tailscale.com/hostname=redis-reckonna`) and
publishes the device on the tailnet.

---

## 2. Developer per-machine setup

1. Install and start Tailscale on your laptop. Sign in to the same tailnet as
   the cluster. You should see `redis-reckonna` in `tailscale status`.
2. Resolve the endpoint:

   ```bash
   make redis-endpoint
   # hostname=redis-reckonna.<your-tailnet>.ts.net
   # ip=100.x.y.z
   ```

3. Pull the password from Vault into a scoped env var and connect:

   ```bash
   export REDISCLI_AUTH="$(vault kv get -mount=secret -field=password app/redis)"
   redis-cli -h "$(./scripts/redis-endpoint.sh --hostname)" -p 6379 PING
   # PONG
   unset REDISCLI_AUTH
   ```

   The `REDISCLI_AUTH` env var avoids putting the password on `argv` or in
   shell history. `make redis-smoke` wraps the same flow with an auto-unset
   trap if you only need a single PING.

### 2.1 Off-tailnet behaviour

From a host NOT on the tailnet, the hostname does not resolve and the TCP
connect fails. This is the intended security boundary — Redis is never
publicly reachable.

### 2.2 Per-stack driver examples

All examples below read their credentials from environment variables that
must be sourced from Vault at boot (`vault kv get` directly, or a runtime
Vault Agent sidecar). No secret enters any tracked file.

```go
// github.com/redis/go-redis/v9
rdb := redis.NewClient(&redis.Options{
  Addr:     os.Getenv("REDIS_ADDR"),                 // sourced from vault at boot
  Password: os.Getenv("REDIS_PASSWORD"),             // sourced from vault at boot
})
```

```typescript
// ioredis
const redis = new Redis({
  host: process.env.REDIS_HOST,                      // sourced from vault at boot
  port: 6379,
  password: process.env.REDIS_PASSWORD,              // sourced from vault at boot
});
```

```python
# redis-py
r = redis.Redis(
  host=os.environ["REDIS_HOST"],                     # sourced from vault at boot
  port=6379,
  password=os.environ["REDIS_PASSWORD"],             # sourced from vault at boot
)
```

```java
// Jedis
JedisPool pool = new JedisPool(
  System.getenv("REDIS_HOST"), 6379,                 // host sourced from vault at boot
  null, System.getenv("REDIS_PASSWORD"));            // password sourced from vault at boot
```

---

## 3. Operations

### 3.1 Rotate the password

Single-field patch + bounce the pod (the Vault Agent Injector renders on
restart):

```bash
vault kv patch -mount=secret app/redis password="$(openssl rand -base64 24)"
kubectl -n redis delete pod redis-0
kubectl -n redis rollout status statefulset/redis --timeout=2m
```

Then re-export `REDISCLI_AUTH` from Vault in each developer shell. Any
long-lived app process re-reads via its own Vault Agent or warm restart.

### 3.2 Trouble: `NOAUTH Authentication required`

The pod started without `/vault/secrets/redis.conf` populated. Check:

```bash
kubectl -n redis logs redis-0 -c vault-agent-init
kubectl -n redis exec redis-0 -c redis -- ls -l /vault/secrets/redis.conf
kubectl -n redis exec redis-0 -c redis -- head -1 /vault/secrets/redis.conf
# expect: requirepass "<...>"
```

If the file is missing or empty, the Vault role binding is wrong (wrong SA
name, wrong namespace, or `secret/data/app/redis` empty). Re-check §1.1 and
§1.2.

### 3.3 Trouble: pod CrashLoops with `Fatal error, can't open config file`

`redis-server --include` could not read `/vault/secrets/redis.conf`. Same
diagnosis as §3.2 — the Vault Agent Injector did not render the file.

### 3.4 Trouble: tailnet resolves but TCP times out

Two common causes:

1. The Tailscale Operator's proxy pod is not ready. `kubectl -n tailscale
   get pod -l tailscale.com/parent-resource=redis` should show `Running 1/1`.
2. The NetworkPolicy denies your namespace. Only the `tailscale` namespace
   may reach `:6379` — that is the intentional boundary; do not widen it.

---

## 4. Security model recap

- Wire: WireGuard inside the tailnet. No PG/Redis-layer TLS needed.
- Authn: `requirepass` from Vault. Per-client ACL users are out of scope for
  plan 03 (single-tenant cache).
- Authz at the network: tailnet ACLs gate device→cluster reachability. The
  NetworkPolicy in `infra/k8s/redis/networkpolicy.yaml` gates
  `tailscale-operator → redis` and nothing else.
- Secrets: never inline. Vault is the only source of truth per
  `.claude/rules/secrets-vault.md`.
