# Terraform — Tailscale tailnet ACLs + the k8s namespace the operator runs in.
# OAuth client credentials are read from Vault (never tfvars, never literals).

data "vault_kv_secret_v2" "tailscale_operator" {
  mount = "secret"
  name  = "app/tailscale/operator" # vault kv get -mount=secret app/tailscale/operator
}

provider "tailscale" {
  # API key from Vault, sourced via the vault provider data block above.
  # Tailnet defaults to the configured account when omitted.
  api_key = data.vault_kv_secret_v2.tailscale_operator.data["api_key"]
}

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
