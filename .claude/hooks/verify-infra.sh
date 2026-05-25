#!/usr/bin/env bash
# .claude/hooks/verify-infra.sh — PostToolUse(Edit|Write): validate ONLY real IaC files.
#   Terraform : any *.tf
#   Kubernetes: yaml ONLY under kubernetes/, k8s/, or manifests/ — NOT every *.yaml.
# Non-manifest yaml (sqlc.yaml, docker-compose.yml, config/*.yaml) is skipped. If a validator
# isn't installed, skip rather than block (real gate is CI).  (plan 00 S7b)
set -uo pipefail
INPUT=$(cat); FILE=$(echo "$INPUT" | jq -r '.tool_input.file_path // empty')
case "$FILE" in
  *.tf)
    command -v terraform >/dev/null 2>&1 || exit 0
    terraform fmt "$FILE" >/dev/null 2>&1 || true
    terraform validate >/dev/null 2>&1 || { echo "tf validate failed" >&2; exit 2; }
    if command -v tflint >/dev/null 2>&1; then
      tflint >/dev/null 2>&1 || { echo "tflint failed" >&2; exit 2; }
    fi ;;
  *kubernetes/*.yaml|*kubernetes/*.yml|*k8s/*.yaml|*k8s/*.yml|*/manifests/*.yaml|*/manifests/*.yml)
    command -v kubeconform >/dev/null 2>&1 || exit 0
    kubeconform -strict "$FILE" 2>&1 || { echo "kubeconform failed" >&2; exit 2; } ;;
  *) exit 0 ;;
esac
exit 0
