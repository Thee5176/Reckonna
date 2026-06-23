---
description: Secrets & env-var policy (loads every session, all agents)
---
# Secrets & Env Vars — self-hosted HashiCorp Vault is the ONLY source
- ALL environment variables and secrets live in Vault. There is no other source of truth.
- NEVER write a real secret into any file: .env, .env.*, compose, k8s manifests,
  Terraform .tfvars, fixtures, or code. Reference a Vault path instead.
- Read a value at dev time via CLI; never paste into a tracked file:
    vault kv get -mount=secret app/<service>/<key>
- Runtime env is RENDERED from Vault (vault agent / k8s Vault Agent Injector), not committed.
- Auth: AppRole (CI) or OIDC (humans). VAULT_ADDR + role-id/secret-id from env, never repo.
- Terraform: pull secrets via the `vault` provider data sources; never .tfvars.
# Enforced by: no-secrets.sh (PreToolUse) + settings.json deny + gitleaks in CI
