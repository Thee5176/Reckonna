# Reckonna OTel Telemetry Setup

<!-- otel-contract:start -->
## OTLP contract

This section pins the app-to-collector OTLP contract locked in
`plans/06-infra-otel-telemetry.md` (D2, D10, R3). It is the source of truth
for `tests/otel-contract_test.sh`. **Do not delete or renumber this section**
— other steps (S6) append around it, they do not edit it.

### Endpoint

Command and query both export OTLP/HTTP to the existing shared collector,
`observability` namespace:

```
OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector.observability.svc.cluster.local:4318
OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf
```

The base endpoint has no `/v1/...` suffix — the SDK's OTLP/HTTP exporters
(`otlptracehttp`, `otlpmetrichttp`) append the signal-specific path
themselves (`/v1/traces`, `/v1/metrics`) per the OTLP spec. gRPC (`4317`) is
also open on the collector Service but HTTP/protobuf is the contracted
protocol — do not switch without updating this doc.

### Resource attributes

```
service.name          = reckonna-command   (command service)
service.name          = reckonna-query     (query service)
deployment.environment = homelab
```

Set in `internal/config/otel.go` (`SetupTelemetry`) via
`semconv.ServiceName("reckonna-"+cfg.ServiceName)` +
`attribute.String("deployment.environment", cfg.Environment)`.

### Metric-export contract (D10 — already implemented)

Traces alone leave the RED dashboard empty, so both signals are wired:

- **Traces**: `otlptracehttp` batches spans through a `TracerProvider` — one
  exporter per process, batched (`sdktrace.WithBatcher`).
- **Metrics**: `otlpmetrichttp` + a `MeterProvider` with a
  `PeriodicReader` (`metric.WithReader(metric.NewPeriodicReader(mexp))`) — NOT
  spans-only. This rides the collector's existing `otlp -> prometheus`
  pipeline; no shared-collector config change required.

Both exporters read `OTEL_EXPORTER_OTLP_ENDPOINT` /
`OTEL_EXPORTER_OTLP_PROTOCOL` from the environment directly (no per-exporter
endpoint override in code) — see `internal/config/otel.go`.

Confirmed instrument names (`internal/metrics/metrics.go`), OTel dot notation
becomes Prometheus underscore + `_total` suffix on export:

```
reckonna.http.server.requests -> reckonna_http_server_requests_total
  labels: http.request.method, http.route, http.response.status_code
reckonna.ledger.rejected      -> reckonna_ledger_rejected_total
  labels: reason (e.g. "unbalanced_entry" on the 借方≠貸方 reject path)
```

When `OTLPEndpoint` is unset (local dev/tests), no-export tracer/meter
providers are installed instead — instrumentation still runs, nothing dials
out.

### Egress rule (for the backend-Deploy plan)

The command/query Deployments and their NetworkPolicy don't exist yet (R3) —
this is a **contract**, not a manifest shipped by this plan. When the
backend-Deploy plan creates the Deployments, it MUST also ship an egress
`NetworkPolicy` allowing:

```
reckonna-backend (command + query pods) -> observability namespace
  TCP 4317   (OTLP/gRPC, open but unused while http/protobuf is contracted)
  TCP 4318   (OTLP/HTTP — the contracted protocol above)
```

Without this policy the exporters will fail to dial the collector once
NetworkPolicies are default-deny in the backend namespace.
<!-- otel-contract:end -->

## Topology

This plan is **additive wiring** into the existing homelab `observability` stack —
it does not deploy or re-own the shared collector/Prometheus (plan 06 D1).

```
 command / query pods
   │ OTLP/HTTP :4318 (traces + metrics)
   ▼
 otel-collector (shared, observability ns)  ── otlp/tempo ──► Grafana Cloud Tempo (traces)
   │ prometheus exporter :8889
   ▼
 PodMonitor (this plan) ─ selects app=otel-collector, targetPort 8889
   ▼
 self-hosted Prometheus (kube-prometheus-stack)  ── remote_write ──► Grafana Cloud (metrics)
   ▲                                                         
   │ datasource                                             
 self-hosted homelab Grafana  ◄── "Reckonna — RED" dashboard (this plan)
```

**The gap this closes:** the collector already exposes app metrics on `:8889`, but its
Service is unlabeled so nothing scraped it → app metrics never reached Prometheus.
A `PodMonitor` selecting the collector **pods** (`app: otel-collector`) fixes it with
zero mutation of shared infra. Traces already flow.

## What this plan ships

| File | Purpose |
|------|---------|
| `infra/k8s/observability/podmonitor-reckonna-collector.yaml` | scrape the collector's `:8889` (S1) |
| `infra/k8s/observability/dashboards/reckonna-red.json` + `README.md` | RED dashboard + provisioning paths (S2) |
| `scripts/otel-{health,metrics-smoke,trace-smoke}.sh` | smokes (S4) |
| this doc + `tests/otel-contract_test.sh` | the OTLP contract above (S3) |

## Grafana provisioning

Grafana is **self-hosted on the homelab** (D-GRAFANA, 2026-07-01), datasource = the
self-hosted Prometheus. The exact location (in-k3s vs standalone) is unconfirmed — see
`infra/k8s/observability/dashboards/README.md` for both provisioning paths (ConfigMap
sidecar if in-k3s; provisioning dir / HTTP API / TF `grafana_dashboard` if standalone).
The dashboard token, when needed, comes from Vault (`secret/app/grafana/homelab`) — never
inline.

**Collector-config preconditions** (verify against the live shared collector before the
dashboard renders per-service): the prometheus exporter must keep `add_metric_suffixes=true`
(default — else the `_total` suffix vanishes), and `service_name` is a resource attribute on
`target_info`, not a metric label, unless `resource_to_telemetry_conversion` is enabled — so
the dashboard queries join via `target_info` to be safe either way.

## Apply order (human-only — `devops.md`)

`kubectl apply` / `terraform apply` are human-only. Once the backend command/query
Deployments exist (backend-Deploy plan) and emit OTLP:

1. `kubectl apply -k infra/k8s/observability/` — the PodMonitor.
2. Provision `dashboards/reckonna-red.json` to the self-hosted Grafana per its README.
3. Verify: `make k8s-validate`, `make otel-health`, `make otel-metrics-smoke`; open the
   "Reckonna — RED" dashboard.

## Rollback

`kubectl delete -k infra/k8s/observability/` removes only the PodMonitor (and, if TF-provisioned,
`terraform destroy -target=grafana_dashboard.reckonna_red`). The shared collector, Prometheus,
and Grafana Cloud remote_write are **untouched** — this plan added nothing to them.
