#!/usr/bin/env bash
# IT4 — PodMonitor carries the release label, pod selector, namespaceSelector, and the
#       :8889/metrics endpoint. IT5 — nothing under infra/k8s/observability/** references
#       or patches the shared otel-collector Service (PodMonitor selects pods, not the Service).
set -euo pipefail
PM="infra/k8s/observability/podmonitor-reckonna-collector.yaml"
DIR="infra/k8s/observability"
fail() { echo "podmonitor: $1" >&2; exit 1; }

[ -f "$PM" ] || fail "$PM not found"

grep -q 'release: kube-prometheus-stack' "$PM" || fail "missing release: kube-prometheus-stack label (Prometheus CR podMonitorSelector)"
grep -q 'app: otel-collector' "$PM" || fail "missing selector.matchLabels: {app: otel-collector}"
grep -q 'targetPort: 8889' "$PM" || fail "missing podMetricsEndpoints targetPort: 8889"
grep -q 'path: /metrics' "$PM" || fail "missing podMetricsEndpoints path: /metrics"
grep -q 'namespaceSelector' "$PM" || fail "missing namespaceSelector"
grep -A2 'namespaceSelector' "$PM" | grep -q 'observability' || fail "namespaceSelector does not target observability"

# IT5 — zero mutation of the shared otel-collector Service: no file in this dir may
# declare/patch `kind: Service` named otel-collector.
if grep -rl 'kind: Service' "$DIR" 2>/dev/null | xargs -r grep -l 'name: otel-collector' 2>/dev/null | grep -q .; then
  fail "a Service named otel-collector is declared/patched under $DIR — shared infra must not be mutated (IT5)"
fi

echo "podmonitor: OK"
