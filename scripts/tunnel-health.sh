#!/usr/bin/env bash
# scripts/tunnel-health.sh — AT1: the public tunnel serves /healthz as HTTP 200 {"status":"ok"}.
# Run from anywhere with internet (goes through the Cloudflare edge, not the origin).
#
# Exit codes: 0 ok | 1 request failed | 2 unexpected body
set -euo pipefail

URL="${RECKONNA_URL:-https://reckonna.thee5176.com}"

body=$(curl -sf --max-time 10 "$URL/healthz") || {
  echo "tunnel-health: request to $URL/healthz failed" >&2
  exit 1
}
if [ "$body" != '{"status":"ok"}' ]; then
  echo "tunnel-health: unexpected body from $URL/healthz: $body" >&2
  exit 2
fi
echo "tunnel-health: OK ($URL/healthz -> $body)"
