#!/usr/bin/env bash
# scripts/otel-smoke.sh — emit a synthetic OTLP span at the local collector and
# verify the receiver accepted it (HTTP 2xx). OTLP/HTTP is used instead of gRPC
# so the script needs only `curl`, not `grpcurl` or proto descriptors.
#
# Usage:
#   scripts/otel-smoke.sh                       # POST to $OTEL_TARGET (default localhost:4318)
#   OTEL_TARGET=node-a:4318 scripts/otel-smoke.sh
#   scripts/otel-smoke.sh node-b:4318           # positional override
#
# Exit codes:
#   0  receiver returned 2xx (span accepted)
#   2  curl missing
#   3  connect refused / DNS failure / timeout
#   4  receiver returned non-2xx
#
set -euo pipefail

TARGET="${1:-${OTEL_TARGET:-localhost:4318}}"

command -v curl >/dev/null 2>&1 || { echo "otel-smoke: curl missing" >&2; exit 2; }

# 16-byte traceId + 8-byte spanId as lowercase hex. Synthetic but well-formed
# per OTLP spec.
TRACE_ID="5b8aa5a2d2c872e8321cf37308d69df2"
SPAN_ID="051581bf3cb55c13"
# Static start/end so the script is deterministic and the same input always
# produces the same span; the collector deduplicates on (traceId, spanId).
START_NS="1700000000000000000"
END_NS="1700000000100000000"

BODY=$(cat <<JSON
{
  "resourceSpans": [{
    "resource": {
      "attributes": [{
        "key": "service.name",
        "value": { "stringValue": "otel-smoke" }
      }]
    },
    "scopeSpans": [{
      "scope": { "name": "scripts/otel-smoke.sh" },
      "spans": [{
        "traceId": "${TRACE_ID}",
        "spanId":  "${SPAN_ID}",
        "name":    "otel-smoke.ping",
        "kind":    1,
        "startTimeUnixNano": "${START_NS}",
        "endTimeUnixNano":   "${END_NS}"
      }]
    }]
  }]
}
JSON
)

URL="http://${TARGET}/v1/traces"
TMP="$(mktemp)"; trap 'rm -f "$TMP"' EXIT
HTTP_CODE=$(curl -sS -o "$TMP" -w '%{http_code}' \
  --connect-timeout 5 --max-time 10 \
  -H 'Content-Type: application/json' \
  -X POST -d "$BODY" "$URL" 2>"$TMP.err") || {
    echo "otel-smoke: curl failed for $URL: $(cat "$TMP.err" 2>/dev/null)" >&2
    rm -f "$TMP.err"
    exit 3
  }
rm -f "$TMP.err"

case "$HTTP_CODE" in
  2*)
    echo "otel-smoke: OK ($URL accepted span, HTTP $HTTP_CODE)"
    ;;
  *)
    echo "otel-smoke: receiver returned HTTP $HTTP_CODE for $URL" >&2
    [[ -s "$TMP" ]] && head -c 500 "$TMP" >&2
    exit 4
    ;;
esac
