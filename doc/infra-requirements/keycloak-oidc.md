# Infra requirement — Keycloak OIDC provider

| | |
|---|---|
| **Requested by** | backend HEAD (plan 03 — backend-cqrs-core) |
| **For infra plan** | `infra/keycloak-oidc` |
| **Unblocks** | S10 (auth middleware) at runtime · S17 live e2e · deploy |
| **Backend status without it** | NOT blocked — S10 + S17 run against a mock JWKS / testcontainers issuer; this doc is needed only for **live** integration and production deploy |

## What backend needs infra to provide

A self-hosted Keycloak acting as the OIDC provider for the accounting API (command + query
resource servers). Backend validates RS256 JWTs against it; it never talks to Keycloak's admin API.

### 1. Realm + issuer
- A realm (e.g. `reckonna`) with a stable **issuer URL** reachable from both services in-cluster and
  from CI e2e.
- The OIDC discovery document MUST be served at `<issuer>/.well-known/openid-configuration` and expose
  a working `jwks_uri`. Backend fetches + caches JWKS from there (TTL-based); no static key files.

### 2. Client / audience
- A client whose tokens carry an **audience (`aud`)** the backend can pin (e.g. `reckonna-api`).
- Access tokens are **RS256**, include a stable **`sub`** claim (this is the owner id backend scopes
  every row by), plus standard `iss`, `aud`, `exp`.

### 3. Seed subjects for tests (AT3/AT4, IT5)
- At least **two** distinct users (two `sub` values) so owner-scoping and cross-owner 403/404 can be
  exercised against the live stack. Provide a way for e2e to obtain their access tokens
  (direct-grant/password flow on a test client is acceptable for the test realm only).

### 4. Config delivery (secrets policy)
Backend reads these at runtime from **Vault-rendered env** — never hardcoded, never in git:

| Env var | Meaning | Vault path (example) |
|---|---|---|
| `OIDC_ISSUER_URL` | issuer base (discovery root) | `secret/app/reckonna/oidc/issuer` |
| `OIDC_AUDIENCE` | expected `aud` | `secret/app/reckonna/oidc/audience` |

No client secret is required for pure JWT validation (public-key verification via JWKS). If a
confidential client is later needed, add its secret under `secret/app/reckonna/oidc/client-secret`
and render to `OIDC_CLIENT_SECRET` — do not commit it.

## Acceptance check backend will run against the live issuer
1. `GET <OIDC_ISSUER_URL>/.well-known/openid-configuration` → 200, has `jwks_uri`.
2. A token minted for seed user A verifies (sig + `iss` + `aud` + `exp`) and yields A's `sub`.
3. A token with wrong `aud`, wrong issuer, bad signature, or expired → rejected (backend returns 401).
4. e2e: A's token can create + read A's entries; B's token gets 404 (not 403) on A's entry id.

## Interface contract summary (stable surface)
- Discovery: `<issuer>/.well-known/openid-configuration`
- Keys: `jwks_uri` from discovery (RS256)
- Token claims consumed: `iss`, `aud`, `exp`, `sub`
- Everything else in Keycloak is infra's internal detail.
