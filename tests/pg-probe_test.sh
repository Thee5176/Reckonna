#!/usr/bin/env bash
# tests/pg-probe_test.sh — exercise scripts/pg-probe.sh with fake psql + getent
# and a tiny throwaway TCP listener for the connect stage.
# No real database, no real network egress.
set -euo pipefail

HERE="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(cd "$HERE/.." && pwd)"
SCRIPT="$ROOT/scripts/pg-probe.sh"
TMP="$(mktemp -d)"
LISTENER_PORT=55432

cleanup() {
  [[ -n "${LISTENER_PID:-}" ]] && kill "$LISTENER_PID" 2>/dev/null || true
  rm -rf "$TMP"
}
trap cleanup EXIT

# Fake getent — succeeds unless DNS_FAIL=1 in the calling env.
cat >"$TMP/getent" <<'EOF'
#!/usr/bin/env bash
if [[ "${DNS_FAIL:-0}" == "1" ]]; then exit 2; fi
[[ "$1" == "hosts" ]] && echo "127.0.0.1 $2"
EOF
chmod +x "$TMP/getent"

# Fake psql — driven by PSQL_MODE.
cat >"$TMP/psql" <<'EOF'
#!/usr/bin/env bash
case "${PSQL_MODE:-ok}" in
  ok)    echo "1"; exit 0 ;;
  tls)   echo "psql: error: server does not support SSL, but SSL was required" >&2; exit 2 ;;
  auth)  echo 'psql: error: FATAL:  password authentication failed for user "app"' >&2; exit 2 ;;
  db)    echo 'psql: error: FATAL:  database "accounting" does not exist' >&2; exit 2 ;;
  other) echo "psql: error: server closed the connection unexpectedly" >&2; exit 2 ;;
esac
EOF
chmod +x "$TMP/psql"

export PATH="$TMP:$PATH"

# Long-running accept loop so every test stage finds an open port.
python3 - "$LISTENER_PORT" <<'PY' &
import socket, sys, threading
port = int(sys.argv[1])
s = socket.socket(); s.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
s.bind(("127.0.0.1", port)); s.listen(32)
def loop():
    while True:
        try: c,_ = s.accept(); c.close()
        except Exception: return
threading.Thread(target=loop, daemon=True).start()
import time
while True: time.sleep(60)
PY
LISTENER_PID=$!
# Wait for bind.
for _ in 1 2 3 4 5; do
  bash -c "exec 3<>/dev/tcp/127.0.0.1/$LISTENER_PORT" 2>/dev/null && break
  sleep 0.2
done

# Common env that every case overrides selectively.
base_env() {
  env -i PATH="$PATH" \
         PGHOST=pg-reckonna.tail-test.ts.net \
         PGPORT=$LISTENER_PORT \
         PGUSER=app PGPASSWORD=pw PGDATABASE=accounting \
         "$@"
}

run() { base_env "$@" "$SCRIPT" >/dev/null 2>&1; echo $?; }

# 1. Happy.
rc=$(run PSQL_MODE=ok); [[ "$rc" == "0" ]] || { echo "FAIL: happy rc=$rc"; exit 1; }

# 2. Missing env var.
rc=$(env -i PATH="$PATH" PGHOST=x PGPASSWORD=p PGDATABASE=d "$SCRIPT" >/dev/null 2>&1; echo $?)
[[ "$rc" == "1" ]] || { echo "FAIL: missing-env rc=$rc"; exit 1; }

# 3. DNS failure.
rc=$(run DNS_FAIL=1 PSQL_MODE=ok); [[ "$rc" == "3" ]] || { echo "FAIL: DNS rc=$rc"; exit 1; }

# 4. TCP failure — point at a guaranteed-closed port. Use IP literal to skip DNS.
rc=$(base_env PGHOST=127.0.0.1 PGPORT=1 PSQL_MODE=ok "$SCRIPT" >/dev/null 2>&1; echo $?)
[[ "$rc" == "4" ]] || { echo "FAIL: TCP rc=$rc"; exit 1; }

# 5–8. Stage-3 classifications.
rc=$(run PSQL_MODE=tls);   [[ "$rc" == "5" ]] || { echo "FAIL: TLS rc=$rc"; exit 1; }
rc=$(run PSQL_MODE=auth);  [[ "$rc" == "6" ]] || { echo "FAIL: AUTH rc=$rc"; exit 1; }
rc=$(run PSQL_MODE=db);    [[ "$rc" == "7" ]] || { echo "FAIL: DB rc=$rc"; exit 1; }
rc=$(run PSQL_MODE=other); [[ "$rc" == "8" ]] || { echo "FAIL: OTHER rc=$rc"; exit 1; }

echo "pg-probe_test: OK"
