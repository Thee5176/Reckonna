#!/usr/bin/env bash
# .claude/hooks/require-prereq.sh — PreToolUse(Edit|Write): gate each domain on its prerequisite
set -uo pipefail
INPUT=$(cat); FILE=$(echo "$INPUT" | jq -r '.tool_input.file_path // empty')
[ "${CLAUDE_BOOTSTRAP:-0}" = "1" ] && exit 0
FEATURE="${CLAUDE_FEATURE:-$(ls plans/*.md 2>/dev/null | grep -v impl | head -1 | xargs -r basename | sed 's/.md$//')}"
[ -z "$FEATURE" ] && exit 0
PLAN="plans/$FEATURE.md"; DESIGN="design/$FEATURE.design-system.html"
case "$FILE" in
  internal/*|cmd/*|db/*)
    if ! grep -q 'status: approved' "$PLAN" 2>/dev/null \
       || ! grep -q 'Acceptance-test spec' "$PLAN" 2>/dev/null \
       || ! grep -q 'Integration-test spec' "$PLAN" 2>/dev/null; then
      echo "Backend blocked: run /plan STEP 1 — approved acceptance + integration spec required ($PLAN)." >&2
      exit 2
    fi ;;
  app/*|components/*)
    if [ ! -f "$DESIGN" ] || ! grep -qi 'approved' "$DESIGN" 2>/dev/null; then
      echo "Frontend blocked: run /plan STEP 2 — approved HTML design system required ($DESIGN)." >&2
      exit 2
    fi ;;
esac
exit 0
