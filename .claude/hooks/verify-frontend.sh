#!/usr/bin/env bash
# .claude/hooks/verify-frontend.sh — PostToolUse(Edit|Write): eslint + tsc + jest for *.ts(x)
set -uo pipefail
INPUT=$(cat); FILE=$(echo "$INPUT" | jq -r '.tool_input.file_path // empty')
case "$FILE" in *.ts|*.tsx) ;; *) exit 0 ;; esac
npx eslint "$FILE"        2>&1 || { echo "eslint failed" >&2; exit 2; }
npx tsc --noEmit          2>&1 || { echo "tsc failed"    >&2; exit 2; }
npx jest --findRelatedTests "$FILE" --passWithNoTests 2>&1 || { echo "jest failed" >&2; exit 2; }
exit 0
