#!/usr/bin/env bash
# tests/pg-endpoint_test.sh — exercise scripts/pg-endpoint.sh with a fake
# tailscale shim and a fake kubectl shim. No real network calls.
set -euo pipefail

HERE="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(cd "$HERE/.." && pwd)"
SCRIPT="$ROOT/scripts/pg-endpoint.sh"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

# Fake tailscale that returns a known peer list.
cat >"$TMP/tailscale" <<'EOF'
#!/usr/bin/env bash
case "$1 $2" in
  "status --json")
    cat <<'JSON'
{
  "Peer": {
    "abc": {
      "HostName": "pg-reckonna",
      "DNSName":  "pg-reckonna.tail-test.ts.net.",
      "TailscaleIPs": ["100.64.10.5"]
    }
  }
}
JSON
    ;;
  *) echo "fake tailscale: unhandled args: $*" >&2; exit 2 ;;
esac
EOF
chmod +x "$TMP/tailscale"

# Fake kubectl that always fails to resolve — so the negative cases below cannot
# fall through to a live homelab kubeconfig (resolve_from_kubectl) and spuriously
# succeed. Mirrors how pg-probe_test isolates psql/getent via $TMP shims.
cat >"$TMP/kubectl" <<'EOF'
#!/usr/bin/env bash
exit 1
EOF
chmod +x "$TMP/kubectl"

export PATH="$TMP:$PATH"

# 1. full mode prints hostname + ip
out=$("$SCRIPT")
echo "$out" | grep -q '^hostname=pg-reckonna.tail-test.ts.net$' \
  || { echo "FAIL: full hostname line"; echo "$out"; exit 1; }
echo "$out" | grep -q '^ip=100.64.10.5$' \
  || { echo "FAIL: full ip line"; echo "$out"; exit 1; }

# 2. hostname mode
[[ "$("$SCRIPT" --hostname)" == "pg-reckonna.tail-test.ts.net" ]] \
  || { echo "FAIL: --hostname"; exit 1; }

# 3. ip mode
[[ "$("$SCRIPT" --ip)" == "100.64.10.5" ]] \
  || { echo "FAIL: --ip"; exit 1; }

# 4. url mode
[[ "$("$SCRIPT" --url)" == "postgres://pg-reckonna.tail-test.ts.net:5432/" ]] \
  || { echo "FAIL: --url"; exit 1; }

# 5. unknown peer (tailscale up, device absent) → kubectl fallback neutralized → exit 3.
RECKONNA_PG_DEVICE="ghost-peer" "$SCRIPT" >/dev/null 2>&1 && \
  { echo "FAIL: missing device should exit non-zero"; exit 1; } || true

# 6. neither tailscale nor kubectl available → exit 2.
# Build an isolated PATH with only minimal bash builtins location.
EMPTY="$(mktemp -d)"
trap 'rm -rf "$TMP" "$EMPTY"' EXIT
for b in bash sh sed awk grep head jq; do
  src="$(command -v "$b" 2>/dev/null || true)"
  [[ -n "$src" ]] && ln -s "$src" "$EMPTY/$b"
done
PATH="$EMPTY" "$SCRIPT" >/dev/null 2>&1 && \
  { echo "FAIL: empty PATH should exit non-zero"; exit 1; } || true

echo "pg-endpoint_test: OK"
