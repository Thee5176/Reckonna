#!/usr/bin/env bash
# IT7 + IT8 (daemonset half) — OTel Collector DaemonSet uses hostNetwork +
# hostPorts 4317/4318 and Vault Agent Injector for OTEL_EXPORTER_OTLP_*.
set -euo pipefail
DS="infra/k8s/otel/daemonset.yaml"
fail() { echo "otel-daemonset: $1" >&2; exit 1; }

# IT7 — host networking.
grep -qE '^\s*hostNetwork:\s*true\s*$' "$DS" || fail "hostNetwork: true missing"
grep -qE '^\s*dnsPolicy:\s*ClusterFirstWithHostNet\s*$' "$DS" || fail "dnsPolicy: ClusterFirstWithHostNet missing"
grep -qE '^\s*hostPort:\s*4317\b' "$DS" || fail "hostPort 4317 missing"
grep -qE '^\s*hostPort:\s*4318\b' "$DS" || fail "hostPort 4318 missing"
grep -qE '^\s*containerPort:\s*4317\b' "$DS" || fail "containerPort 4317 missing"
grep -qE '^\s*containerPort:\s*4318\b' "$DS" || fail "containerPort 4318 missing"

# IT8 daemonset half — vault-injector annotations.
grep -q 'vault.hashicorp.com/agent-inject: "true"' "$DS" || fail "agent-inject annotation missing"
grep -q 'vault.hashicorp.com/role: "reckonna-otel-collector"' "$DS" || fail "role annotation missing"
grep -q 'secret/data/app/otel/exporter' "$DS" || fail "vault path missing"
grep -q 'OTEL_EXPORTER_OTLP_ENDPOINT' "$DS" || fail "endpoint env var missing in vault template"
grep -q 'OTEL_EXPORTER_OTLP_HEADERS'  "$DS" || fail "headers env var missing in vault template"
grep -q '/vault/secrets/otel.env'      "$DS" || fail "vault.env mount path not sourced by entrypoint"
grep -qE 'image:\s*otel/opentelemetry-collector:0\.108\.0\s*$' "$DS" || fail "collector image pin must be otel/opentelemetry-collector:0.108.0 (contrib-free)"

# Defence in depth: forbid plaintext endpoint or Bearer literal anywhere.
if grep -nE 'OTEL_EXPORTER_OTLP_(ENDPOINT|HEADERS)\s*=\s*"[^{$][^"]*"' "$DS" \
   | grep -vE '\.Data\.data\.(endpoint|api_key)' \
   | grep -q .; then
  fail "plaintext OTEL_EXPORTER_OTLP_* literal detected"
fi
if grep -qE 'https?://[a-zA-Z0-9.-]+' "$DS"; then
  fail "literal http(s):// URL detected — endpoint must come from Vault"
fi
echo "otel-daemonset: OK"
