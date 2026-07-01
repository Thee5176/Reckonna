---
feature: 06-infra-otel-telemetry
status: draft
approved_by: TBD
approved_at: TBD
domain: infra
depends_on:
  - 01-infra-postgres-tailnet     # homelab k3s + Vault Agent Injector pattern
  - 03-backend-cqrs-core          # S16 command+query emit OTLP via otelgin + otlptracehttp
external_prereq:
  - live observability stack (NOT repo-managed): kube-prometheus-stack (Prometheus Operator)
    + a shared otel-collector, both in the `observability` namespace. This plan WIRES INTO
    them; it does not create or re-own them.
decisions:
  collector: REUSE the existing shared otel-collector (gateway Deployment, otel/opentelemetry-collector-contrib:0.120.0, observability ns) — do NOT deploy a 2nd collector, do NOT replace the live shared one (it also scrapes homelab postgres). Additive wiring only.
  app_to_collector: command+query export OTLP/HTTP to http://otel-collector.observability.svc.cluster.local:4318 (grpc 4317 also open); resource attrs service.name=reckonna-command / reckonna-query. Contract only — env injection lands with the backend Deploy plan.
  metrics_source: app exports OTLP METRICS (otelgin meter provider + otlpmetrichttp) — NOT just spans. otelgin/otlptracehttp emit traces only; RED metrics need a metric exporter. Rides the collector's EXISTING otlp→prometheus pipeline (no shared-collector change). Backend contract on plan 03 (adds go.mod otlpmetrichttp). Alt (not chosen): collector spanmetrics connector derives RED from spans but mutates shared collector config.
  metrics_path: collector's EXISTING prometheus exporter (:8889) scraped by the self-hosted kube-prometheus-stack Prometheus via a NEW PodMonitor (selects pods app=otel-collector, targetPort 8889; podMonitorSelector={release: kube-prometheus-stack}). PodMonitor over ServiceMonitor: the collector Service has no labels + is not repo-owned (can't kustomize-patch it); the pods DO carry app=otel-collector → zero mutation of shared infra. NOT prometheusremotewrite (Prometheus already owns the single remote_write to Grafana Cloud).
  traces_path: ALREADY WIRED — the live collector exports OTLP → Grafana Cloud Tempo (otlp/tempo). Not dropped, not duplicated. Self-hosted Tempo = OPEN follow-up (out of scope).
  logs_path: OUT of scope — no Loki in cluster; collector logs pipeline is debug-only.
  grafana: Grafana is Grafana CLOUD (NOT self-hosted in-cluster). OPEN (D-GRAFANA, human sign-off): dashboard-as-code via the Grafana Terraform provider (token from Vault) — recommended — vs. commit JSON + import via the existing ESO grafana generator (no new provider/token). Drops into a ConfigMap sidecar if Grafana is ever self-hosted.
  egress: OTLP endpoint + egress are a DOCUMENTED CONTRACT here; the actual NetworkPolicy lands with the backend-Deploy plan (the command/query Deployments don't exist yet — a standalone NP now would be an orphan).
  secrets: Vault only (secrets-vault.md). New Grafana TF SA token → Vault. Live collector's existing k8s secrets are NOT migrated here (shared-infra churn, out of scope).
  human_only: kubectl apply, terraform apply — manifests + tf + tests only in this plan.
review_log:
  - discovery on 2026-07-01: live homelab inventoried read-only; brief's "self-hosted Grafana" + "new collector" assumptions corrected against reality (Grafana Cloud; collector already exists; traces already flow).
  - /plan-eng-review on 2026-07-01 (infra head): 2 architecture blockers fixed (R1 ServiceMonitor+label-patch → PodMonitor, zero shared-infra mutation; R2 RED metrics had no source → lock D10 app exports OTLP metrics via otlpmetrichttp), 1 scope trim (R3 drop orphan NetworkPolicy → contract only). 1 OPEN for human: D-GRAFANA (TF provider vs ESO-import). Files ~18 → ~13.
  - /plan-eng-review on 2026-07-01 (human, lead): (1) D10 metric-export contract DISPATCHED to the live backend head — plan 03 S16 adds otlpmetrichttp + OTLP meter provider + `reckonna_ledger_rejected_total`; closes the R2 landmine at the source, not a doc-only contract. (2) D-GRAFANA RESOLVED by user directive → **self-hosted homelab Grafana** for OUR dashboard, NOT Grafana Cloud; datasource = self-hosted Prometheus. CAVEAT: the kubectl discovery was k3s-scoped and saw only Grafana Cloud; a homelab Grafana OUTSIDE k3s (docker/other host) wouldn't appear — infra-head confirms its location + reachability at implementation (that also sets the provisioning mechanism: ConfigMap sidecar if in-k3s, else provisioning dir / HTTP API / TF provider at the local URL). New test gap: dashboard datasource must target the self-hosted Prometheus (add to grafana-dashboard_test.sh).
  - D10 CLOSED 2026-07-01 (backend head landed + pushed on feat/03, -race green): metric names CONFIRMED — `reckonna_ledger_rejected_total` (label `reason="unbalanced_entry"`, on the 借方≠貸方 path) and `reckonna_http_server_requests_total` (labels `http_request_method`/`http_route`/`http_response_status_code`; backend added its OWN counter — otelgin does NOT emit a `*_requests_total`). Latency: use otelgin's `http_server_request_duration_milliseconds_{sum,count}` for avg (its `_bucket` is dropped by the live D9 `filter/drop_high_cardinality` — consistent with D9's no-histogram_quantile rule). **TWO collector-config PRECONDITIONS for S2's dashboard (verify against the live shared collector, do NOT assume):** (a) the prometheus exporter must keep `add_metric_suffixes=true` (default) or the `_total` suffix vanishes and queries miss; (b) `service.name`/`deployment.environment` are RESOURCE attrs landing on `target_info`, NOT counter labels — so per-service dashboard filtering needs the collector's `resource_to_telemetry_conversion` enabled OR the panels must join via `target_info`. If neither holds on the shared collector, either the dashboard joins on `target_info` or S2 is blocked on a (shared) collector change. App reads OTEL_EXPORTER_OTLP_ENDPOINT (base; exporters append /v1/{traces,metrics}), OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf, DEPLOYMENT_ENVIRONMENT=homelab.
---

# Plan 06 — Wire Reckonna OTLP Telemetry into the Homelab Observability Stack

Connects the Go command+query services' OpenTelemetry output into the **existing**
homelab observability stack so their metrics land in the self-hosted Prometheus (and,
via its remote_write, in Grafana Cloud) and their traces keep flowing to Grafana Cloud
Tempo. The pipeline is **already 90% built** — the missing wire is a **PodMonitor**
so Prometheus actually scrapes the collector's metrics endpoint, plus a **Grafana
dashboard as code** and the app→collector **OTLP contract** (traces AND metrics).
Everything is vendor-neutral (OTLP + OTel Collector; Grafana Cloud is just a swappable
OTLP/remote-write sink). **No `kubectl apply`, no `terraform apply`** — human-only per
`devops.md`. Deliverables: manifests, Terraform, dashboard JSON, scripts, docs, tests.

**Prerequisite:** Plan 01 landed (homelab k3s + Vault Agent Injector). Plan 03 S16 makes
the services emit OTLP **traces** (otelgin + otlptracehttp); this plan's D-METRICSRC adds
the **metrics** exporter (otlpmetrichttp) as a backend contract — `go.mod` currently has
only the trace exporter. The live `observability` namespace (kube-prometheus-stack +
shared collector) exists **outside this repo** — this plan wires into it additively.

## Decisions (locked at draft, 2026-07-01 — grounded in live discovery)

| # | Decision | Rationale |
|---|----------|-----------|
| D1 | **Reuse the existing shared `otel-collector`** (gateway Deployment, contrib 0.120.0, observability ns). NOT a per-node DaemonSet, NOT a second collector. | A single gateway is right for a small homelab; the collector already receives OTLP (4317/4318), scrapes homelab postgres, exports metrics to :8889 and traces to Grafana Cloud Tempo. Fragmenting or re-owning it would churn shared homelab observability (it serves more than Reckonna). Additive wiring only. |
| D2 | **App → collector**: command+query export OTLP/HTTP to `http://otel-collector.observability.svc.cluster.local:4318` (grpc 4317 open too); resource attrs `service.name=reckonna-command` / `reckonna-query`, `deployment.environment=homelab`. Exports **both traces and metrics** (see D10). | Plan 03 S16 wires otelgin + `otlptracehttp`; this plan pins the destination + identity. The env injection into the command/query Deployments lands with the backend-Deploy plan (those Deployments don't exist yet) — this plan owns the **contract**; the egress NetworkPolicy also lands there (R3), not here. |
| D3 | **METRICS scrape = a NEW `PodMonitor`** selecting the collector **pods** (`app: otel-collector`) on `targetPort: 8889`, labeled `release: kube-prometheus-stack`. NOT a ServiceMonitor + label-patch, NOT `prometheusremotewrite`. | **[R1, eng-review]** The collector `prometheus` exporter (:8889) is unscraped because the collector Service has no `metadata.labels` — and it's not repo-owned, so kustomize can't patch a label onto it (kustomize only patches resources in its own build). The **pods** already carry `app: otel-collector`, and the Prometheus CR's `podMonitorSelector = {release: kube-prometheus-stack}` (verified). A PodMonitor is a single self-contained object with **zero mutation** of shared infra → :8889 → Prometheus → its existing remote_write → Grafana Cloud. (`prometheusremotewrite` from the collector would double-ship — Prometheus already owns the single remote_write.) |
| D10 | **METRICS SOURCE = the app exports OTLP metrics** (otelgin meter provider + `otlpmetrichttp`), riding the collector's **existing** `otlp → prometheus` metrics pipeline — no shared-collector-config change. | **[R2, eng-review]** otelgin + `otlptracehttp` emit **spans only**; `go.mod` has no metric exporter and plan 03 S16 wires traces only. Without this, the RED dashboard (`reckonna_http_*`) and the `reckonna_ledger_rejected_total` counter would be **empty**. This is a backend contract on plan 03 (adds `go.mod otlpmetrichttp`). **Alt not chosen:** the collector `spanmetrics` connector derives RED from the already-flowing spans (zero backend work) but mutates the shared collector config — rejected to keep shared infra untouched. |
| D4 | **TRACES = already wired to Grafana Cloud Tempo** (collector `otlp/tempo` exporter). This plan adds **no** trace backend. **Self-hosted Tempo is an OPEN follow-up, explicitly OUT of scope** (not a silent drop). | Traces already flow; OTLP is the vendor-neutral wire, Grafana Cloud Tempo is a swappable sink. Self-hosted Tempo would remove the cloud dependency but adds a stateful service + object storage — unjustified for a small homelab today. Flagged, not dropped. |
| D5 | **LOGS = NOT in scope.** | No Loki in the cluster; the collector's logs pipeline is `debug`-only. Structured-log shipping is a follow-up if/when Loki is added. |
| D6 | **Grafana is Grafana CLOUD** (no in-cluster Grafana pod/svc/ingress; ESO grafana generator + Grafana Cloud remote_write creds confirm it). Dashboard shipped as **versioned JSON**. **RESOLVED 2026-07-01 (user directive): target the SELF-HOSTED homelab Grafana, NOT Grafana Cloud, for OUR dashboard.** Datasource = self-hosted Prometheus (`kube-prometheus-stack-prometheus:9090`). Provisioning mechanism = ConfigMap-sidecar if Grafana runs in k3s, else Grafana provisioning dir / HTTP API / TF provider pointed at the local Grafana URL. **CAVEAT:** kubectl discovery was k3s-scoped and saw only Grafana Cloud — a homelab Grafana OUTSIDE k3s (docker/other host) wouldn't appear; infra-head confirms location + reachability at implementation. (PodMonitor metrics wire D3/D10 unchanged — self-hosted Grafana just reads the same self-hosted Prometheus. Traces D4 stay on Grafana Cloud Tempo unless self-hosted Tempo is later added.) | User runs a self-hosted Grafana on the homelab. The dashboard-as-code JSON is provisioning-target-agnostic; only the datasource pointer + provisioning path change. New test gap: `grafana-dashboard_test.sh` asserts the datasource targets the self-hosted Prometheus. |
| D7 | **All secrets via Vault** (`secrets-vault.md`). The only NEW secret is the Grafana TF SA token → `secret/app/grafana/terraform`. The live collector's existing k8s secrets (`grafana-remote-write-secret`, `postgres-otel-secret`) are **NOT migrated** in this plan. | No literal secret in any manifest/tfvars. Migrating live shared secrets to Vault Injector is shared-infra churn — separate follow-up, not this plan. |
| D8 | **Human-only**: `kubectl apply`, `terraform apply`. One step = one commit = one `Plan: S<n>` trailer. Conventional Commits. | `devops.md`. |
| D9 | **RED latency uses avg (sum/count), NOT `histogram_quantile`.** | The live collector runs `filter/drop_high_cardinality` which drops `.*_bucket` + `.*_created` to stay under the Grafana Cloud free-tier 10k-series cap. Histogram buckets are gone → p50/p95/p99 are unavailable. The dashboard uses request-rate + error-ratio + avg-latency. A filter exception to keep `reckonna_*_bucket` for percentiles is a flagged follow-up. |

## File structure

```
plans/06-infra-otel-telemetry.md                 # this file
infra/k8s/observability/
  podmonitor-reckonna-collector.yaml             # [R1] scrape collector pods (app=otel-collector) targetPort 8889; labeled release=kube-prometheus-stack
  kustomization.yaml                             # bases the podmonitor (self-contained; no patch of the shared Service)
  dashboards/
    reckonna-red.json                            # RED panels (rate/errors/avg-duration) per service + ledger domain counters
infra/terraform/                                  # only if D-GRAFANA resolves to (a) TF provider
  grafana-providers.tf                           # grafana provider; auth token via `vault_kv_secret_v2` data source (Grafana Cloud)
  grafana-dashboard.tf                           # grafana_dashboard from dashboards/reckonna-red.json
scripts/
  otel-health.sh                                 # curl collector :13133 health_check; exit non-zero if not up
  otel-metrics-smoke.sh                          # collector :8889/metrics has reckonna_* series; Prometheus target for the PodMonitor is UP
  otel-trace-smoke.sh                            # fire a request; assert a service span appears (Grafana Cloud Tempo) — manual/live
tests/
  podmonitor_test.sh                             # grep: release label, selector app=otel-collector, targetPort 8889, path /metrics
  otel-contract_test.sh                          # grep: docs pin OTLP endpoint + service.name attrs + the otlpmetrichttp metric-export contract + the egress rule
  grafana-dashboard_test.sh                      # dashboard JSON is valid + panels reference reckonna_* metrics; TF (if used) uses vault data source (no literal token)
docs/otel-telemetry-setup.md                     # topology, OTLP contract (traces+metrics), egress contract, apply order, Grafana Cloud dashboard, rollback
Makefile                                         # + otel-health, otel-metrics-smoke; extend k8s-validate base list with infra/k8s/observability
.github/workflows/ci.yml                          # note: k8s-validate now renders the new base; gitleaks covers new files
```

**Deliberately NOT created** (would churn shared live infra): a replacement `otel-collector`
Deployment/ConfigMap; a label-patch on the shared collector Service (R1 — PodMonitor selects
pods instead); a standalone egress `NetworkPolicy` (R3 — deferred to the backend-Deploy plan
as a contract, since the command/query Deployments don't exist yet); migration of the
collector's existing k8s secrets to Vault Injector; any Prometheus/Grafana-operator install.

---

## Section 1 — Acceptance-test spec (E2E)

E2E tests are **manual until** the manifests + TF are applied (human-only) and the backend
command/query Deployments exist. Once live, the smoke scripts run from CI on a schedule.

| ID  | Given / When / Then | Domain | Test artifact |
|-----|---------------------|--------|---------------|
| AT1 | Given the collector is running / When `curl -sf http://otel-collector.observability.svc:13133` (health_check) / Then it responds healthy. | infra | `scripts/otel-health.sh` |
| AT2 | Given the PodMonitor applied / When you open Prometheus → Targets / Then the `podMonitor/observability/reckonna-otel-collector` target is **UP** and scraping the collector pod `:8889`. | infra | `scripts/otel-metrics-smoke.sh` |
| AT3 | Given a deployed command/query service **with the OTLP metric exporter (D10)** / When a request completes / Then `otel-collector:8889/metrics` exposes `reckonna_*` (http request rate/duration) series, and they appear in Prometheus. | infra+app | `scripts/otel-metrics-smoke.sh` |
| AT4 | Given the dashboard provisioned (D-GRAFANA) / When you open the "Reckonna — RED" dashboard in Grafana Cloud / Then the request-rate, error-ratio, and avg-duration panels render for `reckonna-command` + `reckonna-query`. | infra | manual; `docs/otel-telemetry-setup.md` |
| AT5 | Given a traced request / When it completes / Then a span for `reckonna-command`/`reckonna-query` appears in Grafana Cloud Tempo (via the already-wired `otlp/tempo` exporter). | infra+app | `scripts/otel-trace-smoke.sh` (manual/live) |
| AT6 | **Ledger domain signal:** Given an unbalanced-ledger write is rejected (借方≠貸方) / When the reject path runs / Then a `reckonna_ledger_rejected_total` counter increments and shows on the dashboard. | app+infra | manual; depends on the app exposing the counter (flagged contract) |

## Section 2 — Integration-test spec (static gates, no live apply)

| ID  | Condition to verify | Domain | Test artifact |
|-----|---------------------|--------|---------------|
| IT1 | Every manifest under `infra/k8s/observability/**` passes `kubeconform -strict` (k8s 1.30, `-ignore-missing-schemas` for the PodMonitor CRD). | infra | `make k8s-validate` |
| IT2 | `terraform validate` green for `infra/terraform/grafana-*.tf` (if D-GRAFANA = TF provider). | infra | `make tf-validate` |
| IT3 | `gitleaks` clean on the new files — no literal Grafana token, no remote-write cred. | infra | `gitleaks detect --no-git -s infra/` |
| IT4 | The `PodMonitor` carries label `release: kube-prometheus-stack` (matches the operator's `podMonitorSelector`), `selector.matchLabels: {app: otel-collector}`, `namespaceSelector` → `observability`, and an endpoint on `targetPort: 8889` / `path: /metrics`. | infra | `tests/podmonitor_test.sh` |
| IT5 | The plan adds **no patch/mutation** of the shared `otel-collector` Service — grep asserts no file under `infra/k8s/observability/**` references `kind: Service` named `otel-collector` or patches its labels. (PodMonitor selects pods; nothing shared is edited.) | infra | `tests/podmonitor_test.sh` |
| IT6 | `docs/otel-telemetry-setup.md` pins the OTLP endpoint `otel-collector.observability.svc.cluster.local:4318`, the resource attrs `service.name=reckonna-command` / `reckonna-query`, **and the D10 metric-export contract** (`otlpmetrichttp` + otelgin meter provider; `go.mod` must add `otlpmetrichttp`); it also documents the egress rule (observability `4317`+`4318`) the backend-Deploy plan must ship. | infra+app | `tests/otel-contract_test.sh` |
| IT7 | `dashboards/reckonna-red.json` is valid JSON, its panels reference `reckonna_*` metrics, and it uses avg-duration (`_sum`/`_count`) — no `histogram_quantile`/`_bucket` (respects the live cardinality filter, D9). | infra | `tests/grafana-dashboard_test.sh` |
| IT8 | If D-GRAFANA = TF provider: `grafana-dashboard.tf` sources the Grafana auth token from a `vault_kv_secret_v2` data source — NO literal token, NO `.tfvars`. | infra | `tests/grafana-dashboard_test.sh` + gitleaks |

## Section 3 — Implementation steps (one commit each)

Each step validates standalone via static checks (kubeconform / terraform validate / grep tests).
No live apply in any step.

| ID | Commit (verbatim) | Files | Verify |
|----|-------------------|-------|--------|
| S0 | `docs(plan): infra plan 06 — wire reckonna otlp telemetry into homelab observability` | `plans/06-infra-otel-telemetry.md` | review only |
| S1 | `feat(obs): podmonitor scraping otel-collector metrics (zero shared-infra mutation)` | `infra/k8s/observability/{podmonitor-reckonna-collector,kustomization}.yaml`, `tests/podmonitor_test.sh` | `kubeconform -strict`; IT1; IT4; IT5 grep |
| S2 | `feat(obs): reckonna RED dashboard (json) + grafana provisioning (per D-GRAFANA)` | `infra/k8s/observability/dashboards/reckonna-red.json`, `infra/terraform/grafana-{providers,dashboard}.tf` (if TF path), `tests/grafana-dashboard_test.sh` | `terraform validate`; IT2; IT3 gitleaks; IT7; IT8 grep |
| S3 | `docs(obs): otlp endpoint + metric-export + egress contract for the backend-deploy plan` | `tests/otel-contract_test.sh` (+ contract section in `docs/otel-telemetry-setup.md`) | IT6 grep |
| S4 | `feat(scripts): otel health + metrics-smoke + trace-smoke checks` | `scripts/otel-health.sh`, `scripts/otel-metrics-smoke.sh`, `scripts/otel-trace-smoke.sh` | `shellcheck`; `bash -n` |
| S5 | `chore(make): otel-health + otel-metrics-smoke targets; k8s-validate covers new base` | `Makefile` | `make help` lists new targets; k8s-validate loop includes `infra/k8s/observability` |
| S6 | `docs(infra): otel telemetry topology, otlp contract, apply order, rollback` | `docs/otel-telemetry-setup.md` | manual review; IT6 grep passes |

### Step notes

- **S1 — the actual missing wire [R1].** The live `otel-collector` Service has `spec.selector={app: otel-collector}` but **empty `metadata.labels`**, and it is not repo-owned — kustomize can only patch resources inside its own build, so a label-patch on it is invalid. The collector **pods** carry `app: otel-collector` (verified), and the Prometheus CR's `podMonitorSelector = {release: kube-prometheus-stack}` (verified). So `podmonitor-reckonna-collector.yaml` is a `PodMonitor` with `labels.release: kube-prometheus-stack`, `selector.matchLabels: {app: otel-collector}`, `namespaceSelector.matchNames: [observability]`, `podMetricsEndpoints: [{targetPort: 8889, path: /metrics, interval: 30s}]`. **Zero mutation** of any shared resource. (The collector pod's :8889 container port is unnamed → address it by `targetPort: 8889`.)
- **S2 — Grafana Cloud dashboard as code [per D-GRAFANA].** Ship `dashboards/reckonna-red.json` regardless of path. If D-GRAFANA = TF provider: `grafana-providers.tf` sets the `grafana` provider `url` = Grafana Cloud stack URL, `auth` = token via `data "vault_kv_secret_v2"` from `secret/app/grafana/terraform` (ESO grafana generator mints it; Vault path human-provisioned first, documented S6); `grafana-dashboard.tf` = `resource "grafana_dashboard" "reckonna_red"` from the JSON. If D-GRAFANA = ESO-import: skip the TF files; S6 documents the import. Panels: per-service request rate `sum(rate(reckonna_http_server_requests_total[5m])) by (service_name)`, error ratio (5xx/total), avg duration `rate(_sum)/rate(_count)` (D9 — no buckets), and a ledger panel on `reckonna_ledger_rejected_total`. Confirm the emitted metric names with the backend head at green-time (otelgin metric names may differ).
- **S3 — contract only [R3].** No standalone NetworkPolicy is created here (the command/query Deployments don't exist yet — only the nginx `reckonna-app` harness does; a standalone NP would be an orphan). Instead, `docs/otel-telemetry-setup.md` pins the contract that plan-03 + the backend-Deploy plan must honor: `internal/config/otel.go` sets `OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector.observability.svc.cluster.local:4318` + `OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf`; **both a trace exporter (`otlptracehttp`, already in go.mod) AND a metric exporter (`otlpmetrichttp`, D10 — to be added) are wired**; and the backend-Deploy plan ships the egress NetworkPolicy (`reckonna-backend` → `observability` TCP 4317+4318). `otel-contract_test.sh` greps the doc for all of these.
- **S4 — smokes.** `otel-metrics-smoke.sh`: `kubectl -n observability exec deploy/otel-collector -- wget -qO- localhost:8889/metrics | grep -q reckonna_` AND checks the PodMonitor target is UP (`curl prometheus:9090/api/v1/targets | jq` for `reckonna-otel-collector` health). `otel-health.sh`: hits `:13133`. All skip cleanly when kubectl/cluster is absent (CI-safe), mirroring the plan-02 script convention.
- **S5 — Makefile.** Extend the `k8s-validate` base loop to include `infra/k8s/observability`. Add `otel-health` + `otel-metrics-smoke` phony targets that call the S4 scripts.
- **S6 — docs.** Topology diagram (app → collector:4318 → [prometheus:8889 → PodMonitor → self-hosted Prometheus → remote_write → Grafana Cloud] + [otlp/tempo → Grafana Cloud Tempo]); the metric-export contract (D10); the exact `vault kv` path for the Grafana token (if TF path); apply order (PodMonitor first, then dashboard); rollback = delete the PodMonitor + (if TF) `terraform destroy -target=grafana_dashboard.reckonna_red` (removes only our additions; the shared collector + Prometheus are untouched).

---

## Failure modes

| Codepath | Realistic failure | Test? | Error handling? | User visibility |
|----------|-------------------|-------|-----------------|-----------------|
| PodMonitor selection | Label typo → operator never scrapes :8889 | IT4 grep at PR; AT2 target check live | Prometheus target absent (not errored) | Silent-ish — **AT2 explicitly checks the target is UP** to catch it |
| **App exports traces but not metrics** | S16 lands `otlptracehttp` only; D10 metric exporter forgotten → RED panels empty | IT6 asserts the contract doc; AT3 catches live | Backend contract (D10) + `otel-contract_test.sh`; dashboard renders blank until fixed | **This was the R2 landmine** — empty panels; caught by AT3 + the contract test |
| Cardinality filter drops buckets | Dashboard uses `histogram_quantile` → empty latency panel | IT7 forbids `_bucket`/`histogram_quantile` | D9 uses avg via sum/count | Panel would render blank; caught by IT7 |
| Metric-name drift | otelgin/otlpmetrichttp emit different metric names than the dashboard queries | Partial (IT7 checks JSON refs reckonna_*) | Confirm names with backend head; adjust queries at green-time | Empty panels until query names match |
| Grafana token in a tracked file | Literal SA token committed | IT3/IT8 + gitleaks CI | Vault data source only (if TF path) | Blocked at PR by gitleaks |
| Traces backend outage | Grafana Cloud Tempo unreachable | No (live-only) | collector `otlp/tempo` retries with backoff; metrics unaffected (separate pipeline) | Traces gap in Grafana Cloud; visible in collector logs |
| `targetPort` unsupported by operator | Old operator rejects numeric `targetPort` on PodMonitor | IT1 kubeconform (schema) | Fallback: add a named-port Service we own + ServiceMonitor (additive, no shared mutation) | Target absent; fallback documented in S6 |

**No silent failures flagged** — the two silence risks (PodMonitor label; app-emits-traces-only) are both caught: AT2's target-UP assertion and AT3 + the contract test.

---

## Worktree parallelization strategy

| Step | Modules touched | Depends on |
|------|----------------|------------|
| S0 | plans/ | — |
| S1 | infra/k8s/observability/ (podmonitor) | — |
| S2 | infra/k8s/observability/dashboards/, infra/terraform/ | — |
| S3 | tests/ + docs contract section | — |
| S4 | scripts/, tests/ | — |
| S5 | Makefile | S1 (new base), S4 (new scripts) |
| S6 | docs/ | S1–S3 (describes them) |

**Lanes:** A=S1, B=S2, C=S3, D=S4 — all independent, run in parallel worktrees. S5 lands
after A/D (references the new base + scripts). S6 lands last (documents the rest). Only S5's
`Makefile` sits in a shared file and lands late.

---

## Hand-off to the heads

- **infra-engineer (HEAD):** owns S0–S6. Writes IT1–IT8 (grep/kubeconform) + AT smoke
  scripts FIRST (RED), then greens via `iac-ops` → `code-reviewer`. Applies the
  PodMonitor + Grafana dashboard **manually** post-merge (human-only apply). PodMonitor is
  fully additive — nothing shared is mutated (R1), so no endpoints-diff dance is needed.
- **backend-engineer (HEAD):** owns the OTLP **emit** side (plan 03 S16) — must set the
  endpoint + resource attrs from D2 **and add the OTLP metric exporter (D10: `otlpmetrichttp`
  + otelgin meter provider — `go.mod` currently has only `otlptracehttp`)**, plus (if AT6 is
  wanted) the `reckonna_ledger_rejected_total` counter. Confirms the emitted metric names so
  S2's dashboard queries match. The egress NetworkPolicy ships with the backend-Deploy plan.
- **plan-tracker:** logs each landed step to `06-infra-otel-telemetry.impl.md`.

**"Done" (plan 06)** = IT1–IT8 green; `make k8s-validate` (+ `make tf-validate` if D-GRAFANA
= TF) clean; `gitleaks` clean; docs merged. AT1–AT6 run **manually** post-apply (human-only),
and once the backend Deployments + the D10 metric exporter are up, AT2/AT3 become scheduled
CI smokes. **Blocked on human sign-off of D-GRAFANA** before S2 finalizes its provisioning path.

## NOT in scope (plan 06)

- **A replacement/re-owned collector** — D1 reuses the live shared one; codifying it into
  the repo (importing its full config, migrating its secrets to Vault Injector) is a
  separate follow-up to avoid shared-infra churn.
- **Self-hosted Tempo** — D4 keeps traces on Grafana Cloud Tempo (already working);
  in-cluster Tempo (removing the cloud dependency) is a flagged OPEN follow-up.
- **Loki / log shipping** — D5; no Loki exists.
- **Self-hosting Grafana** — D6; Grafana is Grafana Cloud. A ConfigMap-sidecar dashboard
  path activates only if/when Grafana is brought in-cluster.
- **Histogram-percentile (p95/p99) panels** — blocked by the live `filter/drop_high_cardinality`
  (D9); needs a collector filter exception for `reckonna_*_bucket` — follow-up.
- **The command/query k8s Deployments + the egress NetworkPolicy** — the Deployments don't
  exist yet (only the nginx `reckonna-app` harness does); both the OTLP env injection and the
  egress `NetworkPolicy` land with the backend-Deploy plan (R3). This plan ships the **contract**.
- **The collector `spanmetrics` connector** (the D10 alternative) — would derive RED from spans
  with zero backend work but mutates the shared collector config; rejected here, available as a
  follow-up if adding the app metric exporter proves undesirable.
- **App instrumentation code** (otelgin trace + metric exporter setup) — plan 03 S16 + the D10
  contract; this plan does not write Go code.

## What already exists (live homelab discovery — 2026-07-01, read-only)

- **Namespace `observability`** hosts a **kube-prometheus-stack** (Prometheus Operator).
  CRDs present: `servicemonitors`, `podmonitors`, `prometheusrules`, `scrapeconfigs`,
  `probes`, `alertmanagers` (`monitoring.coreos.com`).
- **Prometheus** (self-hosted): pod `prometheus-kube-prometheus-stack-prometheus-0`,
  Service `kube-prometheus-stack-prometheus:9090`. Its CR has `remoteWrite` →
  **Grafana Cloud** (`https://prometheus-prod-49-prod-ap-northeast-0.grafana.net/api/prom/push`),
  basicAuth from k8s secret `grafana-remote-write-secret`.
  `serviceMonitorSelector.matchLabels = {release: kube-prometheus-stack}` **and
  `podMonitorSelector.matchLabels = {release: kube-prometheus-stack}`** (both verified);
  the SM/PM `namespaceSelector`s are empty `{}` (all namespaces), so objects in
  `observability` are picked up.
- **Grafana** is **Grafana Cloud** — NO in-cluster Grafana pod / Service / Ingress. The ESO
  CRD `grafanas.generators.external-secrets.io` (Grafana Cloud SA-token generator) + the
  Grafana Cloud remote-write/Tempo creds confirm the viz layer is hosted.
- **otel-collector ALREADY EXISTS**: Deployment `otel-collector` (single-replica gateway),
  image `otel/opentelemetry-collector-contrib:0.120.0`, in `observability`. Service
  `otel-collector` exposes `grpc:4317`, `http:4318`, `prometheus:8889`, `health:13133`;
  `spec.selector={app: otel-collector}` but **empty `metadata.labels`** (the gap). The
  **pods** carry `app: otel-collector`; the collector container's `:8889` port is **unnamed**.
  ConfigMap `otel-collector-config`: receivers `otlp` (4317/4318) + `postgresql`
  (`postgres.main.svc:5432`); processors `batch`, `memory_limiter`,
  `filter/drop_high_cardinality` (drops `.*_bucket`, `.*_created`); exporters `prometheus`
  (:8889), `debug`, `otlp/tempo` → Grafana Cloud Tempo (basicauth). Pipelines: traces →
  tempo(cloud); metrics → [prometheus, debug]; logs → [debug]. Secrets via k8s
  `secretKeyRef` (`grafana-remote-write-secret`, `postgres-otel-secret`) — not Vault Injector.
- **No Tempo, no Jaeger, no Loki in-cluster.** Traces go to Grafana Cloud Tempo; there is no
  self-hosted trace or log backend.
- **The gap this plan closes:** the collector exposes app metrics on `:8889` but nothing
  scrapes it (no PodMonitor/ServiceMonitor targets the collector) → app OTLP metrics never
  reach the self-hosted Prometheus (nor, via its remote_write, Grafana Cloud). Traces already
  flow. So the metrics scrape wire + a dashboard are the real deliverables. **[R1]** a
  PodMonitor selecting the pods' `app: otel-collector` label closes it with zero shared-infra
  mutation.
- **Repo side (R2 landmine):** `go.mod` has `otelgin v0.56.0`, `otel v1.32.0`,
  `otlptracehttp v1.32.0`, `otel/sdk v1.32.0` — a **trace exporter only, NO metric exporter**
  (`otlpmetrichttp` absent). Plan 03 S16 wires otelgin **spans** at router setup;
  `internal/config/otel.go` does NOT exist yet (S16 pending). So the RED dashboard has no
  metric source until D10's metric exporter lands. Infra IaC lives under `infra/k8s/**`
  (kustomize) + `infra/terraform/**`; `make k8s-validate` (kubeconform) + `make tf-validate`
  are the gates; gitleaks runs in CI. No observability manifests exist in the repo yet.

---

## GSTACK REVIEW REPORT

| Review | Trigger | Why | Runs | Status | Findings |
|--------|---------|-----|------|--------|----------|
| Eng Review | `/plan-eng-review` | Architecture & tests (required) | 1 | CLEAR (PLAN) | **2 architecture blockers fixed:** R1 — ServiceMonitor + kustomize label-patch on the shared, non-repo-owned collector Service was invalid + mutated shared infra → replaced with a **PodMonitor** selecting pods `app: otel-collector` on `targetPort 8889` (`podMonitorSelector` verified; zero mutation). R2 — the RED dashboard had **no metric source** (otelgin + otlptracehttp emit spans only; `go.mod` has no metric exporter) → locked **D10**: app exports OTLP metrics via `otlpmetrichttp` on the collector's existing `otlp→prometheus` pipeline (backend contract). **1 scope trim:** R3 — dropped the orphan `reckonna-backend` NetworkPolicy → contract only. Files ~18 → ~13. **1 OPEN (R4/D-GRAFANA)** left for human. |
| CEO Review | `/plan-ceo-review` | Scope & strategy | 0 | — | — |
| Codex Review | `/codex review` | Independent 2nd opinion | 0 | — | — |
| Design Review | `/plan-design-review` | UI/UX gaps | 0 | N/A | infra plan — no UI |
| DX Review | `/plan-devex-review` | Developer experience gaps | 0 | — | — |

**UNRESOLVED:** 0 — **D-GRAFANA RESOLVED** (2026-07-01, human): provision the dashboard to the **self-hosted homelab Grafana**, datasource = self-hosted Prometheus. **D10 DISPATCHED** to the live backend head (plan 03 S16 adds `otlpmetrichttp` + meter provider + `reckonna_ledger_rejected_total`). One follow-up test to add: `grafana-dashboard_test.sh` asserts the datasource targets the self-hosted Prometheus.
**VERDICT:** ENG CLEARED — additive *wiring* plan grounded in live discovery. Correction to the discovery: **Grafana IS self-hosted on the homelab** (user directive) — the kubectl inventory was k3s-scoped and saw only Grafana Cloud, so a homelab Grafana outside k3s didn't surface; infra-head confirms its location + reachability at implementation. Standing correction: a **collector already exists + traces already flow to Grafana Cloud Tempo**. Eng review caught two landmines the first draft missed — the invalid shared-Service patch (R1) and the empty-RED-metrics source (R2) — both fixed; the plan touches **no shared live resource** (PodMonitor is fully additive). D10 metric-export contract was pushed to the backend head so plan 03 lands metrics, not a doc-only promise. Traces (D4): keep Grafana Cloud Tempo; self-hosted Tempo flagged OPEN. Ready for human flip of `status: draft → approved` + `approved_by`/`approved_at` before S1.
