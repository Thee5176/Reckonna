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
    tailscale = {
      source  = "tailscale/tailscale"
      version = "~> 0.17"
    }
  }
}

provider "vault" {
  # address + auth read from VAULT_ADDR / VAULT_TOKEN / AppRole env — see secrets-vault.md
}

provider "kubernetes" {
  # kubeconfig from env / in-cluster — vendor-neutral, no cloud-specific auth here
}
