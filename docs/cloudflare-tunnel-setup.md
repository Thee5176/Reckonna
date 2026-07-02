# Cloudflare Tunnel — provisioning, rotation, rollback (plan 02)

Human runbook for the `reckonna.thee5176.com` ingress. Terraform owns the Cloudflare zone/DNS/
tunnel/ingress; kustomize owns the k8s workloads (`reckonna-app` + `cloudflared`). **`terraform
apply` and `kubectl apply` are human-only** (devops.md). No real credential values appear here —
they live only in Vault; commands below read them from your shell env, never from a tracked file.

## Architecture (recap)
```
browser --HTTPS--> Cloudflare edge --tunnel--> cloudflared pod (outbound-only, no inbound port)
                                                    +-> reckonna-app.reckonna-app.svc:80 (nginx)
```
- No public LoadBalancer, no inbound firewall hole. cloudflared dials **out** to Cloudflare.
- Ingress is **remote-managed**: cloudflared runs `--no-autoupdate run --token …` with **no
  `--config`**; it pulls the ingress rules (S3's `cloudflare_zero_trust_tunnel_cloudflared_config`)
  from the Cloudflare API at startup.

## Prerequisites (one-time, human)
1. Cloudflare account with the `thee5176.com` zone.
2. A scoped API token (Zone:DNS:Edit + Account:Cloudflare Tunnel:Edit) for the Terraform provider.
3. The Cloudflare account id.
4. A tunnel secret — 32 random bytes base64: `openssl rand -base64 32`.

### Seed Vault
Export the four values into your shell first (from the CF dashboard / openssl), then write them in
one command — the literal values never touch a tracked file:
```bash
export CF_API_TOKEN=…  CF_ACCOUNT_ID=…  TUNNEL_SECRET=…  CONNECTOR_TOKEN=…
vault kv put -mount=secret app/cloudflare/tunnel api_token="$CF_API_TOKEN" account_id="$CF_ACCOUNT_ID" tunnel_secret="$TUNNEL_SECRET" token="$CONNECTOR_TOKEN"
```
The `token` field is the connector token cloudflared runs with. If Terraform creates the tunnel
(below), read the connector token from the CF dashboard afterward and patch it in (see Rotation).

### Vault k8s-auth role for cloudflared
The `cloudflared` ServiceAccount (ns `cloudflared`) authenticates to Vault as role
`reckonna-cloudflared` — a **NEW** role, distinct from plan 01's `reckonna-postgres`.
```bash
vault policy write reckonna-cloudflared - <<'POLICY'
path "secret/data/app/cloudflare/tunnel" { capabilities = ["read"] }
POLICY

vault write auth/kubernetes/role/reckonna-cloudflared bound_service_account_names=cloudflared bound_service_account_namespaces=cloudflared policies=reckonna-cloudflared ttl=1h
```

## Deploy (human)
```bash
# 1. Cloudflare zone/DNS/tunnel/ingress config
terraform -chdir=infra/terraform init
terraform -chdir=infra/terraform apply      # creates tunnel + reckonna CNAME (apex untouched)

# 2. Workloads (Vault Agent Injector renders the token into the cloudflared pod)
kubectl apply -k infra/k8s/reckonna-app
kubectl apply -k infra/k8s/cloudflared

# 3. Verify
make tunnel-dns-check    # reckonna.thee5176.com -> *.cfargotunnel.com  (AT5)
make tunnel-health       # https://reckonna.thee5176.com/healthz -> {"status":"ok"} (AT1)
kubectl get pods -n cloudflared    # 2 replicas Running
```

## Rotation
cloudflared does **not** hot-reload. Rotate the connector token with **patch, not put** (`put`
overwrites the whole secret and would wipe the other fields). Both lines run through Vault:
```bash
export CONNECTOR_TOKEN=…                                      # new value from the CF dashboard
vault kv patch -mount=secret app/cloudflare/tunnel token="$CONNECTOR_TOKEN"
kubectl rollout restart deployment/cloudflared -n cloudflared   # re-render + reconnect
```
Then revoke the old tunnel/token in the Cloudflare dashboard. Verify with `make tunnel-health`.

## Rollback (apex-safe)
Remove only the subdomain + connector; **never** touch the apex.
```bash
terraform -chdir=infra/terraform destroy -target=cloudflare_record.reckonna   # drops the subdomain CNAME only
kubectl scale deployment/cloudflared -n cloudflared --replicas=0              # optional: stop the connector
```

## Apex regression check (AT6)
The apex `thee5176.com` is only ever **read** (zone data source) — never written. Confirm it is
unchanged around any apply:
```bash
curl -sf https://thee5176.com/ > /tmp/apex-before    # BEFORE apply
# ... terraform apply ...
curl -sf https://thee5176.com/ > /tmp/apex-after     # AFTER
diff /tmp/apex-before /tmp/apex-after && echo "apex unchanged"
```

## Observability — approved exception
Per devops.md "observability is part of done", endpoints normally emit OpenTelemetry spans.
**Waived for plan 02**: the `reckonna-app` harness is throwaway nginx with no business logic;
cloudflared health is observed via `kubectl logs -n cloudflared` + the Cloudflare dashboard. Real
OTel spans arrive with plan 03's Go services on this same ingress. Recorded here as a scoped,
approved deviation — not an oversight.
