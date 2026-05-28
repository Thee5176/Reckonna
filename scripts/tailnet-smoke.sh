#!/usr/bin/env bash
# scripts/tailnet-smoke.sh — non-destructive 'SELECT 1' against the tailnet PG endpoint.
#
# Reads creds from Vault at secret/app/database. Never accepts a password on
# the command line. Run from a tailnet-joined host.
#
# Exit codes:
#   0  PG returned 1
#   2  endpoint unresolvable (pg-endpoint.sh exit code)
#   3  vault read failed
#   4  psql failed (connect / auth / query)
#
set -euo pipefail

HERE="$(cd "$(dirname "$0")" && pwd)"
HOST="$("$HERE/pg-endpoint.sh" --hostname)" || exit 2

command -v vault >/dev/null 2>&1 || { echo "tailnet-smoke: vault CLI missing" >&2; exit 3; }
command -v psql  >/dev/null 2>&1 || { echo "tailnet-smoke: psql missing" >&2; exit 3; }

PGUSER="$(vault kv get -mount=secret -field=username app/database)" || exit 3
PGDB="$(vault kv get -mount=secret -field=dbname app/database)"     || exit 3
PGPASSWORD="$(vault kv get -mount=secret -field=password app/database)" || exit 3
export PGPASSWORD

out=$(psql -h "$HOST" -U "$PGUSER" -d "$PGDB" -tA -c 'SELECT 1' 2>&1) || {
  echo "tailnet-smoke: psql failed: $out" >&2
  exit 4
}
[[ "$out" == "1" ]] || { echo "tailnet-smoke: unexpected output '$out'" >&2; exit 4; }
echo "tailnet-smoke: OK ($HOST returned 1)"
