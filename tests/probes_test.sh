#!/usr/bin/env bash
# tests/probes_test.sh — IT7: reckonna-app Deployment readinessProbe AND livenessProbe both
# target path /healthz, port 80, scheme HTTP. Static grep test — no cluster needed.
set -euo pipefail

HERE="$(cd "$(dirname "$0")" && pwd)"
DEP="$HERE/../infra/k8s/reckonna-app/deployment.yaml"

fail() { echo "probes: FAIL: $1" >&2; exit 1; }

grep -q 'readinessProbe:' "$DEP" || fail "no readinessProbe"
grep -q 'livenessProbe:'  "$DEP" || fail "no livenessProbe"
# Each probe block must carry path /healthz, port 80, scheme HTTP.
for probe in readinessProbe livenessProbe; do
  block=$(awk "/$probe:/{f=1} f{print} /scheme: HTTP/{if(f)exit}" "$DEP")
  echo "$block" | grep -q 'path: /healthz' || fail "$probe path is not /healthz"
  echo "$block" | grep -q 'port: 80'       || fail "$probe port is not 80"
  echo "$block" | grep -q 'scheme: HTTP'   || fail "$probe scheme is not HTTP"
done

echo "probes: OK (IT7 — readiness + liveness both /healthz:80 HTTP)"
