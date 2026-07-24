#!/usr/bin/env bash
# tests/cloudflared-args_test.sh — IT8: cloudflared runs `tunnel --no-autoupdate run --token ...`
# with NO --config flag (remote-managed tunnel config, pulled from the Cloudflare API at startup).
# Static grep test — no cluster needed.
set -euo pipefail

HERE="$(cd "$(dirname "$0")" && pwd)"
DEP="$HERE/../infra/k8s/cloudflared/deployment.yaml"

fail() { echo "cloudflared-args: FAIL: $1" >&2; exit 1; }

# Check actual manifest lines only — strip YAML comments so a comment mentioning --config
# or --token can't false-trip the guards.
NONCOMMENT="$(grep -vE '^[[:space:]]*#' "$DEP")"
printf '%s\n' "$NONCOMMENT" | grep -q 'tunnel --no-autoupdate run' || fail "run command is not 'tunnel --no-autoupdate run'"
# Token must reach cloudflared via the ENVIRONMENT (read natively from TUNNEL_TOKEN),
# never on argv where /proc exposes it. (Deviation from the plan's literal 'run --token',
# approved 2026-07-06.)
if printf '%s\n' "$NONCOMMENT" | grep -q -- '--token'; then
  fail "--token on argv — token must be env-only (TUNNEL_TOKEN), argv leaks via /proc"
fi
printf '%s\n' "$NONCOMMENT" | grep -q 'export TUNNEL_TOKEN' || fail "TUNNEL_TOKEN not exported from the Vault-rendered file"
if printf '%s\n' "$NONCOMMENT" | grep -q -- '--config'; then
  fail "--config present in manifest — tunnel must be remote-managed (no local config)"
fi

echo "cloudflared-args: OK (IT8 — remote-managed run, env-only token, no --config)"
