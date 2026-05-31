# Terraform provider config — cloud-agnostic. Vault is the secrets source.
# VAULT_ADDR + auth (AppRole in CI / OIDC for humans) come from the ENVIRONMENT, never hardcoded.
terraform {
  required_version = ">= 1.9.0"
  required_providers {
    vault = {
      source  = "hashicorp/vault"
      version = "~> 4.0"
    }
    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = "~> 2.30"
    }
    github = {
      source  = "integrations/github"
      version = "~> 6.4"
    }
  }
}

provider "vault" {
  # address + auth read from VAULT_ADDR / VAULT_TOKEN / AppRole env — see secrets-vault.md
}

provider "kubernetes" {
  # kubeconfig from env / in-cluster — vendor-neutral, no cloud-specific auth here
}

# GitHub provider — owner is fixed; token is rendered from Vault, not from
# .tfvars or env (see secrets-vault.md). Read scope: repo + admin:repo_hook
# on Thee5176/Reckonna only. Token lives at:
#   vault kv get -mount=secret homelab/github/terraform-token
provider "github" {
  owner = "Thee5176"
  token = data.vault_kv_secret_v2.github_terraform.data["token"]
}
