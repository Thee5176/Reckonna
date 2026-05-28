#!/usr/bin/env bash
# IT6 — Postgres StatefulSet uses Vault Agent Injector for POSTGRES_* (no literal secret).
set -euo pipefail
SS="infra/k8s/postgres/statefulset.yaml"
fail() { echo "vault-injector: $1" >&2; exit 1; }
grep -q 'vault.hashicorp.com/agent-inject: "true"' "$SS" || fail "agent-inject annotation missing"
grep -q 'vault.hashicorp.com/role: "reckonna-postgres"' "$SS" || fail "role annotation missing"
grep -q 'secret/data/app/database' "$SS" || fail "vault path missing"
# Defence in depth: forbid plaintext literal POSTGRES_PASSWORD value.
if grep -E 'POSTGRES_PASSWORD\s*[:=]\s*[^\$"]' "$SS" \
   | grep -v 'Data.data.password' \
   | grep -q .; then
  fail "plaintext POSTGRES_PASSWORD detected"
fi
echo "vault-injector: OK"
