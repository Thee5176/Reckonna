#!/usr/bin/env bash
# scripts/deps-check.sh — validate the installed toolchain + module deps against the pinned versions.
# Single source of pins: .devcontainer/versions.sh. Wired to `make tools-verify` and CI (plan 00 IT4).
# Exit 0 = all good; 1 = a tool is missing/mismatched; 2 = setup error.
#
# Usage: scripts/deps-check.sh [--tools-only]
set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PINS="$ROOT/.devcontainer/versions.sh"
[ -f "$PINS" ] || { echo "FATAL: pins file not found: $PINS" >&2; exit 2; }
# shellcheck disable=SC1090
source "$PINS"

TOOLS_ONLY=0
[ "${1:-}" = "--tools-only" ] && TOOLS_ONLY=1

fail=0
norm() { echo "${1#v}"; }   # strip a leading "v"

# check <label> <expected-version> <command-that-prints-version>
check() {
  local name="$1" want got bin; want="$(norm "$2")"; local cmd="$3"
  bin="${cmd%% *}"   # first token = the binary
  if ! command -v "$bin" >/dev/null 2>&1; then
    printf '  %-16s MISSING        (want %s)\n' "$name" "$want"; fail=1; return
  fi
  got="$(eval "$cmd" 2>/dev/null)" || true
  if [ -z "$got" ]; then
    printf '  %-16s NO-VERSION     (want %s)\n' "$name" "$want"; fail=1; return
  fi
  if echo "$got" | grep -qF "$want"; then
    printf '  %-16s OK    %s\n' "$name" "$want"
  else
    printf '  %-16s MISMATCH want %s got "%s"\n' "$name" "$want" "$got"; fail=1
  fi
}

echo "── Toolchain (pins: .devcontainer/versions.sh) ─────────────────────────"
check go            "$GO_VERSION"            'go version | grep -oE "go[0-9]+\.[0-9]+" | head -1'
check node          "$NODE_VERSION"          'node -v | grep -oE "[0-9]+" | head -1'
check terraform     "$TERRAFORM_VERSION"     'terraform version 2>/dev/null | head -1 | grep -oE "[0-9]+\.[0-9]+" | head -1'
check sqlc          "$SQLC_VERSION"          'sqlc version 2>&1'
check migrate       "$MIGRATE_VERSION"       'migrate -version 2>&1 | head -1'
check golangci-lint "$GOLANGCI_LINT_VERSION" 'golangci-lint version 2>&1 | head -1'
check tbls          "$TBLS_VERSION"          'tbls version 2>&1'
check oapi-codegen  "$OAPI_CODEGEN_VERSION"  'oapi-codegen --version 2>&1 | head -1'
check gitleaks      "$GITLEAKS_VERSION"      'gitleaks version 2>&1'
check vault         "$VAULT_VERSION"         'vault version 2>&1 | head -1'
check mmdc          "$MERMAID_CLI_VERSION"   'mmdc --version 2>&1 | head -1'

if [ "$TOOLS_ONLY" -eq 0 ]; then
  echo "── Go modules ──────────────────────────────────────────────────────────"
  if [ -f "$ROOT/go.mod" ]; then
    godir="$(grep -oE '^go [0-9]+\.[0-9]+' "$ROOT/go.mod" | awk '{print $2}')"
    if [ "$godir" = "$GO_VERSION" ]; then
      printf '  %-16s OK    %s\n' "go.mod directive" "$GO_VERSION"
    else
      printf '  %-16s MISMATCH want %s got "%s"\n' "go.mod directive" "$GO_VERSION" "${godir:-none}"; fail=1
    fi
    if ( cd "$ROOT" && go mod verify >/dev/null 2>&1 ); then
      printf '  %-16s OK\n' "go mod verify"
    else
      printf '  %-16s FAIL (run: go mod download)\n' "go mod verify"; fail=1
    fi
  else
    printf '  %-16s skip  (no go.mod yet — plan 00 S1)\n' "go.mod"
  fi

  echo "── Node deps ───────────────────────────────────────────────────────────"
  if [ -f "$ROOT/package.json" ]; then
    if [ -d "$ROOT/node_modules" ]; then
      printf '  %-16s OK    (node_modules present)\n' "package.json"
    else
      printf '  %-16s WARN  (run: npm ci)\n' "package.json"
    fi
  else
    printf '  %-16s skip  (no package.json yet — plan 00 S4)\n' "package.json"
  fi
fi

echo "────────────────────────────────────────────────────────────────────────"
if [ "$fail" -eq 0 ]; then echo "deps-check: PASS"; else echo "deps-check: FAIL"; fi
exit "$fail"
