# Plan 03 — Redis Cache on Kubernetes + OpenTelemetry Collector Sidecar (Tailnet-Only Cache, Per-Node Collector)

status: approved  <!-- require-prereq.sh greps this -->
approved_by: thee5176
approved_at: 2026-05-31

**Source of truth** for adding a Redis cache and an OpenTelemetry Collector to the existing
remote Kubernetes cluster. Redis mirrors the Plan 02 Postgres pattern: a `StatefulSet` with a
persistent volume, password sourced from Vault, exposed only over the tailnet via the Tailscale
Operator, with a default-deny NetworkPolicy. The OTel Collector ships as a `DaemonSet` so every
node has a local OTLP receiver on `:4317` (gRPC) and `:4318` (HTTP), then forwards in batches
to an external OTLP gRPC endpoint whose URL and API key are read from Vault. **No `terraform
apply`, no `kubectl apply`, no `helm install` in this plan** — those are human-only per
`devops.md`. The deliverables here are: manifests, Terraform, helper scripts, docs.

## Decisions (locked, 2026-05-31)

| # | Decision | Rationale |
|---|----------|-----------|
| 1 | Redis runs as a single-replica `StatefulSet` in its own `redis` namespace with a persistent `volumeClaimTemplate`. | Matches Plan 02 Postgres topology and survives node restarts. Single replica is sufficient as a cache for the current CQRS workload; HA is a future plan. |
| 2 | Redis password is rendered from Vault path `secret/app/redis` by the Vault Agent Injector into `/vault/secrets/redis.conf`, then loaded via `redis-server --include`. No plaintext in any manifest. | Enforces `secrets-vault.md`. Mirrors the `secret/app/database` pattern from Plan 02. |
| 3 | Redis is exposed **tailnet-only** via the Tailscale Operator. `Service` annotations `tailscale.com/expose: "true"` and `tailscale.com/hostname: "redis-reckonna"` publish the MagicDNS device. No public route, no Funnel. | Same threat model as Plan 02; Funnel does not expose arbitrary TCP and Redis auth alone is not a sound public boundary. |
| 4 | Redis ingress NetworkPolicy is default-deny; only pods in the `tailscale` namespace may reach `:6379`. Egress allows kube-dns + Vault. | Mirror Plan 02 networkpolicy; tailnet ACLs gate the operator. |
| 5 | OpenTelemetry Collector runs as a `DaemonSet` in its own `otel` namespace with `hostNetwork: true` and host ports `4317` (OTLP gRPC) and `4318` (OTLP HTTP). The `otel` namespace's Pod Security Admission is set to `privileged` because `restricted` and `baseline` both forbid `hostNetwork` and `hostPort`. Container `securityContext` stays hardened (`runAsNonRoot: 10001`, `readOnlyRootFilesystem: true`, drop ALL caps, no privilege escalation) — the relaxation is only at the namespace admission layer. | One collector per node yields a stable in-cluster `OTEL_EXPORTER_OTLP_ENDPOINT=http://$(NODE_IP):4317` for every workload, eliminates per-pod sidecar overhead, and keeps the receiver loopback-cheap for instrumented services. PSA `privileged` is the smallest namespace-scoped relaxation that lets the chosen DaemonSet shape pass admission. |
| 6 | Collector pipeline: `otlp` receiver (gRPC + HTTP) → `batch` processor → `otlp` exporter (gRPC) to an external endpoint. Endpoint URL and `Authorization` header (API key) are rendered from Vault path `secret/app/otel/exporter` into `/vault/secrets/otel.env` and consumed via env-var substitution in the collector config. | Vendor-neutral export to any OTLP-compatible backend (Honeycomb, Grafana Cloud, Tempo, etc.). Re-keys without rebuilding the image. |
| 7 | The Collector is a `contrib`-free build (`otel/opentelemetry-collector:0.108.0`) — only OTLP in/out, `batch`, `memory_limiter`. No vendor exporters in-image. | Minimal blast radius; supply-chain hygiene; matches `secrets-vault.md` posture (no vendor SDK pulls credentials from non-Vault sources). |
| 8 | One step = one commit = one `Plan: S<n>` trailer. Conventional Commits. | `devops.md`. |

## File structure

```
plans/03-cache-otel-sidecar.md             # this file
infra/
  k8s/
    redis/
      service.yaml                         # ClusterIP + tailscale exposure annotations
      statefulset.yaml                     # redis 7-alpine, vault-agent-injected requirepass, PVC template
      serviceaccount.yaml                  # SA + Vault role binding annotations (reckonna-redis)
      pdb.yaml                             # PodDisruptionBudget minAvailable=1
      networkpolicy.yaml                   # deny all + allow tailscale-operator ns
      kustomization.yaml                   # kustomize base (ns owned by Terraform)
    otel/
      configmap.yaml                       # collector pipeline: otlp recv -> batch -> otlp exp (vault-templated)
      daemonset.yaml                       # hostNetwork=true, hostPorts 4317/4318, vault-agent annotations
      service.yaml                         # headless service for in-cluster discovery via $(NODE_IP)
      serviceaccount.yaml                  # SA + Vault role binding annotations (reckonna-otel-collector)
      networkpolicy.yaml                   # ingress 4317/4318 from any pod; egress kube-dns + Vault + external OTLP
      kustomization.yaml                   # kustomize base (ns owned by Terraform)
  redis.tf                                 # kubernetes_namespace for redis
  otel.tf                                  # kubernetes_namespace for otel
  secrets.tf                               # + data sources for secret/app/redis + secret/app/otel/exporter
scripts/
  redis-endpoint.sh                        # resolve tailnet hostname/IP for the Redis service
  redis-smoke.sh                           # non-destructive reachability probe (redis-cli -h <host> PING)
  otel-smoke.sh                            # send a synthetic span to local NODE_IP:4317 and confirm exporter accepted
tests/
  redis-service-annotations_test.sh        # IT5
  redis-vault-injector_test.sh             # IT6
  redis-networkpolicy_test.sh              # IT3-equivalent for redis
  redis-endpoint_test.sh                   # IT10
  otel-daemonset_test.sh                   # IT7 + IT8 (hostNetwork + vault annotations)
  otel-config_test.sh                      # configmap pipeline shape (no literal endpoint/key)
  otel-networkpolicy_test.sh               # IT9
docs/
  redis-tailnet.md                         # how-to: setup + connect (Diataxis how-to)
  otel-collector.md                        # how-to: pipeline, instrument an app, rotate the API key
  STARTER.md                               # extended with redis + otel sections + new vault paths
Makefile                                   # + redis-endpoint, redis-smoke, otel-smoke
```

---

## Section 1 — Acceptance-test spec (E2E)

Acceptance tests are **manual** in this plan — they need a live cluster and a tailnet-joined host
(same constraint as Plan 02). They become CI once a non-prod cluster lands.

| ID  | Given / When / Then | Domain | Test artifact |
|-----|---------------------|--------|---------------|
| AT1 | Given a host on the tailnet / When run `scripts/redis-endpoint.sh` / Then it prints a `*.ts.net` MagicDNS hostname **and** a `100.x.y.z` tailnet IP, exit 0. | infra | `scripts/redis-endpoint.sh` + `tests/redis-endpoint_test.sh` (dry-run with a fake `tailscale` shim) |
| AT2 | Given the same host / When run `scripts/redis-smoke.sh` / Then `redis-cli -h <hostname> -a <pw> PING` returns `PONG`. Password sourced from Vault, never echoed. | infra | `scripts/redis-smoke.sh` |
| AT3 | Given a host **not** on the tailnet / When `redis-cli -h <hostname>` is attempted / Then the connection fails (no DNS / no route). | infra | manual; documented in `docs/redis-tailnet.md` |
| AT4 | Given an instrumented pod with `OTEL_EXPORTER_OTLP_ENDPOINT=http://$(NODE_IP):4317` / When it emits a span / Then the span is accepted by the local collector and forwarded; `kubectl -n otel logs ds/otel-collector` shows the exporter's success counter incrementing. | infra | `scripts/otel-smoke.sh` + manual log check |
| AT5 | Given the OTLP gRPC external endpoint or API key is rotated in Vault / When the next collector pod restart happens / Then the new credentials are picked up without code change, and AT4 still passes. | infra | manual; documented in `docs/otel-collector.md` "rotation" section |

## Section 2 — Integration-test spec

| ID   | Condition to verify | Domain | Test artifact |
|------|---------------------|--------|---------------|
| IT1  | Every manifest under `infra/k8s/redis/**` and `infra/k8s/otel/**` passes `kubeconform -strict` against k8s 1.30. | infra | `make k8s-validate` |
| IT2  | `terraform validate` is green for `infra/` with the new `redis.tf` + `otel.tf` + extended `secrets.tf`. | infra | `make tf-validate` |
| IT3  | `infra/k8s/redis/networkpolicy.yaml` denies pods outside the `tailscale` and `vault` namespaces from reaching `app/component=cache`. | infra | `tests/redis-networkpolicy_test.sh` |
| IT4  | No literal secret value appears in any committed file under `infra/k8s/redis/**`, `infra/k8s/otel/**`, or `infra/*.tf`. `gitleaks` clean. | infra | `gitleaks detect --no-git -s infra/` |
| IT5  | The Redis `Service` has annotations `tailscale.com/expose: "true"` and `tailscale.com/hostname: "redis-reckonna"`. | infra | `tests/redis-service-annotations_test.sh` |
| IT6  | The Redis `StatefulSet` declares Vault Agent annotations rendering `requirepass` from `secret/app/redis` into `/vault/secrets/redis.conf`, and `redis-server` loads it via `--include`. No literal `requirepass` value in the manifest. | infra | `tests/redis-vault-injector_test.sh` |
| IT7  | The OTel `DaemonSet` declares `hostNetwork: true`, `dnsPolicy: ClusterFirstWithHostNet`, and `hostPort` 4317 (gRPC) + 4318 (HTTP) on the collector container. | infra | `tests/otel-daemonset_test.sh` |
| IT8  | The OTel `DaemonSet` declares Vault Agent annotations rendering `OTEL_EXPORTER_OTLP_ENDPOINT` and `OTEL_EXPORTER_OTLP_HEADERS` from `secret/app/otel/exporter` into `/vault/secrets/otel.env`. ConfigMap references those env vars via `${env:...}` substitution, never literal values. | infra | `tests/otel-daemonset_test.sh` + `tests/otel-config_test.sh` |
| IT9  | OTel `NetworkPolicy` permits ingress on TCP 4317 + 4318 from any namespace (cluster-wide trace ingest), allows egress to kube-dns, Vault, and `0.0.0.0/0` on TCP 443 (external OTLP/gRPC TLS). Documented exception — call out in the policy comment. | infra | `tests/otel-networkpolicy_test.sh` |
| IT10 | `scripts/redis-endpoint.sh` is idempotent and exits non-zero with a clear message when `tailscale` is missing or the operator hostname is not yet propagated. | infra | `tests/redis-endpoint_test.sh` |

## Section 3 — Implementation steps (one commit each)

Each step compiles/validates on its own. RED tests are committed before the manifest that satisfies them where TDD applies. For pure manifest steps without behavioural code, the verifier is the static check (kubeconform / terraform validate / grep test).

| ID  | Commit (verbatim) | Files | Verify |
|-----|-------------------|-------|--------|
| S0  | `docs(plan): infra plan 03 — redis cache + otel collector daemonset (tailnet-only cache)` | `plans/03-cache-otel-sidecar.md` | review only |
| S1  | `chore(k8s): redis namespace + kustomization base` | `infra/k8s/redis/kustomization.yaml` | `kubeconform -strict` |
| S2  | `feat(k8s): redis service + statefulset with vault-injector annotations` | `infra/k8s/redis/service.yaml`, `infra/k8s/redis/statefulset.yaml`, `infra/k8s/redis/serviceaccount.yaml`, `infra/k8s/redis/pdb.yaml`, `tests/redis-service-annotations_test.sh`, `tests/redis-vault-injector_test.sh` | `kubeconform`; IT5; IT6 |
| S3  | `feat(k8s): redis networkpolicy — deny-all + allow tailscale-operator + vault` | `infra/k8s/redis/networkpolicy.yaml`, `tests/redis-networkpolicy_test.sh` | `kubeconform`; IT3 |
| S4  | `feat(infra): terraform stub — kubernetes_namespace.redis + vault data source` | `infra/redis.tf`, `infra/secrets.tf` | `terraform validate` |
| S5  | `chore(k8s): otel namespace + kustomization base` | `infra/k8s/otel/kustomization.yaml` | `kubeconform -strict` |
| S6  | `feat(k8s): otel collector configmap — otlp recv -> batch -> otlp export (vault-templated)` | `infra/k8s/otel/configmap.yaml`, `tests/otel-config_test.sh` | `kubeconform`; IT8 (config half) |
| S7  | `feat(k8s): otel collector daemonset with hostNetwork + vault-injector annotations` | `infra/k8s/otel/daemonset.yaml`, `infra/k8s/otel/service.yaml`, `infra/k8s/otel/serviceaccount.yaml`, `tests/otel-daemonset_test.sh` | `kubeconform`; IT7; IT8 (daemonset half) |
| S8  | `feat(k8s): otel networkpolicy — ingress 4317/4318 cluster-wide; egress dns + vault + external OTLP` | `infra/k8s/otel/networkpolicy.yaml`, `tests/otel-networkpolicy_test.sh` | `kubeconform`; IT9 |
| S9  | `feat(infra): terraform stub — kubernetes_namespace.otel + vault data source` | `infra/otel.tf`, `infra/secrets.tf` | `terraform validate` |
| S10 | `feat(scripts): redis-endpoint.sh + redis-smoke.sh` | `scripts/redis-endpoint.sh`, `scripts/redis-smoke.sh`, `tests/redis-endpoint_test.sh` | shellcheck; `tests/redis-endpoint_test.sh` |
| S11 | `feat(scripts): otel-smoke.sh — emit synthetic OTLP span to local node` | `scripts/otel-smoke.sh` | shellcheck; manual span emission against fake collector |
| S12 | `chore(make): redis-endpoint, redis-smoke, otel-smoke targets` | `Makefile` | `make help` lists targets; targets skip cleanly when tool missing |
| S13 | `docs(infra): how to set up redis on the tailnet` | `docs/redis-tailnet.md` | manual review; markdownlint optional |
| S14 | `docs(infra): otel collector pipeline, app instrumentation, key rotation` | `docs/otel-collector.md` | manual review; markdownlint optional |
| S15 | `docs(starter): extend STARTER.md with redis + otel sections + new vault paths` | `docs/STARTER.md` | manual review; cross-links resolve |

### Step notes

- **S2 — Redis Vault Agent Injector annotations.** Use `vault.hashicorp.com/agent-inject: "true"`, `vault.hashicorp.com/role: reckonna-redis`, and per-template annotations rendering the `requirepass` directive from `secret/data/app/redis` into `/vault/secrets/redis.conf`. The container command is `redis-server --include /vault/secrets/redis.conf` (the include path picks up `requirepass "<value>"` written by the agent). The injector mounts in pre-populate mode so the file exists before PID 1 starts. **No secret value is committed** — only Vault paths. PVC template requests 5Gi (cache, not durable store of record).
- **S3 — Redis NetworkPolicy.** Default-deny ingress to the `redis` namespace. Allow ingress from pods in namespace `tailscale` on TCP 6379 (the Operator's proxy pod). Allow egress to the `vault` namespace on 8200 and to `kube-system` on 53/TCP+UDP. Mirror of Plan 02 IT3.
- **S4 — Redis Terraform.** Adds `kubernetes_namespace.redis` with the same PSA `restricted` labels as Plan 02's `postgres` ns, plus a `vault_kv_secret_v2.redis` data source in `secrets.tf` reading `secret/app/redis`. The data source is unused inside Terraform (no resource references it) — its purpose is contract documentation + Vault-side audit on `terraform plan`.
- **S6 — Collector ConfigMap.** Pipeline shape:
  ```yaml
  receivers:
    otlp:
      protocols:
        grpc: { endpoint: 0.0.0.0:4317 }
        http: { endpoint: 0.0.0.0:4318 }
  processors:
    memory_limiter: { check_interval: 1s, limit_mib: 400 }
    batch: { timeout: 5s, send_batch_size: 1024 }
  exporters:
    otlp:
      endpoint: ${env:OTEL_EXPORTER_OTLP_ENDPOINT}
      headers:  ${env:OTEL_EXPORTER_OTLP_HEADERS}
      tls: { insecure: false }
  service:
    pipelines:
      traces:  { receivers: [otlp], processors: [memory_limiter, batch], exporters: [otlp] }
      metrics: { receivers: [otlp], processors: [memory_limiter, batch], exporters: [otlp] }
      logs:    { receivers: [otlp], processors: [memory_limiter, batch], exporters: [otlp] }
  ```
  `tests/otel-config_test.sh` asserts the file uses `${env:...}` substitution and contains **no** literal `https://`, `Bearer `, or `Authorization:` strings.
- **S7 — DaemonSet shape.** `hostNetwork: true` + `dnsPolicy: ClusterFirstWithHostNet`. `containerPort` 4317 and 4318 with matching `hostPort`. `securityContext` drops all caps, `allowPrivilegeEscalation: false`, `readOnlyRootFilesystem: true`, `runAsNonRoot: true`. Vault Agent annotations `vault.hashicorp.com/role: reckonna-otel-collector`, target `secret/data/app/otel/exporter`, render `OTEL_EXPORTER_OTLP_ENDPOINT` and `OTEL_EXPORTER_OTLP_HEADERS` (the latter formatted as `Authorization=Bearer <key>`) into `/vault/secrets/otel.env`. Container entrypoint `sources` `/vault/secrets/otel.env` before `exec otelcol --config /etc/otel/config.yaml`. The headless `service.yaml` is **only** for in-cluster service discovery name (`otel-collector.otel.svc.cluster.local`) — actual traffic goes to `$(NODE_IP):4317` via the downward API in workload pods.
- **S8 — OTel NetworkPolicy.** Ingress allow on TCP 4317 + 4318 from `namespaceSelector: {}` (cluster-wide) — workloads in any namespace need to ship telemetry. Egress allow: `kube-system` 53/TCP+UDP, `vault` 8200/TCP, and a wildcard `ipBlock: { cidr: 0.0.0.0/0, except: [10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16, 100.64.0.0/10] }` on TCP 443 for the external OTLP/gRPC endpoint over TLS. The `except` list keeps the egress from accidentally re-entering RFC1918/CGNAT private space.
- **S9 — OTel Terraform.** Adds `kubernetes_namespace.otel` with the same PSA `restricted` labels and a `vault_kv_secret_v2.otel_exporter` data source reading `secret/app/otel/exporter` (contract + audit only).
- **S10 — Redis endpoint script.** `scripts/redis-endpoint.sh` mirrors `scripts/pg-endpoint.sh`: resolves the hostname two ways (`tailscale status --json | jq` for the `redis-reckonna` device, **or** `kubectl get service redis -n redis -o jsonpath='{.metadata.annotations.tailscale\.com/hostname}'`). Prints hostname + IP. Tests use a `tailscale` PATH shim. `redis-smoke.sh` pulls the password from Vault into a sub-shell variable (never persisted), then `redis-cli -h <host> -a "$REDIS_PASSWORD" PING`, finally `unset REDIS_PASSWORD`.
- **S11 — OTel smoke script.** Emits a single synthetic OTLP span to `http://$(NODE_IP):4317` via `grpcurl` against the published proto OR via a 10-line Go one-shot under `scripts/otel-smoke/`. Exit 0 on accepted span. CI variant runs against a fake OTLP receiver container.
- **S12 — Makefile.** Targets skip cleanly when their tool (`redis-cli`, `grpcurl`) is absent. `make redis-endpoint`, `make redis-smoke`, `make otel-smoke`.
- **S13 — Redis docs.** Diataxis how-to: Vault path setup (`secret/app/redis`), policy + role wiring (`reckonna-redis`), tailnet client setup, connection examples (`redis-cli`, `go-redis`, `ioredis`, `aioredis`, `jedis`), troubleshooting `NOAUTH Authentication required`, off-tailnet behaviour, key rotation procedure (`vault kv patch` → delete pod).
- **S14 — OTel docs.** Diataxis how-to: pipeline diagram, instrument an app (Go/Node/Python/JVM/RN snippets), set `OTEL_EXPORTER_OTLP_ENDPOINT=http://$(NODE_IP):4317` via downward API, what gets batched, what goes external, rotation procedure for `secret/app/otel/exporter`, `kubectl -n otel logs ds/otel-collector` troubleshooting (drop-rate metric, queue-saturation, TLS handshake errors).
- **S15 — STARTER.md extension.** Add A4 (`secret/app/redis`), A5 (`secret/app/otel/exporter`) to Step 1; B4/B5 policies + roles to Step 2; Step 7b (apply otel + redis kustomizations); Step 8b (`make redis-smoke`, `make otel-smoke`); add the new common failure modes; bump tag header to `starter/reckonna-infra-v0.2.0`. The tag itself is created by the human after S15 lands and CI is green.

## Hand-off to the heads

- **infra-engineer (HEAD):** owns every step. Writes IT1–IT10 as failing checks first where applicable (S2/S3/S6/S7/S8/S10), then green via `iac-ops` → `code-reviewer`.
- **Human:** runs `terraform apply`, `kubectl apply -k infra/k8s/redis`, `kubectl apply -k infra/k8s/otel`, plus Vault policy + role wiring (A4/A5, B4/B5 in the extended STARTER.md). Not in this plan.
- "Done" = AT1–AT5 documented; IT1–IT10 green; `docs/redis-tailnet.md` + `docs/otel-collector.md` + extended `docs/STARTER.md` reviewed; plan-tracker logs to `03-cache-otel-sidecar.impl.md`; tag `starter/reckonna-infra-v0.2.0` cut on `main` after merge.

## Known gaps (deferred)

- **No Redis HA** (single replica, no Sentinel/Cluster). Deferred to a future plan if cache durability becomes critical.
- **No Redis TLS at the wire** (WireGuard already encrypts inside the tailnet; same posture as PG in Plan 02). Add `--tls-port` if cross-tailnet exposure is later required.
- **No Redis ACL users beyond the default.** Only `requirepass` is set; per-app users land when the workload split needs them.
- **No exporter retry / persistent queue.** Collector pipeline uses in-memory `batch` only — a long external outage will drop telemetry. Add `file_storage` extension + `sending_queue.persistent` once volume justifies it.
- **No Collector resource autoscaling.** DaemonSet inherits one collector per node, sized via static `resources.requests/limits` (200m CPU / 256Mi mem); tune per backend pressure.
- **No log/metric SDK wiring in app code.** Plan 01 endpoints emit traces only via Gin middleware; structured logs + Prometheus → OTLP bridges are app-side follow-ups.
- **No tailnet ACL update for the new `redis-reckonna` device.** The Plan 02 ACL grants `autogroup:admin → *:*` so admin reach is preserved (commit `c68b68d`); narrowing per the Plan 02 follow-up note also covers Redis.
