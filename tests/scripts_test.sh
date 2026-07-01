#!/usr/bin/env bash
# tests/scripts_test.sh — offline checks for the tunnel helper scripts. Stubs curl/dig on a temp
# PATH so nothing hits the network or a live tunnel. Covers happy + negative paths.
set -euo pipefail

HERE="$(cd "$(dirname "$0")" && pwd)"
SCRIPTS="$HERE/../scripts"
STUB="$(mktemp -d)"
trap 'rm -rf "$STUB"' EXIT

fail() { echo "scripts_test: FAIL: $1" >&2; exit 1; }

# tunnel-health: healthy body -> exit 0
cat > "$STUB/curl" <<'EOF'
#!/usr/bin/env bash
printf '{"status":"ok"}'
EOF
chmod +x "$STUB/curl"
PATH="$STUB:$PATH" bash "$SCRIPTS/tunnel-health.sh" >/dev/null || fail "tunnel-health should pass on healthy body"

# tunnel-health: wrong body -> non-zero
cat > "$STUB/curl" <<'EOF'
#!/usr/bin/env bash
printf 'nope'
EOF
chmod +x "$STUB/curl"
if PATH="$STUB:$PATH" bash "$SCRIPTS/tunnel-health.sh" >/dev/null 2>&1; then fail "tunnel-health should fail on wrong body"; fi

# tunnel-dns-check: cfargotunnel target -> exit 0
cat > "$STUB/dig" <<'EOF'
#!/usr/bin/env bash
echo "abc123.cfargotunnel.com."
EOF
chmod +x "$STUB/dig"
PATH="$STUB:$PATH" bash "$SCRIPTS/tunnel-dns-check.sh" >/dev/null || fail "tunnel-dns-check should pass on cfargotunnel target"

# tunnel-dns-check: other target -> non-zero
cat > "$STUB/dig" <<'EOF'
#!/usr/bin/env bash
echo "192.0.2.1"
EOF
chmod +x "$STUB/dig"
if PATH="$STUB:$PATH" bash "$SCRIPTS/tunnel-dns-check.sh" >/dev/null 2>&1; then fail "tunnel-dns-check should fail when target is not cfargotunnel"; fi

echo "scripts_test: OK (tunnel-health + tunnel-dns-check happy + negative paths)"
