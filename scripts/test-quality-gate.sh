#!/usr/bin/env bash
# scripts/test-quality-gate.sh — assert the Sonar QG actually blocks bad code.
#
# Runs the scanner against two fixtures and checks the QG verdict via the
# Sonar Web API:
#   tests/qualitygate/bad/   → expect status = ERROR  (gate blocks)
#   tests/qualitygate/good/  → expect status = OK     (gate passes)
#
# Exits 0 only if BOTH expectations hold. Use this to validate the QG
# definition (Reckonna-Go-CleanArch) after changing thresholds.
#
# Requirements:
#   - sonar-scanner on PATH (or in PATH via npm/brew/asdf)
#   - go on PATH (to produce coverage.out for the good fixture)
#   - jq on PATH (parses Sonar API responses)
#   - env: SONAR_HOST_URL, SONAR_TOKEN  (token needs "Execute Analysis"
#     on both test project keys + read on api/qualitygates)
#
# Optional env:
#   QG_NAME       — quality gate to bind (default: Reckonna-Go-CleanArch)
#   POLL_TIMEOUT  — seconds to wait for QG computation (default: 180)
#   KEEP_PROJECTS — set to 1 to skip post-run project deletion (debug)
#
# Usage:
#   export SONAR_HOST_URL=http://sonar.sonarqube.svc.cluster.local:9000
#   export SONAR_TOKEN=$(vault kv get -mount=secret -field=token homelab/sonar/ci-token)
#   ./scripts/test-quality-gate.sh

set -euo pipefail

: "${SONAR_HOST_URL:?SONAR_HOST_URL not set}"
: "${SONAR_TOKEN:?SONAR_TOKEN not set}"
QG_NAME="${QG_NAME:-Reckonna-Go-CleanArch}"
POLL_TIMEOUT="${POLL_TIMEOUT:-180}"
KEEP_PROJECTS="${KEEP_PROJECTS:-0}"

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BAD_DIR="$REPO_ROOT/tests/qualitygate/bad"
GOOD_DIR="$REPO_ROOT/tests/qualitygate/good"

for tool in sonar-scanner go jq curl; do
  command -v "$tool" >/dev/null || { echo "missing tool: $tool" >&2; exit 2; }
done

api() {
  local method="$1" path="$2"; shift 2
  curl -fsS -u "$SONAR_TOKEN:" -X "$method" "$SONAR_HOST_URL$path" "$@"
}

# Ensure both test projects exist and are bound to the target QG.
ensure_project() {
  local key="$1" name="$2"
  api POST "/api/projects/create" \
    --data-urlencode "project=$key" \
    --data-urlencode "name=$name" >/dev/null 2>&1 || true
  api POST "/api/qualitygates/select" \
    --data-urlencode "gateName=$QG_NAME" \
    --data-urlencode "projectKey=$key" >/dev/null
}

delete_project() {
  local key="$1"
  [ "$KEEP_PROJECTS" = "1" ] && { echo "  (KEEP_PROJECTS=1, skipping delete of $key)"; return; }
  api POST "/api/projects/delete" --data-urlencode "project=$key" >/dev/null || true
}

# Run scanner; print CE task id so we can poll the QG verdict.
scan() {
  local dir="$1" key="$2"
  (
    cd "$dir"
    # produce coverage for the good fixture; bad has no tests on purpose
    if ls ./*_test.go >/dev/null 2>&1; then
      go test ./... -race -covermode=atomic -coverprofile=coverage.out >/dev/null
    else
      : > coverage.out
    fi
    sonar-scanner \
      -Dsonar.host.url="$SONAR_HOST_URL" \
      -Dsonar.token="$SONAR_TOKEN" \
      -Dsonar.projectKey="$key" \
      -Dsonar.qualitygate.wait=true \
      -Dsonar.qualitygate.timeout="$POLL_TIMEOUT" 2>&1 | tee /tmp/scan.$$.log >/dev/null
    grep -oE 'task\?id=[A-Za-z0-9_-]+' /tmp/scan.$$.log | head -1 | cut -d= -f2
  )
}

# Poll until the CE task finishes, then return the QG status (OK|ERROR|WARN|NONE).
qg_status() {
  local task="$1" deadline=$((SECONDS + POLL_TIMEOUT))
  local analysis_id=""
  while [ -z "$analysis_id" ]; do
    [ $SECONDS -gt $deadline ] && { echo "TIMEOUT" >&2; return 1; }
    analysis_id="$(api GET "/api/ce/task?id=$task" \
      | jq -r 'select(.task.status=="SUCCESS") | .task.analysisId // empty')"
    [ -z "$analysis_id" ] && sleep 2
  done
  api GET "/api/qualitygates/project_status?analysisId=$analysis_id" \
    | jq -r '.projectStatus.status'
}

# Pretty per-condition breakdown — helps see WHICH condition tripped.
qg_conditions() {
  local key="$1"
  api GET "/api/qualitygates/project_status?projectKey=$key" \
    | jq -r '.projectStatus.conditions[] | "  [\(.status)] \(.metricKey) \(.comparator) \(.errorThreshold) (actual: \(.actualValue))"'
}

run_case() {
  local label="$1" dir="$2" key="$3" expect="$4"
  echo "--- $label ($key) — expect $expect ---"
  ensure_project "$key" "Reckonna QG Test ($label)"
  local task; task="$(scan "$dir" "$key")"
  [ -z "$task" ] && { echo "  no CE task id returned" >&2; return 1; }
  echo "  ce-task: $task"
  local status; status="$(qg_status "$task")"
  echo "  qg-status: $status"
  qg_conditions "$key"
  if [ "$status" = "$expect" ]; then
    echo "  PASS"
    return 0
  else
    echo "  FAIL (got $status, expected $expect)" >&2
    return 1
  fi
}

cleanup() {
  delete_project "reckonna-qg-test-bad"
  delete_project "reckonna-qg-test-good"
}
trap cleanup EXIT

rc=0
run_case "BAD"  "$BAD_DIR"  "reckonna-qg-test-bad"  "ERROR" || rc=1
run_case "GOOD" "$GOOD_DIR" "reckonna-qg-test-good" "OK"    || rc=1

if [ $rc -eq 0 ]; then
  echo "QG verified — '$QG_NAME' blocks bad, passes good."
else
  echo "QG verification FAILED — review conditions above." >&2
fi
exit $rc
