#!/usr/bin/env bash
# tests/cloudflared-vault_test.sh — IT5: cloudflared Deployment renders its tunnel token from
# Vault (role reckonna-cloudflared, path secret/data/app/cloudflare/tunnel) and never inlines it.
# Static grep test — no cluster needed.
set -euo pipefail

HERE="$(cd "$(dirname "$0")" && pwd)"
DEP="$HERE/../infra/k8s/cloudflared/deployment.yaml"

fail() { echo "cloudflared-vault: FAIL: $1" >&2; exit 1; }

grep -q 'vault.hashicorp.com/agent-inject: "true"' "$DEP" || fail "no agent-inject annotation"
grep -q 'vault.hashicorp.com/role: reckonna-cloudflared' "$DEP" || fail "role is not reckonna-cloudflared"
grep -q 'agent-inject-template-cloudflared.env' "$DEP" || fail "no Vault render template for the token"
grep -q 'secret/data/app/cloudflare/tunnel' "$DEP" || fail "Vault path secret/data/app/cloudflare/tunnel missing"
# The token must arrive only via the Vault-rendered file — reject any inline container env block.
if grep -Eq '^[[:space:]]*env:[[:space:]]*$' "$DEP"; then
  fail "deployment declares an env: block; the token must come solely from the Vault-rendered file"
fi

echo "cloudflared-vault: OK (IT5 - Vault-rendered token via reckonna-cloudflared, nothing inlined)"
