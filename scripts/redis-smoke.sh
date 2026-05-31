#!/usr/bin/env bash
# scripts/redis-smoke.sh — non-destructive PING against the tailnet Redis endpoint.
#
# Reads requirepass from Vault at secret/app/redis. Never accepts a password on
# the command line. Run from a tailnet-joined host. Password lives only in a
# REDISCLI_AUTH env var scoped to the redis-cli child process so `ps` cannot
# see it; unset on exit.
#
# Exit codes:
#   0  redis returned PONG
#   2  endpoint unresolvable (redis-endpoint.sh exit code)
#   3  vault read failed
#   4  redis-cli failed (connect / auth / query)
#
set -euo pipefail

HERE="$(cd "$(dirname "$0")" && pwd)"
if ! HOST="$("$HERE/redis-endpoint.sh" --hostname)"; then
  exit 2
fi

command -v vault     >/dev/null 2>&1 || { echo "redis-smoke: vault CLI missing" >&2; exit 3; }
command -v redis-cli >/dev/null 2>&1 || { echo "redis-smoke: redis-cli missing" >&2; exit 3; }

REDISCLI_AUTH="$(vault kv get -mount=secret -field=password app/redis)" || exit 3
export REDISCLI_AUTH
# Defence: unset on any exit so the secret never persists in this shell.
trap 'unset REDISCLI_AUTH' EXIT

out=$(redis-cli -h "$HOST" -p 6379 PING 2>&1) || {
  echo "redis-smoke: redis-cli failed: $out" >&2
  exit 4
}
[[ "$out" == "PONG" ]] || { echo "redis-smoke: unexpected output '$out'" >&2; exit 4; }
echo "redis-smoke: OK ($HOST returned PONG)"
