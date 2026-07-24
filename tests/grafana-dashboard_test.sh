#!/usr/bin/env bash
# IT7/IT8 — reckonna-red.json is valid, panels reference the confirmed reckonna_* /
# otelgin metric names, no histogram_quantile/_bucket (D9), datasource targets the
# self-hosted Prometheus (plan 06 D-GRAFANA), and (if a TF provisioning path is added)
# it sources its Grafana token from Vault, never a literal.
set -euo pipefail
DASH="infra/k8s/observability/dashboards/reckonna-red.json"
fail() { echo "grafana-dashboard: $1" >&2; exit 1; }

[ -f "$DASH" ] || fail "$DASH missing"
command -v jq >/dev/null 2>&1 || fail "jq not installed"

jq -e . "$DASH" >/dev/null || fail "$DASH is not valid JSON"

# IT7 — panels reference the confirmed metric names.
grep -q 'reckonna_http_server_requests_total' "$DASH" || fail "missing reckonna_http_server_requests_total (request rate / error ratio)"
grep -q 'http_server_request_duration_milliseconds_sum' "$DASH" || fail "missing http_server_request_duration_milliseconds_sum (avg latency)"
grep -q 'http_server_request_duration_milliseconds_count' "$DASH" || fail "missing http_server_request_duration_milliseconds_count (avg latency)"
grep -q 'reckonna_ledger_rejected_total' "$DASH" || fail "missing reckonna_ledger_rejected_total (ledger domain signal, AT6)"

# IT7 — D9: no histogram_quantile / _bucket (the live collector drops buckets).
if grep -q 'histogram_quantile' "$DASH"; then
  fail "histogram_quantile found — D9 forbids it (live collector drops .*_bucket)"
fi
if grep -qE '_bucket\b' "$DASH"; then
  fail "_bucket metric referenced — D9 forbids it (live collector drops .*_bucket)"
fi

# D10 precondition (b) — service_name is a resource attr on target_info, not a
# metric label; queries must join via target_info rather than assume a bare
# service_name label on reckonna_*/http_server_* series.
grep -q 'target_info' "$DASH" || fail "no target_info join found — service_name is a resource attr (D10 precondition b), queries must join via target_info"

# D-GRAFANA — datasource must target the self-hosted Prometheus, not Grafana Cloud.
grep -qi 'grafanacloud' "$DASH" && fail "dashboard references a grafanacloud-style datasource UID — must target the self-hosted Prometheus"
grep -q '"type": "prometheus"' "$DASH" || fail "no prometheus datasource type found"

# No literal Grafana/Vault token embedded in the dashboard JSON itself.
if grep -qiE '(glsa_|eyJrIjoi)' "$DASH"; then
  fail "literal Grafana token pattern found in $DASH"
fi

# IT8 — if a TF provisioning path was added, it must source the token from Vault.
if compgen -G "infra/terraform/grafana-*.tf" >/dev/null 2>&1; then
  for f in infra/terraform/grafana-*.tf; do
    grep -q 'vault_kv_secret_v2' "$f" || fail "$f does not source its Grafana token via vault_kv_secret_v2"
    grep -qE '"(glsa_|eyJrIjoi)' "$f" && fail "$f embeds a literal Grafana token"
  done
fi

echo "grafana-dashboard: OK"
