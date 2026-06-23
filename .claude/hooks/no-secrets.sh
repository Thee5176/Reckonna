#!/usr/bin/env bash
# .claude/hooks/no-secrets.sh — PreToolUse(Edit|Write): block .env writes + inline secrets BEFORE the write
set -uo pipefail
INPUT=$(cat); FILE=$(echo "$INPUT" | jq -r '.tool_input.file_path // empty')
case "$FILE" in
  *.env|*.env.*) echo "Policy: secrets belong in Vault, not $FILE. Use 'vault kv get'." >&2; exit 2 ;;
esac
CONTENT=$(echo "$INPUT" | jq -r '.tool_input.content // .tool_input.new_string')
if echo "$CONTENT" | grep -nEi '(password|secret|api[_-]?key|token)[[:space:]]*[:=][[:space:]]*["'\''']?[^"'\''' ]{6,}' \
     | grep -vEqi 'vault|\$\{\{[[:space:]]*(secrets|env)\.'; then
  echo "Policy: possible hardcoded secret — reference Vault, don't inline." >&2
  exit 2
fi
exit 0
