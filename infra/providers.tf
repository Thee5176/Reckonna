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

variable "kubeconfig_path" {
  description = "Path to kubeconfig. Empty → in-cluster / provider env-var lookup (KUBE_CONFIG_PATH, KUBECONFIG)."
  type        = string
  default     = ""
}

provider "kubernetes" {
  # Local dev (default): var.kubeconfig_path unset → fall back to
  # pathexpand("~/.kube/config") so workstation runs Just Work and the
  # provider doesn't drop to http://localhost:80.
  # CI / in-cluster: set var.kubeconfig_path = "" AND unset $HOME (or run
  # without a home dir) so config_path becomes null and the provider reads
  # its standard env-var chain (KUBE_CONFIG_PATH / KUBECONFIG) or the
  # in-cluster service-account token. Keeps this vendor-neutral.
  config_path = var.kubeconfig_path != "" ? var.kubeconfig_path : try(pathexpand("~/.kube/config"), null)
}
