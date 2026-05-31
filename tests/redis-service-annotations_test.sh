#!/usr/bin/env bash
# IT5 — Redis Service exposes the Tailscale + MagicDNS annotations.
set -euo pipefail
SVC="infra/k8s/redis/service.yaml"
fail() { echo "redis-service-annotations: $1" >&2; exit 1; }
grep -q 'tailscale.com/expose: "true"' "$SVC" || fail "missing tailscale.com/expose=true"
grep -q 'tailscale.com/hostname: "redis-reckonna"' "$SVC" || fail "missing tailscale.com/hostname=redis-reckonna"
echo "redis-service-annotations: OK"
