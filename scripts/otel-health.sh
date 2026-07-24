#!/usr/bin/env bash
# scripts/otel-health.sh — AT1: the shared otel-collector's health_check
# extension (:13133) reports healthy.
#
# Port-forwards the observability-namespace otel-collector Service to a local
# ephemeral port, curls it, then tears the port-forward down. Skips cleanly
# (exit 0) when kubectl/curl are absent or the cluster/namespace is
# unreachable, so it is CI-safe (mirrors the plan-02 tunnel-* script
# convention).
#
# Env overrides:
#   RECKONNA_OTEL_NS   collector namespace (default: observability)
#   RECKONNA_OTEL_SVC  collector Service name (default: otel-collector)
#
# Exit codes:
#   0  healthy, OR skipped (no kubectl/curl/cluster)
#   1  port-forward failed to establish
#   2  health_check endpoint did not respond OK
#
set -euo pipefail

NAMESPACE="${RECKONNA_OTEL_NS:-observability}"
SERVICE="${RECKONNA_OTEL_SVC:-otel-collector}"

command -v kubectl >/dev/null 2>&1 || {
  echo "otel-health: kubectl not on PATH — skipping (no cluster)"; exit 0
}
command -v curl >/dev/null 2>&1 || {
  echo "otel-health: curl not on PATH — skipping"; exit 0
}
kubectl get namespace "$NAMESPACE" >/dev/null 2>&1 || {
  echo "otel-health: namespace '$NAMESPACE' unreachable — skipping (no cluster)"; exit 0
}

LOGFILE="$(mktemp)"
cleanup() {
  if [[ -n "${PF_PID:-}" ]]; then
    kill "$PF_PID" >/dev/null 2>&1 || true
    wait "$PF_PID" 2>/dev/null || true
  fi
  rm -f "$LOGFILE"
}
trap cleanup EXIT

kubectl -n "$NAMESPACE" port-forward "svc/$SERVICE" :13133 >"$LOGFILE" 2>&1 &
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
  echo "otel-health: port-forward to $SERVICE:13133 did not establish" >&2
  cat "$LOGFILE" >&2
  exit 1
fi

if ! curl -sf --max-time 5 "http://127.0.0.1:$LOCAL_PORT/" >/dev/null; then
  echo "otel-health: health_check endpoint did not respond OK" >&2
  exit 2
fi

echo "otel-health: OK ($SERVICE.$NAMESPACE:13133 healthy)"
