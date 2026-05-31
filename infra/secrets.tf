# Secrets are READ from Vault via data sources — NEVER tfvars, NEVER literals (secrets-vault.md).
# Example: the DB credentials the services consume are rendered from Vault, not stored in state inputs.
data "vault_kv_secret_v2" "database" {
  mount = "secret"
  name  = "app/database" # vault kv get -mount=secret app/database
}

# Redis cache password (plan 03, S4). Contract + audit only — the value flows
# into pods via the Vault Agent Injector annotations on the Redis StatefulSet,
# not through Terraform state.
data "vault_kv_secret_v2" "redis" {
  mount = "secret"
  name  = "app/redis" # vault kv get -mount=secret app/redis
}

# OTLP exporter endpoint + API key (plan 03, S9). Same contract: the value
# flows into the OTel Collector DaemonSet via the Vault Agent Injector, never
# through Terraform state or tfvars.
data "vault_kv_secret_v2" "otel_exporter" {
  mount = "secret"
  name  = "app/otel/exporter" # vault kv get -mount=secret app/otel/exporter
}

# Usage elsewhere (illustrative — concrete resources land in the infra feature plan):
#   username = data.vault_kv_secret_v2.database.data["username"]
#   password = data.vault_kv_secret_v2.database.data["password"]
# These values flow into k8s Secrets via the Vault Agent Injector at deploy time, not into tfvars.
