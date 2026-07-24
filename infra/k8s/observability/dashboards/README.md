# `dashboards/` — Reckonna RED dashboard (Grafana, self-hosted)

`reckonna-red.json` is the dashboard-as-code source for the "Reckonna — RED" board.
Per plan 06 D-GRAFANA (human directive, 2026-07-01): the dashboard targets the
**self-hosted homelab Grafana**, NOT Grafana Cloud. Datasource = the self-hosted
`kube-prometheus-stack-prometheus` (fed by the PodMonitor from S1 — see
`../podmonitor-reckonna-collector.yaml`).

## Open item: exact Grafana location

The plan's live-homelab discovery was **k3s-scoped** and only saw Grafana Cloud
credentials (ESO `grafanas.generators.external-secrets.io` + remote-write/Tempo
creds). A homelab Grafana running **outside k3s** (docker-compose, bare host, a
different cluster) would not have surfaced. This decides *which* of the two
provisioning paths below applies — confirm at apply time, don't assume:

```bash
kubectl get pods -A -l app.kubernetes.io/name=grafana   # in-k3s?
# else: check the docker host / systemd unit that runs Grafana on the homelab
```

The dashboard JSON itself is provisioning-target-agnostic — it uses a
`templating` datasource variable (`DS_PROMETHEUS`) instead of a hardcoded
datasource UID, so the same JSON works under either path below; only the
datasource variable gets bound to the local Prometheus datasource name/UID
at import/provisioning time.

## Path A — Grafana runs in k3s (ConfigMap sidecar)

If Grafana is deployed in-cluster with the `grafana` sidecar sitting on
`ConfigMap`s labeled for dashboard discovery (the standard
`kube-prometheus-stack` / `grafana` Helm chart pattern):

```bash
kubectl -n observability create configmap reckonna-red-dashboard \
  --from-file=reckonna-red.json=infra/k8s/observability/dashboards/reckonna-red.json \
  --dry-run=client -o yaml > /tmp/reckonna-red-cm.yaml
```

Label the ConfigMap so the sidecar picks it up (label key/value must match the
sidecar's `--label` flag / Helm `sidecar.dashboards.label` value on the live
deployment — verify before applying, don't assume `grafana_dashboard`):

```yaml
metadata:
  labels:
    grafana_dashboard: "1"   # confirm against the live sidecar config
```

`kubectl apply` is human-only (D8) — hand the rendered manifest to a human for apply.
Datasource: the sidecar-provisioned Grafana instance must already have a
Prometheus datasource pointed at `kube-prometheus-stack-prometheus.observability.svc:9090`;
bind `DS_PROMETHEUS` to that datasource's name/UID (Grafana resolves the
`${DS_PROMETHEUS}` template variable at dashboard-load time — no JSON edit needed
if the datasource is named/aliased consistently).

## Path B — Grafana runs outside k3s (standalone: provisioning dir / HTTP API / TF)

**Provisioning dir** (if you control the Grafana host's filesystem):
drop `reckonna-red.json` into Grafana's `provisioning/dashboards/<name>/` path
per a `provisioning/dashboards/*.yaml` provider config pointing at that directory.
No secret involved — provisioning-dir dashboards don't need an API token.

**HTTP API** (no filesystem access, e.g. remote docker host):
```bash
# token comes from Vault — never paste it into a file or shell history literal
GRAFANA_TOKEN=$(vault kv get -mount=secret -field=token app/grafana/homelab)
curl -sf -X POST "http://<homelab-grafana-host>:3000/api/dashboards/db" \
  -H "Authorization: Bearer ${GRAFANA_TOKEN}" \
  -H "Content-Type: application/json" \
  -d "{\"dashboard\": $(cat infra/k8s/observability/dashboards/reckonna-red.json), \"overwrite\": true}"
```

**Terraform** (`grafana_dashboard` resource) — only if the team wants dashboard
state tracked in TF for this homelab Grafana too:
```hcl
data "vault_kv_secret_v2" "grafana_homelab" {
  mount = "secret"
  name  = "app/grafana/homelab"
}

provider "grafana" {
  url  = "http://<homelab-grafana-host>:3000"
  auth = data.vault_kv_secret_v2.grafana_homelab.data["token"]
}

resource "grafana_dashboard" "reckonna_red" {
  config_json = file("${path.module}/../../k8s/observability/dashboards/reckonna-red.json")
}
```
`terraform apply` is human-only (D8) — this file, if added, only gets `terraform
validate` run against it here.

## Datasource — always the self-hosted Prometheus

Regardless of path, the datasource bound to `DS_PROMETHEUS` MUST be the
self-hosted `kube-prometheus-stack-prometheus` (the one the new PodMonitor from
S1 feeds), never the Grafana Cloud-remote-written copy — the PodMonitor scrape
interval (30s) is fresher and doesn't depend on Grafana Cloud's remote_write
lag. See `tests/grafana-dashboard_test.sh` for the static assertion (dashboard
does not hardcode a `grafanacloud`-style datasource UID).

## No secrets committed here

Any Grafana API token (Path B / HTTP API or TF) is Vault-only
(`secret/app/grafana/homelab`), read at apply time via `vault kv get` — never
written to a tracked file, per `.claude/rules/secrets-vault.md`.
