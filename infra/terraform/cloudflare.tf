locals {
  account_id = data.vault_kv_secret_v2.cloudflare_tunnel.data["account_id"]
}

# Zone id for thee5176.com (apex is only READ here, never modified — AT6).
data "cloudflare_zone" "reckonna" {
  name = "thee5176.com"
}

# The named tunnel. Its runtime token (Vault -> cloudflared pod, S2) authenticates the
# connector; this resource just declares the tunnel + its credentials-secret in Cloudflare.
resource "cloudflare_zero_trust_tunnel_cloudflared" "reckonna" {
  account_id = local.account_id
  name       = "reckonna"
  secret     = data.vault_kv_secret_v2.cloudflare_tunnel.data["tunnel_secret"]
}

# Remote-managed ingress config (IT4): reckonna.thee5176.com -> the in-cluster nginx harness,
# with a 404 catch-all. cloudflared pulls this from the CF API at startup (no local --config).
resource "cloudflare_zero_trust_tunnel_cloudflared_config" "reckonna" {
  account_id = local.account_id
  tunnel_id  = cloudflare_zero_trust_tunnel_cloudflared.reckonna.id

  config {
    ingress_rule {
      hostname = "reckonna.thee5176.com"
      service  = "http://reckonna-app.reckonna-app.svc.cluster.local:80"
    }
    ingress_rule {
      service = "http_status:404"
    }
  }
}

# Subdomain CNAME -> tunnel. name = "reckonna" (NOT the apex "@") so thee5176.com is untouched
# (IT9 + AT6). proxied = true routes through Cloudflare to the tunnel.
resource "cloudflare_record" "reckonna" {
  zone_id = data.cloudflare_zone.reckonna.id
  name    = "reckonna"
  type    = "CNAME"
  value   = "${cloudflare_zero_trust_tunnel_cloudflared.reckonna.id}.cfargotunnel.com"
  proxied = true
}
