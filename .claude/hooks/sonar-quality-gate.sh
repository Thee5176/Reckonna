#!/usr/bin/env bash
# .claude/hooks/sonar-quality-gate.sh — SubagentStop (no matcher = ALL subagents)
set -uo pipefail
: "${SONAR_HOST_URL:?SONAR_HOST_URL not set}"
: "${SONAR_TOKEN:?SONAR_TOKEN not set}"
if ! sonar-scanner -Dsonar.host.url="$SONAR_HOST_URL" -Dsonar.token="$SONAR_TOKEN" \
      -Dsonar.qualitygate.wait=true -Dsonar.qualitygate.timeout=300 2>&1; then
  echo "SonarQube Quality Gate FAILED — fix bugs/coverage/duplication/security." >&2
  exit 2
fi
exit 0
