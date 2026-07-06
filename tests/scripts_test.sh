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

# tunnel-health: curl itself fails (network/timeout) -> exit 1 (request-failed branch)
cat > "$STUB/curl" <<'EOF'
#!/usr/bin/env bash
exit 22
EOF
chmod +x "$STUB/curl"
rc=0; PATH="$STUB:$PATH" bash "$SCRIPTS/tunnel-health.sh" >/dev/null 2>&1 || rc=$?
[ "$rc" -eq 1 ] || fail "tunnel-health should exit 1 when curl fails (got $rc)"

# tunnel-dns-check: dig not installed -> exit 2 (bare PATH with only bash+grep — the
# temp PATH assignment also governs lookup of `bash` itself, so it must be linked in)
BARE="$(mktemp -d)"
trap 'rm -rf "$STUB" "$BARE"' EXIT
ln -s "$(command -v bash)" "$BARE/bash"
ln -s "$(command -v grep)" "$BARE/grep"
rc=0; PATH="$BARE" bash "$SCRIPTS/tunnel-dns-check.sh" >/dev/null 2>&1 || rc=$?
[ "$rc" -eq 2 ] || fail "tunnel-dns-check should exit 2 when dig is missing (got $rc)"

# tunnel-info: kubectl present -> live pod-status branch; kubectl absent -> skip message
cat > "$STUB/kubectl" <<'EOF'
#!/usr/bin/env bash
echo "cloudflared-fake-pod   1/1   Running"
EOF
chmod +x "$STUB/kubectl"
# Capture then grep (a piped `grep -q` exits early -> SIGPIPE -> pipefail false-fails)
info_out=$(PATH="$STUB:$PATH" bash "$SCRIPTS/tunnel-info.sh")
printf '%s' "$info_out" | grep -q 'cloudflared-fake-pod' || fail "tunnel-info should print pod status when kubectl present"
info_out=$(PATH="$BARE" bash "$SCRIPTS/tunnel-info.sh")
printf '%s' "$info_out" | grep -q 'kubectl not on PATH' || fail "tunnel-info should note missing kubectl"

echo "scripts_test: OK (health 0/1/2, dns-check 0/1/2, tunnel-info both branches)"
