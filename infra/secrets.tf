# Secrets are READ from Vault via data sources — NEVER tfvars, NEVER literals (secrets-vault.md).
# Example: the DB credentials the services consume are rendered from Vault, not stored in state inputs.
data "vault_kv_secret_v2" "database" {
  mount = "secret"
  name  = "app/database" # vault kv get -mount=secret app/database
}

# Usage elsewhere (illustrative — concrete resources land in the infra feature plan):
#   username = data.vault_kv_secret_v2.database.data["username"]
#   password = data.vault_kv_secret_v2.database.data["password"]
# These values flow into k8s Secrets via the Vault Agent Injector at deploy time, not into tfvars.
