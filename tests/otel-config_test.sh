#!/usr/bin/env bash
# IT8 (config half) — OTel Collector ConfigMap uses ${env:...} substitution for
# the exporter endpoint + headers, never literal values. Pipeline must include
# otlp receiver + batch + otlp exporter at minimum.
set -euo pipefail
CM="infra/k8s/otel/configmap.yaml"
fail() { echo "otel-config: $1" >&2; exit 1; }
grep -q '${env:OTEL_EXPORTER_OTLP_ENDPOINT}' "$CM" || fail "endpoint must use \${env:OTEL_EXPORTER_OTLP_ENDPOINT}"
grep -q '${env:OTEL_EXPORTER_OTLP_HEADERS}' "$CM"  || fail "headers must use \${env:OTEL_EXPORTER_OTLP_HEADERS}"
grep -qE 'grpc:\s*$|endpoint:\s*0\.0\.0\.0:4317' "$CM" || fail "otlp gRPC receiver on 4317 missing"
grep -qE 'http:\s*$|endpoint:\s*0\.0\.0\.0:4318' "$CM" || fail "otlp HTTP receiver on 4318 missing"
grep -q 'batch:' "$CM" || fail "batch processor missing"
grep -q 'memory_limiter:' "$CM" || fail "memory_limiter processor missing"
# Forbid any literal exporter endpoint or bearer token.
if grep -nE 'https?://[a-zA-Z0-9._-]+' "$CM" \
   | grep -vE '^\s*#|0\.0\.0\.0' \
   | grep -q .; then
  fail "literal http(s):// URL detected outside comments — must be vault-templated"
fi
if grep -qiE '(authorization:\s*Bearer\s+[A-Za-z0-9])' "$CM"; then
  fail "literal Bearer token detected"
fi
echo "otel-config: OK"
