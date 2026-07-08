#!/usr/bin/env bash
# scripts/render-test-db-url.sh — print RECKONNA_TEST_DATABASE_URL from Vault to stdout.
#
# Reads shared-PG creds from Vault (secret/app/database) and prints a pgx DSN.
# testsupport.NewPostgres consumes this env var and appends a per-test search_path
# for schema isolation, so pointing at the shared "accounting" DB is safe.
#
# Usage:
#   export RECKONNA_TEST_DATABASE_URL="$(scripts/render-test-db-url.sh)"
#   go test ./...
#
# Or via Makefile (auto-detected by `make test`).
#
# Exit codes:
#   0  DSN printed to stdout
#   1  vault CLI missing
#   2  vault sealed or unauthenticated
#   3  secret path missing / required field empty
#
# NEVER writes secrets to disk. Stdout is the only channel; caller captures into env.
set -euo pipefail

MOUNT="${RECKONNA_TEST_DB_VAULT_MOUNT:-secret}"
PATH_="${RECKONNA_TEST_DB_VAULT_PATH:-app/database}"

command -v vault >/dev/null 2>&1 || { echo "render-test-db-url: 'vault' CLI not on PATH" >&2; exit 1; }

if ! vault token lookup >/dev/null 2>&1; then
  echo "render-test-db-url: vault sealed or no valid token (run 'vault login' or set VAULT_TOKEN)" >&2
  exit 2
fi

read_field() {
  local field="$1" val
  val=$(vault kv get -mount="$MOUNT" -field="$field" "$PATH_" 2>/dev/null) || return 1
  [[ -n "$val" ]] || return 1
  printf '%s' "$val"
}

host=$(read_field host)     || { echo "render-test-db-url: missing field 'host' at $MOUNT/$PATH_" >&2; exit 3; }
user=$(read_field username) || { echo "render-test-db-url: missing field 'username' at $MOUNT/$PATH_" >&2; exit 3; }
pass=$(read_field password) || { echo "render-test-db-url: missing field 'password' at $MOUNT/$PATH_" >&2; exit 3; }
db=$(read_field dbname)     || { echo "render-test-db-url: missing field 'dbname' at $MOUNT/$PATH_" >&2; exit 3; }
port="${RECKONNA_TEST_DB_PORT:-5432}"

# URL-encode password (may contain reserved chars). jq handles this without leaking to stdout logs.
if command -v jq >/dev/null 2>&1; then
  pass_enc=$(printf '%s' "$pass" | jq -sRr @uri)
else
  pass_enc="$pass"  # no jq: rely on absence of reserved chars; fail loud if that breaks
fi

printf 'postgres://%s:%s@%s:%s/%s?sslmode=disable\n' "$user" "$pass_enc" "$host" "$port" "$db"
