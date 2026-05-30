# Terraform — Tailscale tailnet ACLs + the k8s namespace the operator runs in.
# OAuth client credentials are read from Vault (never tfvars, never literals).

data "vault_kv_secret_v2" "tailscale_operator" {
  mount = "secret"
  name  = "app/tailscale/operator" # vault kv get -mount=secret app/tailscale/operator
}

provider "tailscale" {
  # OAuth client credentials from Vault (provider ~> 0.17 prefers OAuth over
  # raw api_key — tokens auto-rotate so we don't have to re-mint keys every
  # 90 days). The same vault path stores client_id + client_secret alongside
  # the legacy api_key; we read the OAuth pair here.
  #
  # The OAuth client used here MUST be minted with these scopes in the
  # tailnet admin console:
  #   - Policy File: write      (required for tailscale_acl.policy below)
  #   - Devices: Core (write)   (operator device lifecycle)
  #   - Auth Keys: write        (operator auth key minting)
  # Tailnet defaults to the configured account when omitted.
  oauth_client_id     = data.vault_kv_secret_v2.tailscale_operator.data["client_id"]     # vault
  oauth_client_secret = data.vault_kv_secret_v2.tailscale_operator.data["client_secret"] # vault
}

# Single-owner contract: kustomize (infra/k8s/tailscale/) deliberately does NOT
# declare this Namespace — Terraform is the only writer.
resource "kubernetes_namespace" "tailscale" {
  metadata {
    name = "tailscale"
    labels = {
      "app.kubernetes.io/name"      = "tailscale-operator"
      "app.kubernetes.io/part-of"   = "reckonna"
      "kubernetes.io/metadata.name" = "tailscale"
    }
  }
}

# ACL excerpt — tailnet-side gate that controls who can reach the PG device.
# Edit the JSON inline here so the policy is reviewable in PR. Concrete tag
# owners live in the tailnet admin console (Vault stores the bootstrap token).
resource "tailscale_acl" "policy" {
  acl = jsonencode({
    tagOwners = {
      "tag:k8s-operator" = ["autogroup:admin"]
      "tag:k8s"          = ["tag:k8s-operator"]
      "tag:dev"          = ["autogroup:admin"]
    }
    acls = [
      # Devs reach the PG proxy on 5432.
      {
        action = "accept"
        src    = ["tag:dev"]
        dst    = ["tag:k8s:5432"]
      }
    ]
    ssh = []
  })
  overwrite_existing_content = true
}

output "tailscale_namespace" {
  value       = kubernetes_namespace.tailscale.metadata[0].name
  description = "Namespace that hosts the Tailscale Operator."
}
