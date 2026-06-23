# Cloud-agnostic root module skeleton. Concrete infrastructure (vendor-neutral k8s, RDS-equivalent,
# Vault Agent Injector, Keycloak) lands in the infra feature plan. This keeps `terraform validate` green
# from the bootstrap onward.
locals {
  project     = "reckonna"
  environment = terraform.workspace
}

output "project" {
  value = local.project
}

output "environment" {
  value = local.environment
}
