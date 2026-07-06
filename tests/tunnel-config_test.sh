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

# ORDER matters: cloudflared evaluates ingress_rule blocks top-down, so a catch-all
# placed before the hostname rule would 404 ALL traffic while every grep above still
# passes. Assert the catch-all line comes strictly after the hostname rule.
HOST_LINE=$(grep -nE 'hostname[[:space:]]*=[[:space:]]*"reckonna.thee5176.com"' "$TF" | head -1 | cut -d: -f1)
CATCH_LINE=$(grep -nE 'service[[:space:]]*=[[:space:]]*"http_status:404"' "$TF" | head -1 | cut -d: -f1)
[ -n "$HOST_LINE" ] && [ -n "$CATCH_LINE" ] && [ "$CATCH_LINE" -gt "$HOST_LINE" ] \
  || fail "http_status:404 catch-all must come AFTER the reckonna hostname rule (got hostname@$HOST_LINE, catch-all@$CATCH_LINE)"

echo "tunnel-config: OK (IT4 - reckonna.thee5176.com -> app svc + 404 catch-all, order verified)"
