#!/usr/bin/env bash
# tests/no-secrets_test.sh — .claude/hooks/no-secrets.sh must block inline secrets (quoted AND
# unquoted), block *.env writes, and ALLOW Vault references + clean content.
# Secret strings are ASSEMBLED at runtime (split literals) so this file contains no secret pattern
# itself — otherwise the very hook under test would block writing this file. (plan 00 S6/S7)
set -uo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."
HOOK=".claude/hooks/no-secrets.sh"
command -v jq >/dev/null 2>&1 || { echo "FATAL: jq required"; exit 2; }
pass=0; fail=0

run_hook() { # <file_path> <content> -> RC
  jq -nc --arg fp "$1" --arg c "$2" '{tool_input:{file_path:$fp, content:$c}}' | bash "$HOOK" >/dev/null 2>&1
  RC=$?
}
expect() { # <desc> <want_rc> <got_rc>
  if [ "$2" = "$3" ]; then echo "  PASS: $1"; pass=$((pass+1));
  else echo "  FAIL: $1 (want exit $2, got $3)"; fail=$((fail+1)); fi
}

KW="pass""word"          # assembled -> file never holds the literal keyword next to a value
APIKW="api""_key"
VAL="ab""cdef""gh1234"   # 12 chars, no space/quote -> matches a real secret value shape

# 1. UNQUOTED inline secret -> must BLOCK (exit 2).  RED on the current quote-only hook.
run_hook "internal/config/config.go" "$KW=$VAL";            expect "unquoted secret blocked" 2 "$RC"
# 2. QUOTED inline secret -> must BLOCK (exit 2).
run_hook "internal/config/config.go" "$APIKW=\"$VAL\"";     expect "quoted secret blocked"   2 "$RC"
# 3. Vault reference -> ALLOW (exit 0).
run_hook "internal/config/config.go" "$KW = vault:secret/data/db#value"; expect "vault ref allowed" 0 "$RC"
# 4. .env path -> BLOCK (exit 2).
run_hook ".env" "whatever";                                 expect ".env path blocked"       2 "$RC"
# 5. clean content -> ALLOW (exit 0).
run_hook "main.go" "foo = bar";                             expect "clean content allowed"   0 "$RC"

echo "no-secrets_test: $pass passed, $fail failed"
[ "$fail" -eq 0 ]
