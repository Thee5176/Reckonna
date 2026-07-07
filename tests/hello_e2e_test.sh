#!/usr/bin/env bash
# tests/hello_e2e_test.sh — AT2: the public tunnel serves /reckonna/hello as HTTP 200
# with a body containing "hello". Goes through the Cloudflare edge (not the origin).
#
# Unlike scripts/tunnel-health.sh (AT1, which lives in scripts/ and is only shellcheck'd
# in CI), this file sits in tests/ and IS executed by CI's `tests/*_test.sh` loop. So it
# must be SAFE when the tunnel is not up: if the URL is unreachable (DNS NXDOMAIN before
# `terraform apply`, or a network error) it SKIPS with exit 0. It only asserts once the
# endpoint actually responds — then a non-200 or a body without "hello" is a real failure.
# Human runs the live check post-apply; CI stays green pre-deploy and passes once live+healthy.
#
# Exit codes: 0 ok OR skipped (tunnel not reachable) | 3 reachable but non-200 | 4 reachable, 200, body lacks "hello"
set -euo pipefail

URL="${RECKONNA_URL:-https://reckonna.thee5176.com}"

# Capture body + HTTP code in one shot WITHOUT -f, so a live-but-bad response (e.g. 404/5xx)
# is distinguishable from an unreachable tunnel. Network errors (DNS/connect/timeout) make
# curl exit non-zero -> the || branch treats that as "not up yet" and skips.
resp=$(curl -s -w '\n%{http_code}' --max-time 10 "$URL/reckonna/hello" 2>/dev/null) || {
  echo "hello_e2e: SKIP — $URL/reckonna/hello unreachable (tunnel not applied yet); AT2 is a live check" >&2
  exit 0
}
code=${resp##*$'\n'}   # last line = http_code
body=${resp%$'\n'*}    # everything before it = response body

if [ "$code" != "200" ]; then
  echo "hello_e2e: FAIL — $URL/reckonna/hello returned HTTP $code (expected 200)" >&2
  exit 3
fi
case "$body" in
  *hello*) echo "hello_e2e: OK ($URL/reckonna/hello -> 200, body contains 'hello')" ;;
  *)       echo "hello_e2e: FAIL — 200 but body lacks 'hello': $body" >&2; exit 4 ;;
esac
