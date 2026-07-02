#!/usr/bin/env bash
# tests/cloudflared-args_test.sh — IT8: cloudflared runs `tunnel --no-autoupdate run --token ...`
# with NO --config flag (remote-managed tunnel config, pulled from the Cloudflare API at startup).
# Static grep test — no cluster needed.
set -euo pipefail

HERE="$(cd "$(dirname "$0")" && pwd)"
DEP="$HERE/../infra/k8s/cloudflared/deployment.yaml"

fail() { echo "cloudflared-args: FAIL: $1" >&2; exit 1; }

# Check actual manifest lines only — strip YAML comments so a comment mentioning --config
# can't false-trip the guard.
NONCOMMENT="$(grep -vE '^[[:space:]]*#' "$DEP")"
printf '%s\n' "$NONCOMMENT" | grep -q 'tunnel --no-autoupdate run --token' || fail "run command is not 'tunnel --no-autoupdate run --token'"
if printf '%s\n' "$NONCOMMENT" | grep -q -- '--config'; then
  fail "--config present in manifest — tunnel must be remote-managed (no local config)"
fi

echo "cloudflared-args: OK (IT8 — remote-managed run, no --config)"
