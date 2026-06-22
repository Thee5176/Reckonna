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

# GitHub PAT for the github provider is intentionally NOT read as a data source:
# Terraform persists every data-source result in tfstate, which would write an
# admin-capable PAT into terraform.tfstate (no encrypted backend here). Instead,
# export it from Vault into GITHUB_TOKEN before `terraform apply` (apply is
# human-only per devops.md) so it never enters state:
#   export GITHUB_TOKEN=$(vault kv get -mount=secret -field=token homelab/github/terraform-token)
# Mint the PAT (fine-grained, scope repo + admin:repo_hook on Thee5176/Reckonna) via
# GitHub UI, then: vault kv put secret/homelab/github/terraform-token token=<PAT>
