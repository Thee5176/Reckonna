#!/usr/bin/env bash
# tests/tunnel-config_test.sh — IT4: cloudflare.tf declares a remote-managed tunnel config whose
# ingress routes reckonna.thee5176.com -> the in-cluster nginx harness, with a 404 catch-all.
# Static grep test — no cloud, no terraform apply.
set -euo pipefail

HERE="$(cd "$(dirname "$0")" && pwd)"
TF="$HERE/../infra/terraform/cloudflare.tf"

fail() { echo "tunnel-config: FAIL: $1" >&2; exit 1; }

grep -q 'cloudflare_zero_trust_tunnel_cloudflared_config' "$TF" || fail "no tunnel config resource"
grep -Eq 'hostname[[:space:]]*=[[:space:]]*"reckonna.thee5176.com"' "$TF" || fail "ingress hostname is not reckonna.thee5176.com"
grep -Eq 'service[[:space:]]*=[[:space:]]*"http://reckonna-app.reckonna-app.svc.cluster.local:80"' "$TF" || fail "app service target missing"
grep -Eq 'service[[:space:]]*=[[:space:]]*"http_status:404"' "$TF" || fail "404 catch-all missing"

echo "tunnel-config: OK (IT4 - reckonna.thee5176.com -> app svc + 404 catch-all)"
