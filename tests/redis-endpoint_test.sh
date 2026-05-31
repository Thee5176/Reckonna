#!/usr/bin/env bash
# tests/redis-endpoint_test.sh — exercise scripts/redis-endpoint.sh with a fake
# tailscale shim. No real network calls.
set -euo pipefail

HERE="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(cd "$HERE/.." && pwd)"
SCRIPT="$ROOT/scripts/redis-endpoint.sh"
TMP="$(mktemp -d)"
EMPTY="$(mktemp -d)"
trap 'rm -rf "$TMP" "$EMPTY"' EXIT

cat >"$TMP/tailscale" <<'EOF'
#!/usr/bin/env bash
case "$1 $2" in
  "status --json")
    cat <<'JSON'
{
  "Peer": {
    "abc": {
      "HostName": "redis-reckonna",
      "DNSName":  "redis-reckonna.tail-test.ts.net.",
      "TailscaleIPs": ["100.64.10.7"]
    }
  }
}
JSON
    ;;
  *) echo "fake tailscale: unhandled args: $*" >&2; exit 2 ;;
esac
EOF
chmod +x "$TMP/tailscale"

export PATH="$TMP:$PATH"

# 1. full mode prints hostname + ip
out=$("$SCRIPT")
echo "$out" | grep -q '^hostname=redis-reckonna.tail-test.ts.net$' \
  || { echo "FAIL: full hostname line"; echo "$out"; exit 1; }
echo "$out" | grep -q '^ip=100.64.10.7$' \
  || { echo "FAIL: full ip line"; echo "$out"; exit 1; }

# 2. hostname mode
[[ "$("$SCRIPT" --hostname)" == "redis-reckonna.tail-test.ts.net" ]] \
  || { echo "FAIL: --hostname"; exit 1; }

# 3. ip mode
[[ "$("$SCRIPT" --ip)" == "100.64.10.7" ]] \
  || { echo "FAIL: --ip"; exit 1; }

# 4. url mode
[[ "$("$SCRIPT" --url)" == "redis://redis-reckonna.tail-test.ts.net:6379/" ]] \
  || { echo "FAIL: --url"; exit 1; }

# 5. unknown peer → exit 3.
RECKONNA_REDIS_DEVICE="ghost-peer" "$SCRIPT" >/dev/null 2>&1 && \
  { echo "FAIL: missing device should exit non-zero"; exit 1; } || true

# 6. neither tailscale nor kubectl available → exit 2.
for b in bash sh sed awk grep head jq; do
  src="$(command -v "$b" 2>/dev/null || true)"
  [[ -n "$src" ]] && ln -s "$src" "$EMPTY/$b"
done
PATH="$EMPTY" "$SCRIPT" >/dev/null 2>&1 && \
  { echo "FAIL: empty PATH should exit non-zero"; exit 1; } || true

echo "redis-endpoint_test: OK"
