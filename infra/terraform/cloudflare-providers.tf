terraform {
  required_version = ">= 1.6"
  required_providers {
    cloudflare = {
      source  = "cloudflare/cloudflare"
      version = "~> 4.0"
    }
    vault = {
      source  = "hashicorp/vault"
      version = "~> 4.0"
    }
  }
}

# Vault provider: VAULT_ADDR + auth (OIDC/AppRole) come from the environment, never the repo.
provider "vault" {}

# Cloudflare credentials come from Vault (IT3: never a .tfvars, never a literal).
# secret/app/cloudflare/tunnel holds: api_token, account_id, tunnel_secret, token.
data "vault_kv_secret_v2" "cloudflare_tunnel" {
  mount = "secret"
  name  = "app/cloudflare/tunnel"
}

provider "cloudflare" {
  api_token = data.vault_kv_secret_v2.cloudflare_tunnel.data["api_token"]
}
