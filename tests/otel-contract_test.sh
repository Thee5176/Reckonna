#!/usr/bin/env bash
# tests/otel-contract_test.sh — plan 06 IT6: docs/otel-telemetry-setup.md pins
# the OTLP endpoint, service.name resource attrs, the D10 metric-export
# contract, and the egress rule the backend-Deploy plan must ship. Static
# grep only — no live collector, no kubectl.
set -euo pipefail

HERE="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(cd "$HERE/.." && pwd)"
DOC="$ROOT/docs/otel-telemetry-setup.md"

[[ -f "$DOC" ]] || { echo "FAIL: $DOC missing"; exit 1; }

# 1. OTLP endpoint (base URL, no /v1/... suffix — exporters append it).
grep -q 'http://otel-collector.observability.svc.cluster.local:4318' "$DOC" \
  || { echo "FAIL: OTLP endpoint not pinned"; exit 1; }

# 2. Resource attrs — service.name per side.
grep -q 'service.name.*reckonna-command' "$DOC" \
  || { echo "FAIL: reckonna-command service.name not documented"; exit 1; }
grep -q 'service.name.*reckonna-query' "$DOC" \
  || { echo "FAIL: reckonna-query service.name not documented"; exit 1; }

# 3. D10 metric-export contract — otlpmetrichttp + meter provider, not
# traces-only.
grep -q 'otlpmetrichttp' "$DOC" \
  || { echo "FAIL: otlpmetrichttp metric-export contract not documented"; exit 1; }
grep -qi 'MeterProvider' "$DOC" \
  || { echo "FAIL: MeterProvider not documented"; exit 1; }

# 4. Egress rule for the backend-Deploy plan — both ports.
grep -q '4317' "$DOC" \
  || { echo "FAIL: egress port 4317 not documented"; exit 1; }
grep -q '4318' "$DOC" \
  || { echo "FAIL: egress port 4318 not documented"; exit 1; }
grep -qi 'NetworkPolicy' "$DOC" \
  || { echo "FAIL: egress NetworkPolicy not documented"; exit 1; }

echo "otel-contract_test: OK"
