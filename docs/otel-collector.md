# OpenTelemetry Collector — DaemonSet Setup + App Instrumentation

This document is the operator+developer how-to for the OpenTelemetry Collector
DaemonSet deployed by plan `plans/03-cache-otel-sidecar.md`. It assumes the
Vault Agent Injector + Tailscale Operator are already running (per
`docs/postgres-tailnet.md` §1).

The Collector runs as a `DaemonSet` (one pod per node) with `hostNetwork: true`
and host ports `4317` (OTLP gRPC) and `4318` (OTLP HTTP). Every workload pod
on the node ships its telemetry to `$(NODE_IP):4317` via the downward API.
The Collector then forwards in batches to an external OTLP/gRPC backend whose
URL and bearer token are sourced from Vault at `secret/app/otel/exporter`.

---

## 1. Pipeline

```
workload pod  ──(OTLP gRPC :4317)──>  node-local otel-collector
                                              │
                                              ▼
                                      memory_limiter (400 MiB)
                                              │
                                              ▼
                                      batch (5s / 1024 spans)
                                              │
                                              ▼
                            otlp/gRPC over TLS  ──>  external backend
                            (endpoint + Authorization header from Vault)
```

Three pipelines (`traces`, `metrics`, `logs`) share the same receivers,
processors, and exporters per `infra/k8s/otel/configmap.yaml`. The collector
image is pinned to `otel/opentelemetry-collector:0.108.0` — the contrib-free
build. Only `otlp`, `batch`, `memory_limiter` ship in-image; no vendor
exporters can pull credentials from outside Vault.

---

## 2. One-time setup (operator)

### 2.1 Seed the exporter credentials in Vault

```bash
read -rs OTEL_ENDPOINT      # e.g. otlp.<your-backend>.example:443
read -rs OTEL_API_KEY       # e.g. the bearer string the backend issued
vault kv put -mount=secret app/otel/exporter endpoint="$OTEL_ENDPOINT" api_key="$OTEL_API_KEY"
unset OTEL_ENDPOINT OTEL_API_KEY
```

Verify the two fields are non-empty without echoing values:

```bash
vault kv get -format=json -mount=secret app/otel/exporter \
  | jq '.data.data | to_entries | map({key, len: (.value|length)})'
```

### 2.2 Vault policy + role for the Collector pod

```bash
vault policy write reckonna-otel-collector - <<'POL'
path "secret/data/app/otel/exporter" { capabilities = ["read"] }
POL

vault write auth/kubernetes/role/reckonna-otel-collector bound_service_account_names=collector bound_service_account_namespaces=otel policies=reckonna-otel-collector ttl=1h
```

The role name (`reckonna-otel-collector`) matches the annotation on
`infra/k8s/otel/serviceaccount.yaml` and `infra/k8s/otel/daemonset.yaml`.

### 2.3 Apply the namespace + manifests

```bash
cd infra
terraform apply              # creates the `otel` namespace
cd ..

kubectl apply -k infra/k8s/otel
kubectl -n otel rollout status daemonset/otel-collector --timeout=2m
```

Confirm one pod per node is `Running 1/1`:

```bash
kubectl -n otel get pod -o wide
kubectl -n otel logs ds/otel-collector --tail=20
# expect: "Everything is ready. Begin running and processing data."
```

---

## 3. Instrumenting an app

Workloads send telemetry to the local-node collector via the downward API.
Inject `NODE_IP` into the pod and point the OTLP SDK at `$(NODE_IP):4317`.

### 3.1 Pod spec snippet (any workload)

```yaml
spec:
  template:
    spec:
      containers:
        - name: app
          env:
            - name: NODE_IP
              valueFrom:
                fieldRef:
                  fieldPath: status.hostIP
            - name: OTEL_EXPORTER_OTLP_ENDPOINT
              value: "http://$(NODE_IP):4317"
            - name: OTEL_EXPORTER_OTLP_PROTOCOL
              value: "grpc"
            - name: OTEL_SERVICE_NAME
              value: "my-service"
            - name: OTEL_RESOURCE_ATTRIBUTES
              value: "deployment.environment=prod"
```

`OTEL_EXPORTER_OTLP_ENDPOINT` is recognised by every OTLP-conformant SDK.
No Authorization header is needed at the workload — the local collector adds
it before forwarding upstream.

### 3.2 SDK examples

```go
// Go — go.opentelemetry.io/otel + otlptracegrpc
exp, _ := otlptracegrpc.New(ctx,
  otlptracegrpc.WithEndpoint(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")),
  otlptracegrpc.WithInsecure(),                      // local node hop is plaintext over hostNetwork
)
tp := sdktrace.NewTracerProvider(sdktrace.WithBatcher(exp))
otel.SetTracerProvider(tp)
```

```typescript
// Node — @opentelemetry/sdk-node + @opentelemetry/exporter-trace-otlp-grpc
import { NodeSDK } from '@opentelemetry/sdk-node';
import { OTLPTraceExporter } from '@opentelemetry/exporter-trace-otlp-grpc';

const sdk = new NodeSDK({
  traceExporter: new OTLPTraceExporter({
    url: process.env.OTEL_EXPORTER_OTLP_ENDPOINT,    // http://$(NODE_IP):4317
  }),
});
sdk.start();
```

```python
# Python — opentelemetry-sdk + opentelemetry-exporter-otlp-proto-grpc
from opentelemetry import trace
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import BatchSpanProcessor
from opentelemetry.exporter.otlp.proto.grpc.trace_exporter import OTLPSpanExporter

provider = TracerProvider()
exporter = OTLPSpanExporter(endpoint=os.environ["OTEL_EXPORTER_OTLP_ENDPOINT"],
                            insecure=True)
provider.add_span_processor(BatchSpanProcessor(exporter))
trace.set_tracer_provider(provider)
```

```java
// JVM — autoconfigure SDK reads OTEL_EXPORTER_OTLP_ENDPOINT directly.
// No code change — just set the env var via the pod spec above and add:
//   -javaagent:/opentelemetry-javaagent.jar
```

```typescript
// React Native (Expo) — @opentelemetry/sdk-trace-web + OTLP/HTTP
// RN cannot use gRPC; point at the HTTP receiver on :4318 instead.
new OTLPTraceExporter({
  url: `${process.env.EXPO_PUBLIC_OTEL_ENDPOINT}/v1/traces`,    // http://$(NODE_IP):4318/v1/traces
});
```

### 3.3 Verify

From a debug pod on the cluster, or from any tailnet host with `curl`:

```bash
make otel-smoke OTEL_TARGET=<node-ip>:4318
# otel-smoke: OK (http://<node-ip>:4318/v1/traces accepted span, HTTP 200)

kubectl -n otel logs ds/otel-collector --tail=20 \
  | grep -E 'TracesExporter|otlp.+success'
```

---

## 4. Operations

### 4.1 Rotate the API key

```bash
read -rs NEW
vault kv patch -mount=secret app/otel/exporter api_key="$NEW"
unset NEW
kubectl -n otel rollout restart daemonset/otel-collector
kubectl -n otel rollout status  daemonset/otel-collector --timeout=2m
```

The Vault Agent Injector re-renders `/vault/secrets/otel.env` on each pod
restart. AT5 acceptance: emit a synthetic span via `make otel-smoke` after
the rollout completes; if it lands at the backend, the new key is live.

### 4.2 Rotate the endpoint

Same flow, different field:

```bash
read -rs NEW_ENDPOINT
vault kv patch -mount=secret app/otel/exporter endpoint="$NEW_ENDPOINT"
unset NEW_ENDPOINT
kubectl -n otel rollout restart daemonset/otel-collector
```

If the new endpoint requires a different `Authorization` scheme than `Bearer`,
also update the vault template in `infra/k8s/otel/daemonset.yaml` — the
current template hardcodes the `Bearer ` prefix.

### 4.3 Trouble: collector logs show `connection refused` on the exporter

```bash
kubectl -n otel logs ds/otel-collector --tail=50 \
  | grep -E 'rpc error|connection refused|TLS|x509'
```

Causes:
1. NetworkPolicy egress missing — `infra/k8s/otel/networkpolicy.yaml` allows
   only TCP 443 to non-RFC1918 space. A backend on TCP 4317 with a public IP
   will be blocked. Either change the backend to TCP 443 or add a rule.
2. TLS misconfigured — the configmap pins `tls.insecure: false`. The backend
   must serve a publicly-trusted cert. Self-signed needs a `tls.ca_file`
   wired through a ConfigMap volume.
3. The Vault-rendered `endpoint` is wrong. Exec into a collector pod and
   `cat /vault/secrets/otel.env` (do not check the file into the repo).

### 4.4 Trouble: workload spans never reach the backend

1. Confirm the workload's `OTEL_EXPORTER_OTLP_ENDPOINT` resolves to a
   reachable host:port. `kubectl exec <pod> -- env | grep OTEL` then
   `curl -v http://$(NODE_IP):4318/v1/traces` from inside the pod.
2. Check the collector's drop counter:
   `kubectl -n otel logs ds/otel-collector | grep otelcol_processor_dropped`.
   Non-zero means `memory_limiter` shed load — increase resources or scale
   the backend.
3. Run `make otel-smoke OTEL_TARGET=$(NODE_IP):4318` from the same pod's
   namespace to isolate "SDK problem" vs "collector problem".

---

## 5. Security model recap

- The local node hop (`workload → 127.0.0.1:4317`) is plaintext over the host
  network. That is intentional — it never leaves the node.
- The external hop (`collector → backend:443`) is TLS over WireGuard-protected
  egress out of the cluster.
- The bearer token never appears in any tracked file, process arg, or env
  var literal. It is rendered into `/vault/secrets/otel.env` by the Vault
  Agent Injector and sourced by the entrypoint before exec'ing the collector.
- The Collector NetworkPolicy is the only path out: kube-dns + Vault +
  `0.0.0.0/0 - RFC1918/CGNAT` on TCP 443. Adding a backend on any other port
  requires updating the policy.
