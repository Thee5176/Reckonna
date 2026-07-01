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
  metrics_path: collector's EXISTING prometheus exporter (:8889) scraped by the self-hosted kube-prometheus-stack Prometheus via a NEW ServiceMonitor (Operator + CRDs present). NOT prometheusremotewrite from the collector (Prometheus already owns the single remote_write path to Grafana Cloud).
  traces_path: ALREADY WIRED — the live collector exports OTLP → Grafana Cloud Tempo (otlp/tempo). Not dropped, not duplicated. Self-hosted Tempo = OPEN follow-up (out of scope).
  logs_path: OUT of scope — no Loki in cluster; collector logs pipeline is debug-only.
  grafana: Grafana is Grafana CLOUD (NOT self-hosted in-cluster). Dashboard shipped as code (JSON) + provisioned via the Grafana Terraform provider; token from Vault. Drops into a ConfigMap sidecar if Grafana is ever self-hosted.
  secrets: Vault only (secrets-vault.md). New Grafana TF SA token → Vault. Live collector's existing k8s secrets are NOT migrated here (shared-infra churn, out of scope).
  human_only: kubectl apply, terraform apply — manifests + tf + tests only in this plan.
review_log:
  - discovery on 2026-07-01: live homelab inventoried read-only; brief's "self-hosted Grafana" + "new collector" assumptions corrected against reality (Grafana Cloud; collector already exists; traces already flow).
---

# Plan 06 — Wire Reckonna OTLP Telemetry into the Homelab Observability Stack

Connects the Go command+query services' OpenTelemetry output into the **existing**
homelab observability stack so their metrics land in the self-hosted Prometheus (and,
via its remote_write, in Grafana Cloud) and their traces keep flowing to Grafana Cloud
Tempo. The pipeline is **already 90% built** — the missing wire is a **ServiceMonitor**
so Prometheus actually scrapes the collector's metrics endpoint, plus a **Grafana
dashboard as code** and the app→collector **OTLP contract**. Everything is vendor-neutral
(OTLP + OTel Collector; Grafana Cloud is just a swappable OTLP/remote-write sink).
**No `kubectl apply`, no `terraform apply`** — human-only per `devops.md`. Deliverables:
manifests, Terraform, dashboard JSON, scripts, docs, tests.

**Prerequisite:** Plan 01 landed (homelab k3s + Vault Agent Injector). Plan 03 S16 makes
the services emit OTLP (otelgin + otlptracehttp; go.mod already has the deps). The live
`observability` namespace (kube-prometheus-stack + shared collector) exists **outside this
repo** — this plan wires into it additively.

## Decisions (locked at draft, 2026-07-01 — grounded in live discovery)

| # | Decision | Rationale |
|---|----------|-----------|
| D1 | **Reuse the existing shared `otel-collector`** (gateway Deployment, contrib 0.120.0, observability ns). NOT a per-node DaemonSet, NOT a second collector. | A single gateway is right for a small homelab; the collector already receives OTLP (4317/4318), scrapes homelab postgres, exports metrics to :8889 and traces to Grafana Cloud Tempo. Fragmenting or re-owning it would churn shared homelab observability (it serves more than Reckonna). Additive wiring only. |
| D2 | **App → collector**: command+query export OTLP/HTTP to `http://otel-collector.observability.svc.cluster.local:4318` (grpc 4317 open too); resource attrs `service.name=reckonna-command` / `reckonna-query`, `deployment.environment=homelab`. | Plan 03 S16 wires otelgin + `otlptracehttp`; this plan pins the destination + identity. The env injection into the command/query Deployments lands with the backend-Deploy plan (those Deployments don't exist yet) — this plan owns the **contract** + the egress NetworkPolicy. |
| D3 | **METRICS = collector `prometheus` exporter (:8889) scraped by the self-hosted Prometheus via a NEW `ServiceMonitor`.** NOT `prometheusremotewrite` from the collector. | The Prometheus Operator + `ServiceMonitor`/`PodMonitor` CRDs are present. The collector already exposes :8889 but **nothing scrapes it** (the collector Service has no `metadata.labels`, so it matches no ServiceMonitor). Prometheus already remote-writes the single path to Grafana Cloud — adding a second write from the collector would double-ship. So: scrape :8889 → Prometheus → its existing remote_write → Grafana Cloud. |
| D4 | **TRACES = already wired to Grafana Cloud Tempo** (collector `otlp/tempo` exporter). This plan adds **no** trace backend. **Self-hosted Tempo is an OPEN follow-up, explicitly OUT of scope** (not a silent drop). | Traces already flow; OTLP is the vendor-neutral wire, Grafana Cloud Tempo is a swappable sink. Self-hosted Tempo would remove the cloud dependency but adds a stateful service + object storage — unjustified for a small homelab today. Flagged, not dropped. |
| D5 | **LOGS = NOT in scope.** | No Loki in the cluster; the collector's logs pipeline is `debug`-only. Structured-log shipping is a follow-up if/when Loki is added. |
| D6 | **Grafana is Grafana CLOUD** (no in-cluster Grafana pod/svc/ingress; ESO grafana generator + Grafana Cloud remote_write creds confirm it). Dashboard shipped as **versioned JSON** + provisioned via the **Grafana Terraform provider** (`grafana_dashboard`); the Prometheus/Tempo datasources already exist in Grafana Cloud. | Matches how the stack already integrates with Grafana Cloud (Prometheus remote_write, ESO SA-token generator). If Grafana is ever self-hosted, the same JSON drops into a ConfigMap sidecar with a one-line switch. |
| D7 | **All secrets via Vault** (`secrets-vault.md`). The only NEW secret is the Grafana TF SA token → `secret/app/grafana/terraform`. The live collector's existing k8s secrets (`grafana-remote-write-secret`, `postgres-otel-secret`) are **NOT migrated** in this plan. | No literal secret in any manifest/tfvars. Migrating live shared secrets to Vault Injector is shared-infra churn — separate follow-up, not this plan. |
| D8 | **Human-only**: `kubectl apply`, `terraform apply`. One step = one commit = one `Plan: S<n>` trailer. Conventional Commits. | `devops.md`. |
| D9 | **RED latency uses avg (sum/count), NOT `histogram_quantile`.** | The live collector runs `filter/drop_high_cardinality` which drops `.*_bucket` + `.*_created` to stay under the Grafana Cloud free-tier 10k-series cap. Histogram buckets are gone → p50/p95/p99 are unavailable. The dashboard uses request-rate + error-ratio + avg-latency. A filter exception to keep `reckonna_*_bucket` for percentiles is a flagged follow-up. |

## File structure

```
plans/06-infra-otel-telemetry.md                 # this file
infra/k8s/observability/
  servicemonitor-reckonna-collector.yaml         # scrape otel-collector:8889; labeled release=kube-prometheus-stack
  otel-collector-service-label.yaml              # kustomize patch: add metadata.labels.app=otel-collector to the live Service (metadata-only, non-disruptive)
  kustomization.yaml                             # bases the patch + servicemonitor
  dashboards/
    reckonna-red.json                            # RED panels (rate/errors/avg-duration) per service + ledger domain counters
infra/k8s/reckonna-backend/                       # egress wiring for the command/query pods (applies when those Deployments land)
  networkpolicy-egress-otel.yaml                 # allow egress reckonna-backend → observability :4317/:4318
  kustomization.yaml
infra/terraform/
  grafana-providers.tf                           # grafana provider; auth token via `vault_kv_secret_v2` data source (Grafana Cloud)
  grafana-dashboard.tf                           # grafana_dashboard from dashboards/reckonna-red.json
scripts/
  otel-health.sh                                 # curl collector :13133 health_check; exit non-zero if not up
  otel-metrics-smoke.sh                          # collector :8889/metrics has reckonna_* series; Prometheus target for the ServiceMonitor is UP
  otel-trace-smoke.sh                            # fire a request; assert a service span appears (Grafana Cloud Tempo) — manual/live
tests/
  servicemonitor_test.sh                         # grep: release label, selector app=otel-collector, port prometheus, path /metrics
  otel-contract_test.sh                          # grep: docs pin OTLP endpoint + service.name resource attrs; NetworkPolicy allows :4318
  grafana-dashboard_test.sh                      # dashboard JSON is valid + panels reference reckonna_* metrics; TF uses vault data source (no literal token)
docs/otel-telemetry-setup.md                     # topology, OTLP contract, apply order, Grafana Cloud dashboard import, rollback
Makefile                                         # + otel-health, otel-metrics-smoke; extend k8s-validate base list with infra/k8s/observability + infra/k8s/reckonna-backend
.github/workflows/ci.yml                          # note: k8s-validate now renders the two new bases; gitleaks covers new files
```

**Deliberately NOT created** (would churn shared live infra): a replacement `otel-collector`
Deployment/ConfigMap; migration of the collector's existing k8s secrets to Vault Injector;
any Prometheus/Grafana-operator install.

---

## Section 1 — Acceptance-test spec (E2E)

E2E tests are **manual until** the manifests + TF are applied (human-only) and the backend
command/query Deployments exist. Once live, the smoke scripts run from CI on a schedule.

| ID  | Given / When / Then | Domain | Test artifact |
|-----|---------------------|--------|---------------|
| AT1 | Given the collector is running / When `curl -sf http://otel-collector.observability.svc:13133` (health_check) / Then it responds healthy. | infra | `scripts/otel-health.sh` |
| AT2 | Given the ServiceMonitor applied / When you open Prometheus → Targets / Then the `reckonna-otel-collector` target is **UP** and scraping `:8889/metrics`. | infra | `scripts/otel-metrics-smoke.sh` |
| AT3 | Given a request to a deployed command/query service / When it completes / Then `otel-collector:8889/metrics` exposes `reckonna_*` (http request rate/duration) series, and they appear in Prometheus. | infra+app | `scripts/otel-metrics-smoke.sh` |
| AT4 | Given the Grafana TF applied / When you open the "Reckonna — RED" dashboard in Grafana Cloud / Then the request-rate, error-ratio, and avg-duration panels render for `reckonna-command` + `reckonna-query`. | infra | manual; `docs/otel-telemetry-setup.md` |
| AT5 | Given a traced request / When it completes / Then a span for `reckonna-command`/`reckonna-query` appears in Grafana Cloud Tempo (via the already-wired `otlp/tempo` exporter). | infra+app | `scripts/otel-trace-smoke.sh` (manual/live) |
| AT6 | **Ledger domain signal:** Given an unbalanced-ledger write is rejected (借方≠貸方) / When the reject path runs / Then a `reckonna_ledger_rejected_total` counter increments and shows on the dashboard. | app+infra | manual; depends on the app exposing the counter (flagged contract) |

## Section 2 — Integration-test spec (static gates, no live apply)

| ID  | Condition to verify | Domain | Test artifact |
|-----|---------------------|--------|---------------|
| IT1 | Every manifest under `infra/k8s/observability/**` + `infra/k8s/reckonna-backend/**` passes `kubeconform -strict` (k8s 1.30, `-ignore-missing-schemas` for the ServiceMonitor CRD). | infra | `make k8s-validate` |
| IT2 | `terraform validate` green for `infra/terraform/grafana-*.tf`. | infra | `make tf-validate` |
| IT3 | `gitleaks` clean on the new files — no literal Grafana token, no remote-write cred. | infra | `gitleaks detect --no-git -s infra/` |
| IT4 | The `ServiceMonitor` carries label `release: kube-prometheus-stack` (matches the operator's `serviceMonitorSelector`), `selector.matchLabels: {app: otel-collector}`, `namespaceSelector` → `observability`, and an endpoint `port: prometheus` / `path: /metrics`. | infra | `tests/servicemonitor_test.sh` |
| IT5 | The Service-label patch adds `metadata.labels.app: otel-collector` and touches **only** `metadata.labels` (no `spec.selector`/`spec.ports` change) — non-disruptive. | infra | `tests/servicemonitor_test.sh` |
| IT6 | `docs/otel-telemetry-setup.md` pins the OTLP endpoint `otel-collector.observability.svc.cluster.local:4318` and the resource attrs `service.name=reckonna-command` / `reckonna-query`; the egress `NetworkPolicy` allows egress to `observability` on `4317` + `4318`. | infra+app | `tests/otel-contract_test.sh` |
| IT7 | `dashboards/reckonna-red.json` is valid JSON, its panels reference `reckonna_*` metrics, and it uses avg-duration (`_sum`/`_count`) — no `histogram_quantile`/`_bucket` (respects the live cardinality filter, D9). | infra | `tests/grafana-dashboard_test.sh` |
| IT8 | `grafana-dashboard.tf` sources the Grafana auth token from a `vault_kv_secret_v2` data source — NO literal token, NO `.tfvars`. | infra | `tests/grafana-dashboard_test.sh` + gitleaks |

## Section 3 — Implementation steps (one commit each)

Each step validates standalone via static checks (kubeconform / terraform validate / grep tests).
No live apply in any step.

| ID | Commit (verbatim) | Files | Verify |
|----|-------------------|-------|--------|
| S0 | `docs(plan): infra plan 06 — wire reckonna otlp telemetry into homelab observability` | `plans/06-infra-otel-telemetry.md` | review only |
| S1 | `feat(obs): servicemonitor scraping otel-collector metrics + non-disruptive service label` | `infra/k8s/observability/{servicemonitor-reckonna-collector,otel-collector-service-label,kustomization}.yaml`, `tests/servicemonitor_test.sh` | `kubeconform -strict`; IT1; IT4; IT5 grep |
| S2 | `feat(obs): reckonna RED dashboard (json) + grafana terraform provisioning` | `infra/k8s/observability/dashboards/reckonna-red.json`, `infra/terraform/grafana-providers.tf`, `infra/terraform/grafana-dashboard.tf`, `tests/grafana-dashboard_test.sh` | `terraform validate`; IT2; IT3 gitleaks; IT7; IT8 grep |
| S3 | `feat(obs): otlp egress networkpolicy for reckonna-backend + endpoint contract` | `infra/k8s/reckonna-backend/{networkpolicy-egress-otel,kustomization}.yaml`, `tests/otel-contract_test.sh` | `kubeconform`; IT1; IT6 grep |
| S4 | `feat(scripts): otel health + metrics-smoke + trace-smoke checks` | `scripts/otel-health.sh`, `scripts/otel-metrics-smoke.sh`, `scripts/otel-trace-smoke.sh` | `shellcheck`; `bash -n` |
| S5 | `chore(make): otel-health + otel-metrics-smoke targets; k8s-validate covers new bases` | `Makefile` | `make help` lists new targets; k8s-validate loop includes the 2 new bases |
| S6 | `docs(infra): otel telemetry topology, otlp contract, apply order, rollback` | `docs/otel-telemetry-setup.md` | manual review; IT6 grep passes |

### Step notes

- **S1 — the actual missing wire.** The live `otel-collector` Service has `spec.selector={app: otel-collector}` but **empty `metadata.labels`**, so no ServiceMonitor selects it. `otel-collector-service-label.yaml` is a kustomize `patch` (strategic-merge on `metadata.labels` only) adding `app: otel-collector` — it does NOT touch selector or ports, so it is non-disruptive to the running collector (verify with `kubectl get endpoints otel-collector -n observability` before/after: identical). The `ServiceMonitor` (`reckonna-otel-collector`) carries `labels.release: kube-prometheus-stack` (to be discovered by the operator's `serviceMonitorSelector.matchLabels`), `selector.matchLabels: {app: otel-collector}`, `namespaceSelector.matchNames: [observability]`, `endpoints: [{port: prometheus, path: /metrics, interval: 30s}]`.
- **S2 — Grafana Cloud dashboard as code.** `grafana-providers.tf` configures the `grafana` provider with `url` = the Grafana Cloud stack URL and `auth` = a token pulled via `data "vault_kv_secret_v2"` from `secret/app/grafana/terraform` (the ESO grafana generator already mints such tokens; the Vault path is human-provisioned first, documented in S6). `grafana-dashboard.tf` declares `resource "grafana_dashboard" "reckonna_red"` reading `dashboards/reckonna-red.json`. Panels: per-service request rate `sum(rate(reckonna_http_server_requests_total[5m])) by (service_name)`, error ratio (5xx / total), avg duration `rate(_sum)/rate(_count)` (D9 — no buckets), and a ledger panel on `reckonna_ledger_rejected_total`. Metric names are the contract the app must emit (flag to backend head if otelgin's default names differ — adjust the queries to the emitted names at green-time).
- **S3 — egress + contract.** `networkpolicy-egress-otel.yaml` targets the command/query pods (namespace `reckonna-backend`, podSelector on the app label) and allows egress to the `observability` namespace on TCP `4317` + `4318`. This applies only once the backend Deployments land (they don't exist yet — only the nginx `reckonna-app` harness does); until then it is a ready, validated manifest. The doc pins the OTLP endpoint + resource attrs that plan-03's `internal/config/otel.go` must set (`OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector.observability.svc.cluster.local:4318`, `OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf`).
- **S4 — smokes.** `otel-metrics-smoke.sh`: `kubectl -n observability exec deploy/otel-collector -- wget -qO- localhost:8889/metrics | grep -q reckonna_` AND checks the Prometheus target is UP (`curl prometheus:9090/api/v1/targets | jq` for `reckonna-otel-collector` health). `otel-health.sh`: hits `:13133`. All skip cleanly when kubectl/cluster is absent (CI-safe), mirroring the plan-02 script convention.
- **S5 — Makefile.** Extend the `k8s-validate` base loop to include `infra/k8s/observability infra/k8s/reckonna-backend`. Add `otel-health` + `otel-metrics-smoke` phony targets that call the S4 scripts.
- **S6 — docs.** Topology diagram (app → collector:4318 → [prometheus:8889 → ServiceMonitor → self-hosted Prometheus → remote_write → Grafana Cloud] + [otlp/tempo → Grafana Cloud Tempo]); the exact `vault kv` path for the Grafana token; apply order (label+ServiceMonitor first, then TF dashboard); rollback = delete the ServiceMonitor + `terraform destroy -target=grafana_dashboard.reckonna_red` (removes only our additions; the shared collector + Prometheus are untouched).

---

## Failure modes

| Codepath | Realistic failure | Test? | Error handling? | User visibility |
|----------|-------------------|-------|-----------------|-----------------|
| ServiceMonitor selection | Label typo → operator never scrapes :8889 | IT4/IT5 grep at PR; AT2 target check live | Prometheus target absent (not errored) | Silent-ish — **AT2 explicitly checks the target is UP** to catch it |
| Service-label patch | Patch accidentally rewrites `spec.selector` → collector loses endpoints | IT5 asserts metadata-only | endpoints diff before/after apply (documented in S6) | `kubectl get endpoints` shows empty; app metrics stop |
| Cardinality filter drops buckets | Dashboard uses `histogram_quantile` → empty latency panel | IT7 forbids `_bucket`/`histogram_quantile` | D9 uses avg via sum/count | Panel would render blank; caught by IT7 |
| Metric-name drift | otelgin emits different metric names than the dashboard queries | Partial (IT7 checks JSON refs reckonna_*) | Adjust queries to emitted names at green-time | Empty panels until query names match |
| Grafana token in a tracked file | Literal SA token committed | IT3/IT8 + gitleaks CI | Vault data source only | Blocked at PR by gitleaks |
| Traces backend outage | Grafana Cloud Tempo unreachable | No (live-only) | collector `otlp/tempo` retries with backoff; metrics unaffected (separate pipeline) | Traces gap in Grafana Cloud; visible in collector logs |
| Backend Deployments absent | NetworkPolicy/contract has no pods to bind yet | N/A | Manifest is inert until backend pods land | No effect; documented as downstream-activated |

**No silent failures flagged** — the one selection-silence risk (ServiceMonitor label) is caught by AT2's explicit target-UP assertion.

---

## Worktree parallelization strategy

| Step | Modules touched | Depends on |
|------|----------------|------------|
| S0 | plans/ | — |
| S1 | infra/k8s/observability/ | — |
| S2 | infra/k8s/observability/dashboards/, infra/terraform/ | — |
| S3 | infra/k8s/reckonna-backend/, docs contract | — |
| S4 | scripts/, tests/ | — |
| S5 | Makefile | S1+S3 (new bases), S4 (new scripts) |
| S6 | docs/ | S1–S3 (describes them) |

**Lanes:** A=S1, B=S2, C=S3, D=S4 — all independent, run in parallel worktrees. S5 lands
after A/C/D (references new bases + scripts). S6 lands last (documents the rest). Only S5's
`Makefile` sits in a shared file and lands late.

---

## Hand-off to the heads

- **infra-engineer (HEAD):** owns S0–S6. Writes IT1–IT8 (grep/kubeconform) + AT smoke
  scripts FIRST (RED), then greens via `iac-ops` → `code-reviewer`. Applies the
  ServiceMonitor + label + Grafana TF **manually** post-merge (human-only apply). Before
  applying the Service-label patch, captures `kubectl get endpoints otel-collector -n
  observability` to prove non-disruption (failure-mode row 2).
- **backend-engineer (HEAD):** owns the OTLP **emit** side (plan 03 S16) — must set the
  endpoint + resource attrs from D2 and (if AT6 is wanted) expose `reckonna_ledger_rejected_total`.
  Confirms the emitted metric names so S2's dashboard queries match.
- **plan-tracker:** logs each landed step to `06-infra-otel-telemetry.impl.md`.

**"Done" (plan 06)** = IT1–IT8 green; `make k8s-validate` + `make tf-validate` clean;
`gitleaks` clean; docs merged. AT1–AT6 run **manually** post-apply (human-only), and once
the backend Deployments + live wiring are up, AT2/AT3 become scheduled CI smokes.

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
- **The command/query k8s Deployments** — they don't exist yet (only the nginx `reckonna-app`
  harness does); the OTLP env injection lands with the backend-Deploy plan. This plan ships
  the contract + egress policy, ready to bind.
- **App instrumentation code** (otelgin, exporter setup) — plan 03 S16.

## What already exists (live homelab discovery — 2026-07-01, read-only)

- **Namespace `observability`** hosts a **kube-prometheus-stack** (Prometheus Operator).
  CRDs present: `servicemonitors`, `podmonitors`, `prometheusrules`, `scrapeconfigs`,
  `probes`, `alertmanagers` (`monitoring.coreos.com`).
- **Prometheus** (self-hosted): pod `prometheus-kube-prometheus-stack-prometheus-0`,
  Service `kube-prometheus-stack-prometheus:9090`. Its CR has `remoteWrite` →
  **Grafana Cloud** (`https://prometheus-prod-49-prod-ap-northeast-0.grafana.net/api/prom/push`),
  basicAuth from k8s secret `grafana-remote-write-secret`.
  `serviceMonitorSelector.matchLabels = {release: kube-prometheus-stack}`.
- **Grafana** is **Grafana Cloud** — NO in-cluster Grafana pod / Service / Ingress. The ESO
  CRD `grafanas.generators.external-secrets.io` (Grafana Cloud SA-token generator) + the
  Grafana Cloud remote-write/Tempo creds confirm the viz layer is hosted.
- **otel-collector ALREADY EXISTS**: Deployment `otel-collector` (single-replica gateway),
  image `otel/opentelemetry-collector-contrib:0.120.0`, in `observability`. Service
  `otel-collector` exposes `grpc:4317`, `http:4318`, `prometheus:8889`, `health:13133`;
  `spec.selector={app: otel-collector}` but **empty `metadata.labels`** (the gap).
  ConfigMap `otel-collector-config`: receivers `otlp` (4317/4318) + `postgresql`
  (`postgres.main.svc:5432`); processors `batch`, `memory_limiter`,
  `filter/drop_high_cardinality` (drops `.*_bucket`, `.*_created`); exporters `prometheus`
  (:8889), `debug`, `otlp/tempo` → Grafana Cloud Tempo (basicauth). Pipelines: traces →
  tempo(cloud); metrics → [prometheus, debug]; logs → [debug]. Secrets via k8s
  `secretKeyRef` (`grafana-remote-write-secret`, `postgres-otel-secret`) — not Vault Injector.
- **No Tempo, no Jaeger, no Loki in-cluster.** Traces go to Grafana Cloud Tempo; there is no
  self-hosted trace or log backend.
- **The gap this plan closes:** the collector exposes app metrics on `:8889` but the
  collector Service is **unlabeled**, so **no ServiceMonitor scrapes it** → app OTLP metrics
  never reach the self-hosted Prometheus (nor, via its remote_write, Grafana Cloud). Traces
  already flow. So the metrics scrape wire + a dashboard are the real deliverables.
- **Repo side:** `go.mod` has `otelgin v0.56.0`, `otel v1.32.0`, `otlptracehttp v1.32.0`,
  `otel/sdk v1.32.0`. Plan 03 S16 wires otelgin at router setup. `internal/config/otel.go`
  does NOT exist yet (S16 pending). Infra IaC lives under `infra/k8s/**` (kustomize) +
  `infra/terraform/**`; `make k8s-validate` (kubeconform) + `make tf-validate` are the gates;
  gitleaks runs in CI. No observability manifests exist in the repo yet.

---

## GSTACK REVIEW REPORT

| Review | Trigger | Why | Runs | Status | Findings |
|--------|---------|-----|------|--------|----------|
| Eng Review | `/plan-eng-review` | Architecture & tests (required) | 0 | PENDING | to run before `status: draft → approved` |
| CEO Review | `/plan-ceo-review` | Scope & strategy | 0 | — | — |
| Codex Review | `/codex review` | Independent 2nd opinion | 0 | — | — |
| Design Review | `/plan-design-review` | UI/UX gaps | 0 | N/A | infra plan — no UI |
| DX Review | `/plan-devex-review` | Developer experience gaps | 0 | — | — |

**UNRESOLVED:** 0
**VERDICT:** DRAFT — grounded in live homelab discovery. Two brief-vs-reality corrections
surfaced for human sign-off: (1) **Grafana is Grafana Cloud, not self-hosted**; (2) a
**collector already exists and traces already flow to Grafana Cloud Tempo** — so this plan is
an additive *wiring* plan (ServiceMonitor + dashboard + contract), not a from-scratch build.
Traces-backend decision (D4): keep Grafana Cloud Tempo; self-hosted Tempo flagged OPEN.
Awaiting `/plan-eng-review` + human flip of `status` + `approved_by`/`approved_at` before S1.
