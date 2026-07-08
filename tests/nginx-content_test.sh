#!/usr/bin/env bash
# tests/nginx-content_test.sh — IT6: reckonna-app ConfigMap declares healthz ({"status":"ok"})
# and reckonna_hello (contains "hello"); the Deployment mounts content into
# /usr/share/nginx/html. Static grep test — no cluster needed.
set -euo pipefail

HERE="$(cd "$(dirname "$0")" && pwd)"
BASE="$HERE/../infra/k8s/reckonna-app"
CM="$BASE/configmap.yaml"
DEP="$BASE/deployment.yaml"

fail() { echo "nginx-content: FAIL: $1" >&2; exit 1; }

grep -q 'healthz:' "$CM"                         || fail "healthz key missing in configmap"
grep -Eq 'healthz:.*\{"status":"ok"\}' "$CM"     || fail "healthz content is not {\"status\":\"ok\"}"
grep -q 'reckonna_hello:' "$CM"                  || fail "reckonna_hello key missing in configmap"
grep -Eq 'reckonna_hello:.*hello' "$CM"          || fail "reckonna_hello content lacks 'hello'"
grep -q 'mountPath: /usr/share/nginx/html' "$DEP" || fail "content not mounted at /usr/share/nginx/html"

echo "nginx-content: OK (IT6 — healthz + reckonna_hello present, mounted to html root)"
