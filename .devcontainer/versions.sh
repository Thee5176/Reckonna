# Pinned toolchain versions — SINGLE SOURCE OF TRUTH.
# Sourced by .devcontainer/post-create.sh (install) AND scripts/deps-check.sh (validate).
# Mirror of the "Pinned dependencies" table in plans/00-bootstrap-deps-vault.md. Bump here only.
# Named .sh (not .env) on purpose: no-secrets.sh blocks all *.env. NO SECRETS here — versions only.
GO_VERSION="1.23"
NODE_VERSION="20"
TERRAFORM_VERSION="1.9"
POSTGRES_VERSION="17"
SQLC_VERSION="v1.27.0"
MIGRATE_VERSION="v4.18.1"
GOLANGCI_LINT_VERSION="v1.62.2"
TBLS_VERSION="v1.79.0"
OAPI_CODEGEN_VERSION="v2.4.1"
GITLEAKS_VERSION="8.21.2"
VAULT_VERSION="1.18.2"
MERMAID_CLI_VERSION="11.4.0"
