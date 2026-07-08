#!/usr/bin/env bash
# .claude/hooks/no-secrets.sh — PreToolUse(Edit|Write): block .env writes + inline secrets BEFORE the write
set -uo pipefail
INPUT=$(cat); FILE=$(echo "$INPUT" | jq -r '.tool_input.file_path // empty')
case "$FILE" in
  *.env|*.env.*) echo "Policy: secrets belong in Vault, not $FILE. Use 'vault kv get'." >&2; exit 2 ;;
esac
CONTENT=$(echo "$INPUT" | jq -r '.tool_input.content // .tool_input.new_string')
# Build the regex in a double-quoted string so we never hand-escape single quotes
# inside a single-quoted literal. The previous version did, with an odd number of
# quotes, which left bash with "unexpected EOF looking for matching '" and made the
# hook exit 2 on EVERY edit regardless of content. SQ holds a literal single quote
# via ANSI-C quoting; QUOTE is the optional surrounding quote char (" or ').
SQ=$'\''
QUOTE="[\"$SQ]"
SECRET_RE="(password|secret|api[_-]?key|token)[[:space:]]*[:=][[:space:]]*${QUOTE}?[^\"$SQ ]{6,}"
if echo "$CONTENT" | grep -nEi "$SECRET_RE" \
     | grep -vEqi 'vault|\$\{\{[[:space:]]*(secrets|env)\.'; then
  echo "Policy: possible hardcoded secret — reference Vault, don't inline." >&2
  exit 2
fi
exit 0
