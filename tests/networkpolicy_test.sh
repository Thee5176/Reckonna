#!/usr/bin/env bash
# IT3 — NetworkPolicy gates ingress to tailscale ns only; egress to vault + dns only.
set -euo pipefail
NP="infra/k8s/postgres/networkpolicy.yaml"
fail() { echo "networkpolicy: $1" >&2; exit 1; }
grep -q 'kubernetes.io/metadata.name: tailscale' "$NP" || fail "ingress source not scoped to tailscale ns"
grep -q 'kubernetes.io/metadata.name: vault' "$NP" || fail "egress to vault missing"
grep -q 'kubernetes.io/metadata.name: kube-system' "$NP" || fail "egress to kube-dns missing"
# No wildcard ingress / no '0.0.0.0/0'.
if grep -E 'ipBlock|0\.0\.0\.0/0' "$NP" | grep -q .; then
  fail "ipBlock/0.0.0.0 found — must stay namespace-scoped"
fi
echo "networkpolicy: OK"
