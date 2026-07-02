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
