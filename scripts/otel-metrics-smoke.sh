#!/usr/bin/env bash
# scripts/otel-metrics-smoke.sh — AT2 + AT3: the collector exposes reckonna_*
# metrics on :8889, and the self-hosted Prometheus is actually scraping them
# via the `reckonna-otel-collector` PodMonitor.
#
# Two checks, both against the live cluster:
#   1. `kubectl exec` into the collector and grep its own /metrics for a
#      reckonna_* series (literal command per plan 06 S4 step note).
#   2. Port-forward the self-hosted Prometheus Service and confirm the
#      PodMonitor's scrape target reports health="up".
#
# Skips cleanly (exit 0) when kubectl/curl/jq are absent or the
# cluster/namespace is unreachable — CI-safe, mirrors the plan-02 tunnel-*
# script convention.
#
# Env overrides:
#   RECKONNA_OTEL_NS       collector + PodMonitor namespace (default: observability)
#   RECKONNA_OTEL_DEPLOY   collector Deployment ref (default: deploy/otel-collector)
#   RECKONNA_PROM_SVC      Prometheus Service name (default: kube-prometheus-stack-prometheus)
#   RECKONNA_PODMONITOR_JOB PodMonitor name / scrape-pool substring (default: reckonna-otel-collector)
#
# Exit codes:
#   0  both checks pass, OR skipped (no kubectl/curl/jq/cluster)
#   1  collector /metrics has no reckonna_* series
#   2  Prometheus port-forward or targets fetch failed
#   3  PodMonitor target is not health="up" (or not found)
#
set -euo pipefail

NAMESPACE="${RECKONNA_OTEL_NS:-observability}"
COLLECTOR_DEPLOY="${RECKONNA_OTEL_DEPLOY:-deploy/otel-collector}"
PROM_SVC="${RECKONNA_PROM_SVC:-kube-prometheus-stack-prometheus}"
PODMONITOR_JOB="${RECKONNA_PODMONITOR_JOB:-reckonna-otel-collector}"

command -v kubectl >/dev/null 2>&1 || {
  echo "otel-metrics-smoke: kubectl not on PATH — skipping (no cluster)"; exit 0
}
command -v curl >/dev/null 2>&1 || {
  echo "otel-metrics-smoke: curl not on PATH — skipping"; exit 0
}
command -v jq >/dev/null 2>&1 || {
  echo "otel-metrics-smoke: jq not on PATH — skipping"; exit 0
}
kubectl get namespace "$NAMESPACE" >/dev/null 2>&1 || {
  echo "otel-metrics-smoke: namespace '$NAMESPACE' unreachable — skipping (no cluster)"; exit 0
}

echo "otel-metrics-smoke: check 1/2 — collector /metrics has a reckonna_* series"
if ! kubectl -n "$NAMESPACE" exec "$COLLECTOR_DEPLOY" -- wget -qO- localhost:8889/metrics 2>/dev/null | grep -q 'reckonna_'; then
  echo "otel-metrics-smoke: no reckonna_* series on $COLLECTOR_DEPLOY:8889/metrics" >&2
  echo "                    (backend contract D10 not landed yet? see docs/otel-telemetry-setup.md)" >&2
  exit 1
fi
echo "otel-metrics-smoke: check 1/2 OK"

echo "otel-metrics-smoke: check 2/2 — Prometheus target for the PodMonitor is up"
LOGFILE="$(mktemp)"
cleanup() {
  if [[ -n "${PF_PID:-}" ]]; then
    kill "$PF_PID" >/dev/null 2>&1 || true
    wait "$PF_PID" 2>/dev/null || true
  fi
  rm -f "$LOGFILE"
}
trap cleanup EXIT

kubectl -n "$NAMESPACE" port-forward "svc/$PROM_SVC" :9090 >"$LOGFILE" 2>&1 &
PF_PID=$!

LOCAL_PORT=""
for _ in $(seq 1 20); do
  LOCAL_PORT=$(grep -oE 'Forwarding from 127\.0\.0\.1:[0-9]+' "$LOGFILE" 2>/dev/null | head -1 | grep -oE '[0-9]+$' || true)
  if [[ -n "$LOCAL_PORT" ]]; then
    break
  fi
  sleep 0.5
done

if [[ -z "$LOCAL_PORT" ]]; then
  echo "otel-metrics-smoke: port-forward to $PROM_SVC:9090 did not establish" >&2
  exit 2
fi

TARGETS_JSON=$(curl -sf --max-time 5 "http://127.0.0.1:$LOCAL_PORT/api/v1/targets" 2>/dev/null || true)
if [[ -z "$TARGETS_JSON" ]]; then
  echo "otel-metrics-smoke: could not fetch /api/v1/targets from Prometheus" >&2
  exit 2
fi

HEALTH=$(echo "$TARGETS_JSON" | jq -r --arg job "$PODMONITOR_JOB" \
  '.data.activeTargets[] | select(.scrapePool | test($job)) | .health' 2>/dev/null | head -1 || true)

if [[ "$HEALTH" != "up" ]]; then
  echo "otel-metrics-smoke: PodMonitor target '$PODMONITOR_JOB' not up (health=${HEALTH:-<not found>})" >&2
  exit 3
fi

echo "otel-metrics-smoke: check 2/2 OK (target health=up)"
echo "otel-metrics-smoke: OK — reckonna_* metrics flowing and scraped"
