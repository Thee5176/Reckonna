#!/usr/bin/env bash
# scripts/pg-probe.sh — application-side connectivity probe for the tailnet PG endpoint.
#
# Intended for application teams who already have credentials in their own
# runtime (env, init-container, secret manager) and want to verify the four
# distinct ways a tailnet PG connection can fail.
#
# Uses libpq-standard environment variables only — no Vault CLI required:
#
#   PGHOST      tailnet hostname (e.g. pg-reckonna.<tailnet>.ts.net) or IP
#   PGPORT      defaults to 5432
#   PGUSER      database role
#   PGPASSWORD  password (read at use-time; never logged)
#   PGDATABASE  database name
#   PGSSLMODE   prefer | require | disable (default: prefer)
#
# Stages run top-down; the first failure exits with a stage-specific code so
# operators can route the issue without re-reading psql output.
#
# Exit codes:
#   0  all stages OK
#   1  missing required env var or bad CLI argument
#   2  command not on PATH (psql / getent / timeout / bash /dev/tcp)
#   3  DNS resolution failed     — tailnet device not visible; check `tailscale status`
#   4  TCP connect failed         — ACL deny, NetworkPolicy block, or pod not Ready
#   5  TLS handshake failed       — sslmode=require but server lacks cert
#   6  authentication failed      — PGUSER / PGPASSWORD wrong; rotate via Vault
#   7  database does not exist    — PGDATABASE typo or wrong cluster
#   8  query failed (other)       — connection succeeded but `SELECT 1` errored
#
set -euo pipefail

PORT="${PGPORT:-5432}"
SSLMODE="${PGSSLMODE:-prefer}"

usage() {
  sed -n '2,28p' "$0"
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    -h|--help) usage; exit 0 ;;
    *) echo "pg-probe: unknown arg '$1'" >&2; usage >&2; exit 1 ;;
  esac
  shift
done

for var in PGHOST PGUSER PGPASSWORD PGDATABASE; do
  if [[ -z "${!var:-}" ]]; then
    echo "pg-probe: missing required env var \$${var}" >&2
    exit 1
  fi
done

command -v psql    >/dev/null 2>&1 || { echo "pg-probe: psql not on PATH" >&2; exit 2; }
command -v getent  >/dev/null 2>&1 || { echo "pg-probe: getent not on PATH" >&2; exit 2; }
command -v timeout >/dev/null 2>&1 || { echo "pg-probe: timeout not on PATH (need coreutils)" >&2; exit 2; }

# Stage 1 — DNS. Skip when PGHOST is already an IP literal.
RESOLVED_IP="$PGHOST"
if [[ "$PGHOST" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "pg-probe: stage 1/3 DNS skipped (PGHOST is IP literal)"
else
  if ! resolved=$(getent hosts "$PGHOST" 2>/dev/null); then
    echo "pg-probe: stage 1/3 DNS FAIL — '$PGHOST' did not resolve" >&2
    echo "         hint: run \`tailscale status\` — are you joined to the tailnet?" >&2
    exit 3
  fi
  RESOLVED_IP="$(echo "$resolved" | awk '{print $1}' | head -1)"
  echo "pg-probe: stage 1/3 DNS OK ($PGHOST -> $RESOLVED_IP)"
fi

# Stage 2 — TCP. Connect to the IP we just resolved so the probe tests the
# same path psql will take, not whatever bash's libc resolver picks.
if ! timeout 5 bash -c "exec 3<>/dev/tcp/$RESOLVED_IP/$PORT" 2>/dev/null; then
  echo "pg-probe: stage 2/3 TCP FAIL — could not connect to $RESOLVED_IP:$PORT" >&2
  echo "         hint: tailnet ACL may not allow your tag; check \`tailscale netcheck\`" >&2
  exit 4
fi
echo "pg-probe: stage 2/3 TCP OK"

# Stage 3 — auth + query. Capture stderr so we can classify by error string.
export PGSSLMODE="$SSLMODE"
err=$(psql -h "$PGHOST" -p "$PORT" -U "$PGUSER" -d "$PGDATABASE" \
            -v ON_ERROR_STOP=1 -tA -c 'SELECT 1' 2>&1) && rc=0 || rc=$?

if [[ $rc -eq 0 ]]; then
  if [[ "$err" == *"1"* ]]; then
    echo "pg-probe: stage 3/3 QUERY OK (SELECT 1 returned 1)"
    exit 0
  fi
  echo "pg-probe: stage 3/3 QUERY FAIL — unexpected output: $err" >&2
  exit 8
fi

# Classify failure by error string. Order matters (TLS check before auth, since
# a TLS error often masquerades as an auth error in the user-visible message).
case "$err" in
  *"SSL "*|*"TLS "*|*"server does not support SSL"*|*"certificate verify failed"*)
    echo "pg-probe: stage 3/3 TLS FAIL — $err" >&2
    echo "         hint: try PGSSLMODE=prefer to negotiate, or PGSSLMODE=disable for plaintext over tailnet" >&2
    exit 5 ;;
  *"password authentication failed"*|*"role \""*"\" does not exist"*|*"no password supplied"*)
    echo "pg-probe: stage 3/3 AUTH FAIL — $err" >&2
    echo "         hint: rotate creds in Vault and restart the pod; see docs section 5" >&2
    exit 6 ;;
  *"database \""*"\" does not exist"*)
    echo "pg-probe: stage 3/3 DB FAIL — $err" >&2
    echo "         hint: PGDATABASE='$PGDATABASE' — typo or wrong cluster?" >&2
    exit 7 ;;
  *)
    echo "pg-probe: stage 3/3 QUERY FAIL — $err" >&2
    exit 8 ;;
esac
