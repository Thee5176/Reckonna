#!/usr/bin/env bash
# .claude/hooks/require-plan.sh — UserPromptSubmit (allowlist; never blocks /plan)
set -uo pipefail
PROMPT=$(cat | jq -r '.prompt // .user_prompt // empty')
case "$PROMPT" in
  /plan*|/brainstorm*|/write-plan*|/execute-plan*|/review*|/tdd*|/migrate-endpoint*|/help*|/memory*|/caveman*|/graphify*) exit 0 ;;
  *bootstrap*|*scaffold*) exit 0 ;;
esac
[ "${CLAUDE_BOOTSTRAP:-0}" = "1" ] && exit 0
if echo "$PROMPT" | grep -qE '^/(backend|frontend|infra|ship)\b'; then
  grep -lq 'status: approved' plans/*.md 2>/dev/null \
    || { echo "No approved plan. Run /plan and approve Step 1 before implementation." >&2; exit 2; }
fi
exit 0
