#!/usr/bin/env bash
# IT6 — Redis StatefulSet uses Vault Agent Injector for requirepass (no literal secret).
set -euo pipefail
SS="infra/k8s/redis/statefulset.yaml"
fail() { echo "redis-vault-injector: $1" >&2; exit 1; }
grep -q 'vault.hashicorp.com/agent-inject: "true"' "$SS" || fail "agent-inject annotation missing"
grep -q 'vault.hashicorp.com/role: "reckonna-redis"' "$SS" || fail "role annotation missing"
grep -q 'secret/data/app/redis' "$SS" || fail "vault path missing"
grep -q -- '--include' "$SS" || fail "redis-server --include flag missing"
grep -q '/vault/secrets/redis.conf' "$SS" || fail "rendered conf mount path missing"
# Defence in depth: forbid plaintext requirepass literal (excluding YAML comments
# and the vault template which always references Data.data.password).
if grep -nE '^\s*[^#]*requirepass\s+"[^{][^"]*"' "$SS" \
   | grep -v 'Data.data.password' \
   | grep -q .; then
  fail "plaintext requirepass detected"
fi
echo "redis-vault-injector: OK"
