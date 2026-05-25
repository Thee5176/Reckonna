#!/usr/bin/env bash
# .devcontainer/post-create.sh — install pinned project tooling after the container builds.
# Single source of tool versions for the whole team (plan 00 owns bumps). NO secrets here.
set -euo pipefail

# ── pinned versions — single source: .devcontainer/versions.sh ──────────────
# shellcheck disable=SC1091
source "$(dirname "${BASH_SOURCE[0]}")/versions.sh"

GOBIN="$(go env GOPATH)/bin"
export PATH="$GOBIN:$PATH"

echo "[devcontainer] Go module tools (sqlc, migrate, tbls, oapi-codegen)…"
go install "github.com/sqlc-dev/sqlc/cmd/sqlc@${SQLC_VERSION}"
go install -tags 'postgres' "github.com/golang-migrate/migrate/v4/cmd/migrate@${MIGRATE_VERSION}"
go install "github.com/k1LoW/tbls@${TBLS_VERSION}"
go install "github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@${OAPI_CODEGEN_VERSION}"

echo "[devcontainer] golangci-lint ${GOLANGCI_LINT_VERSION}…"
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh \
  | sh -s -- -b "$GOBIN" "$GOLANGCI_LINT_VERSION"

echo "[devcontainer] gitleaks ${GITLEAKS_VERSION}…"
ARCH="$(dpkg --print-architecture)"; case "$ARCH" in amd64) GL_ARCH=x64 ;; arm64) GL_ARCH=arm64 ;; *) GL_ARCH="$ARCH" ;; esac
curl -sSfL "https://github.com/gitleaks/gitleaks/releases/download/v${GITLEAKS_VERSION}/gitleaks_${GITLEAKS_VERSION}_linux_${GL_ARCH}.tar.gz" \
  | sudo tar -xz -C /usr/local/bin gitleaks

echo "[devcontainer] HashiCorp Vault CLI ${VAULT_VERSION}…"
curl -sSfL "https://releases.hashicorp.com/vault/${VAULT_VERSION}/vault_${VAULT_VERSION}_linux_$(dpkg --print-architecture).zip" -o /tmp/vault.zip
sudo unzip -o /tmp/vault.zip -d /usr/local/bin && rm -f /tmp/vault.zip

echo "[devcontainer] Node CLI tools (mermaid-cli for sequence-diagram render)…"
npm install -g "@mermaid-js/mermaid-cli@${MERMAID_CLI_VERSION}"

echo "[devcontainer] go mod download (if go.mod present)…"
[ -f go.mod ] && go mod download || echo "  (no go.mod yet — plan 00 S1 lands it)"

echo "[devcontainer] validating toolchain against pins…"
bash "$(dirname "${BASH_SOURCE[0]}")/../scripts/deps-check.sh" --tools-only || \
  echo "[devcontainer] WARN: deps-check reported a mismatch — see above."
echo "[devcontainer] ready. Secrets come from Vault at runtime — never baked into this image."
