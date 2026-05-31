#!/usr/bin/env bash
# IT3 (redis) — NetworkPolicy gates ingress to tailscale ns only; egress to vault + dns only.
set -euo pipefail
NP="infra/k8s/redis/networkpolicy.yaml"
fail() { echo "redis-networkpolicy: $1" >&2; exit 1; }
grep -q 'kubernetes.io/metadata.name: tailscale' "$NP" || fail "ingress source not scoped to tailscale ns"
grep -q 'kubernetes.io/metadata.name: vault' "$NP" || fail "egress to vault missing"
grep -q 'kubernetes.io/metadata.name: kube-system' "$NP" || fail "egress to kube-dns missing"
grep -qE '^\s*port:\s*6379\b' "$NP" || fail "redis port 6379 not declared on ingress"
# No wildcard ingress / no '0.0.0.0/0'.
if grep -E 'ipBlock|0\.0\.0\.0/0' "$NP" | grep -q .; then
  fail "ipBlock/0.0.0.0 found — must stay namespace-scoped"
fi
echo "redis-networkpolicy: OK"
