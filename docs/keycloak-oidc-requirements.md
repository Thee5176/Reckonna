# Keycloak OIDC — Requirements (OUTSOURCED)

**Status:** requirements only. **Owner:** external agent/team (NOT plans 03/05).
**Consumed by:** plan 03 (backend JWT validation + owner scoping) and plan 05 (deploy smoke +
frontend SSO login). This doc is the contract; the external agent decides implementation.

Referenced by plan 03 `external_prereq: infra/keycloak-oidc` and plan 05 D12 (`reckonna-smoke`).

---

## 1. Scope

Stand up a self-hosted **Keycloak** as the OIDC provider for Reckonna: a realm, three clients, and
two end-user auth methods (**SSO** + **Passkey/WebAuthn**). Publish the discovery + secrets into
Vault so the Reckonna backend (resource server) and CI can consume them without further coordination.

**In scope (external agent):** Keycloak deploy, realm, clients, auth flows (SSO, Passkey), user
federation/registration policy, realm export, Vault publishing, TLS/hosting.

**Out of scope (Reckonna owns):** backend JWT validation code + owner scoping (plan 03); k8s deploy
of the app + tunnel routing (plan 05); the app UI login screen (plan 04 wires the SPA client).

## 2. Functional requirements

- **R1 — Realm.** One realm `reckonna` (name TBD by agent). Single source of identity for all
  Reckonna apps; extensible to future subdomains.
- **R2 — SSO.** Browser single sign-on: one Keycloak session authenticates the user across every
  realm client (web now, future apps later). Standard Authorization Code + PKCE for the SPA.
- **R3 — Passkey / WebAuthn.** Passwordless login via passkeys (WebAuthn). Users can **register** a
  passkey and **authenticate** with it as a first-class factor (not just 2FA). Password fallback
  policy is the agent's call, but passkey must be a complete login path end to end.
- **R4 — Clients (three):**
  | Client | Type | Grant | Purpose | Consumer |
  |--------|------|-------|---------|----------|
  | `reckonna-web` | public (SPA) | Auth Code + PKCE | human SSO login in the RN-Web app | plan 04 frontend |
  | `reckonna-backend` | bearer-only / resource server | (validates JWTs) | audience the backend checks | plan 03 |
  | `reckonna-smoke` | confidential | client-credentials | CI smoke mints a JWT, no human | plan 05 AT4/AT5/AT10 |
- **R5 — Owner-scoping claim.** Access tokens carry a stable `sub` (subject) claim — plan 03 scopes
  ledger ownership by `sub`. `sub` must be stable per user across sessions and login methods
  (password vs passkey must resolve to the SAME `sub`).
- **R6 — Redirect / origins.** `reckonna-web` allows redirect URI + web origin
  `https://reckonna.thee5176.com/*` (root-served SPA per plan 05 D6). Add localhost origins for dev.

## 3. Integration contract (what Reckonna reads)

The backend + CI need these values, published to **Vault** (never handed over any other way):

- **`secret/app/reckonna/oidc`** (fields): `issuer` (e.g. `https://<keycloak-host>/realms/reckonna`),
  `jwks_uri`, `audience` (= the `reckonna-backend` client/audience), `web_client_id` (= `reckonna-web`).
- **`secret/app/reckonna/smoke-oidc`** (fields): `token_url`, `client_id` (= `reckonna-smoke`),
  `client_secret`. Used by plan 05 `scripts/smoke-token.sh`.

**Reachability:** `issuer` + `jwks_uri` must be reachable **from inside the homelab k3s cluster**
(the backend fetches JWKS at runtime) AND from CI. If Keycloak is behind a tunnel/subdomain, ensure
in-cluster DNS/egress resolves it.

**Token/claims the backend expects:** valid `iss` (= issuer), `aud` includes the backend audience,
`exp`/`nbf` honored, `sub` present + stable (R5). RS256 signing; keys served via JWKS (rotation OK —
backend re-fetches). Optional but useful: `preferred_username`, `email`.

## 4. Non-functional

- **Hosting:** self-hosted (homelab), single instance acceptable for v1; TLS terminated (own subdomain
  or the existing Cloudflare tunnel). No public admin console exposure.
- **Secrets:** Keycloak's own DB/admin creds + the client secret live in Vault; nothing committed.
- **Reproducibility:** export the realm (realm JSON) so it can be re-imported — infra-as-code, not
  click-ops-only.
- **Availability:** the backend hard-depends on JWKS; if Keycloak is down, writes 401. Document the
  restart/recovery path.

## 5. Acceptance (handoff is "done" when)

- **A1** — OIDC discovery `GET {issuer}/.well-known/openid-configuration` returns 200 with a `jwks_uri`,
  reachable from a cluster pod AND from CI.
- **A2** — `reckonna-smoke` client-credentials grant returns a valid JWT (`aud` = backend audience,
  `sub` present). Plan 05 `smoke-token.sh` succeeds.
- **A3** — `reckonna-web` SSO login works: Auth Code + PKCE from `https://reckonna.thee5176.com`, one
  session shared across realm clients.
- **A4** — Passkey path works: a user can register a passkey and log in with it; the resulting token
  has the SAME `sub` as their password login (R5).
- **A5** — Vault paths `secret/app/reckonna/oidc` + `secret/app/reckonna/smoke-oidc` populated with the
  fields in §3.
- **A6** — Realm export committed/stored for re-import.

## 6. Handoff to Reckonna

Once A1–A6 pass, Reckonna:
- plan 03 validates JWTs against `issuer`/`jwks_uri`/`audience` and scopes by `sub`.
- plan 05 VSO syncs `secret/app/reckonna/oidc` into `reckonna-app-env`; `smoke-token.sh` uses
  `secret/app/reckonna/smoke-oidc`; the SPA (plan 04) uses `web_client_id` for login.

**Blocks:** plan 03 auth ATs + plan 05 AT4/AT5/AT10. Deliver before the M-C deploy milestone.
