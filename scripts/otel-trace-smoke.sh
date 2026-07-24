#!/usr/bin/env bash
# scripts/otel-trace-smoke.sh — AT5: fire a request at a deployed reckonna
# command/query service and assert a matching span reaches Grafana Cloud
# Tempo (via the collector's already-wired `otlp/tempo` exporter, D4).
#
# LIVE-ONLY. Needs a deployed command/query service (ships with the
# backend-Deploy plan, not this one) and Grafana Cloud Tempo query
# credentials. Skips cleanly (exit 0) whenever either is unavailable, so it
# is safe to wire into CI ahead of those landing — mirrors the plan-02
# tunnel-* script convention.
#
# Required env (operator supplies at run-time; never hardcode a token):
#   RECKONNA_TARGET_URL    URL of a live command/query endpoint to hit
#   TEMPO_QUERY_URL        Grafana Cloud Tempo query API base,
#                          e.g. https://tempo-prod-XX.grafana.net
#   TEMPO_QUERY_TOKEN      Grafana Cloud API token with Tempo read scope —
#                          read at use-time from Vault, never committed:
#                            vault kv get -mount=secret -field=token app/grafana/tempo
# Optional env:
#   RECKONNA_SERVICE_NAME  service.name to search for (default: reckonna-command)
#
# Exit codes:
#   0  matching span found, OR skipped (missing target/creds/tool)
#   1  request to the target failed
#   2  Tempo query failed (network/auth)
#   3  no matching span found within the poll window
#
set -euo pipefail

command -v curl >/dev/null 2>&1 || { echo "otel-trace-smoke: curl not on PATH — skipping"; exit 0; }
command -v jq   >/dev/null 2>&1 || { echo "otel-trace-smoke: jq not on PATH — skipping"; exit 0; }

TARGET="${RECKONNA_TARGET_URL:-}"
TEMPO_URL="${TEMPO_QUERY_URL:-}"
TEMPO_TOKEN="${TEMPO_QUERY_TOKEN:-}"  # from Vault: app/grafana/tempo — never hardcode
SERVICE="${RECKONNA_SERVICE_NAME:-reckonna-command}"

if [[ -z "$TARGET" || -z "$TEMPO_URL" || -z "$TEMPO_TOKEN" ]]; then
  cat >&2 <<'EOF'
otel-trace-smoke: skipped — this is a manual/live-only check (AT5).
Requires:
  RECKONNA_TARGET_URL   a live command/query endpoint
  TEMPO_QUERY_URL       Grafana Cloud Tempo query API base
  TEMPO_QUERY_TOKEN     read at use-time from Vault:
                        vault kv get -mount=secret -field=token app/grafana/tempo
Run manually once the backend-Deploy plan ships the command/query
Deployments. See docs/otel-telemetry-setup.md.
EOF
  exit 0
fi

echo "otel-trace-smoke: firing request at $TARGET"
if ! curl -sf --max-time 10 "$TARGET" -o /dev/null; then
  echo "otel-trace-smoke: request to $TARGET failed" >&2
  exit 1
fi

echo "otel-trace-smoke: polling Tempo for a $SERVICE span..."
START_EPOCH=$(( $(date +%s) - 60 ))  # 60s lookback window

for _ in 1 2 3 4 5; do
  RESP=$(curl -sf --max-time 10 \
    -H "Authorization: Bearer $TEMPO_TOKEN" \
    "$TEMPO_URL/api/search?tags=service.name%3D$SERVICE&start=$START_EPOCH&end=$(date +%s)" 2>/dev/null || true)
  if [[ -z "$RESP" ]]; then
    echo "otel-trace-smoke: Tempo query failed" >&2
    exit 2
  fi
  COUNT=$(echo "$RESP" | jq '.traces | length' 2>/dev/null || echo 0)
  if [[ "$COUNT" -gt 0 ]]; then
    echo "otel-trace-smoke: OK — found $COUNT trace(s) for service.name=$SERVICE"
    exit 0
  fi
  sleep 3
done

echo "otel-trace-smoke: no $SERVICE span found in Tempo within the poll window" >&2
exit 3
