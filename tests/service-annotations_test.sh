#!/usr/bin/env bash
# IT5 — Postgres Service exposes the Tailscale + MagicDNS annotations.
set -euo pipefail
SVC="infra/k8s/postgres/service.yaml"
fail() { echo "service-annotations: $1" >&2; exit 1; }
grep -q 'tailscale.com/expose: "true"' "$SVC" || fail "missing tailscale.com/expose=true"
grep -q 'tailscale.com/hostname: "pg-reckonna"' "$SVC" || fail "missing tailscale.com/hostname=pg-reckonna"
echo "service-annotations: OK"
