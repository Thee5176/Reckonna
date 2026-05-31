#!/usr/bin/env bash
# IT9 — OTel Collector NetworkPolicy allows cluster-wide ingress on 4317/4318,
# egress to kube-dns + vault, and a wildcard egress to TCP 443 with RFC1918/CGNAT
# explicitly excluded.
set -euo pipefail
NP="infra/k8s/otel/networkpolicy.yaml"
fail() { echo "otel-networkpolicy: $1" >&2; exit 1; }

# Ingress: cluster-wide namespaceSelector {} on 4317 + 4318.
grep -q 'namespaceSelector: {}' "$NP" || fail "ingress must allow namespaceSelector {} for cluster-wide ingest"
grep -qE '^\s*port:\s*4317\b' "$NP" || fail "ingress on 4317 missing"
grep -qE '^\s*port:\s*4318\b' "$NP" || fail "ingress on 4318 missing"

# Egress: kube-dns + vault namespaces, and ipBlock 0.0.0.0/0 with RFC1918 except.
grep -q 'kubernetes.io/metadata.name: kube-system' "$NP" || fail "egress to kube-dns missing"
grep -q 'kubernetes.io/metadata.name: vault'       "$NP" || fail "egress to vault missing"
grep -q 'cidr: 0.0.0.0/0' "$NP" || fail "external egress ipBlock 0.0.0.0/0 missing"
for cidr in '10.0.0.0/8' '172.16.0.0/12' '192.168.0.0/16' '100.64.0.0/10'; do
  grep -q "$cidr" "$NP" || fail "RFC1918/CGNAT exception $cidr missing"
done
grep -qE '^\s*port:\s*443\b' "$NP" || fail "external egress port 443 missing"
grep -qE '^\s*port:\s*8200\b' "$NP" || fail "vault egress port 8200 missing"
echo "otel-networkpolicy: OK"
