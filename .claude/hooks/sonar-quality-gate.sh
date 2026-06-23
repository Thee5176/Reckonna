#!/usr/bin/env bash
# .claude/hooks/sonar-quality-gate.sh — SubagentStop (no matcher = ALL subagents)
set -uo pipefail
# If Sonar isn't configured/enabled in this environment, don't block the session.
[ "${SONAR_ENABLED:-}" = "true" ] || exit 0
[ -n "${SONAR_HOST_URL:-}" ] && [ -n "${SONAR_TOKEN:-}" ] || exit 0
if ! sonar-scanner -Dsonar.host.url="$SONAR_HOST_URL" -Dsonar.token="$SONAR_TOKEN" \
      -Dsonar.qualitygate.wait=true -Dsonar.qualitygate.timeout=300 2>&1; then
  echo "SonarQube Quality Gate FAILED — fix bugs/coverage/duplication/security." >&2
  exit 2
fi
exit 0
