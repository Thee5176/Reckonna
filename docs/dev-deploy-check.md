# Dev Deployment Check

Local sanity checks before pushing. Confirms both CQRS services boot, Postgres is reachable, migrations applied, OIDC discovery resolves, and OpenAPI spec matches registered routes.

Runs on developer machine. Not for CI, not for prod. See [otel-telemetry-setup.md](otel-telemetry-setup.md) for observability, [postgres-tailnet.md](postgres-tailnet.md) for shared PG.

---

## 0. Prereqs

| Tool | Version pin | Why |
|------|-------------|-----|
| Go | 1.23 | `go run ./cmd/...` |
| Docker/Podman | any | `make up` (Postgres) |
| `migrate` | golang-migrate v4 | schema apply |
| `sqlc` | latest | codegen |
| `curl` + `jq` | any | probes |

Verify:

```bash
make tools-verify
```

Fails loud on missing/wrong versions. Fix before continuing.

---

## 1. Env vars (Vault-rendered)

Non-negotiable — no `.env` file, no inline credentials. Render via vault agent or direnv. Minimum set per service:

| Var | Required | Notes |
|-----|----------|-------|
| `DATABASE_URL` | yes | pgx URL rendered from Vault (`vault kv get -mount=secret app/reckonna/db/url`) |
| `OIDC_ISSUER_URL` | yes | Keycloak realm root; discovery hit at boot |
| `OIDC_AUDIENCE` | yes | client id / audience claim |
| `PORT` | no | default `8080`. Override for one service to avoid collision |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | no | empty → traces dropped; fine for dev |
| `DEPLOYMENT_ENVIRONMENT` | no | default `homelab` |
| `POSTGRES_USER` / `POSTGRES_PASSWORD` / `POSTGRES_DB` | yes for `make up` | compose reads from env (rendered from vault) |

Verify env loaded, print names only (never values):

```bash
printenv | grep -E '^(DATABASE_URL|OIDC_|PORT|POSTGRES_)' | awk -F= '{print $1"=<set>"}'
```

---

## 2. Boot local stack

```bash
make up          # Postgres StatefulSet-alike via compose
make migrate     # golang-migrate up
make generate    # sqlc + CoA
make test        # go test ./... -race (fast smoke)
```

`make test` green = domain + handler + contract tests pass before you even boot HTTP.

---

## 3. Run both services

Two terminals, distinct ports:

```bash
# terminal A — command (write side)
PORT=8081 go run ./cmd/command

# terminal B — query (read side)
PORT=8082 go run ./cmd/query
```

Expect logs:

```
command service listening on :8081
query service listening on :8082
```

`must()` fatal at boot = check stderr for `setup telemetry`, `connect db`, `oidc discovery`. Root cause is env, not code.

---

## 4. Health probe (public)

```bash
curl -sS http://localhost:8081/command/health | jq
# → {"service":"command","status":"ok"}

curl -sS http://localhost:8082/query/health | jq
# → {"service":"query","status":"ok"}
```

Both 200 + `status:ok` = HTTP + Gin + middleware stack alive. Does NOT prove DB or OIDC (health is registered before auth middleware). Next step verifies those.

---

## 5. DB + OIDC reachability

DB — hit an authenticated read that requires a live pool. Mint a dev JWT via a script that reads client creds from vault (never paste JWT into a tracked file):

```bash
# dev-mint-token.sh pulls Keycloak client creds from vault and exchanges for a JWT
BEARER=$(scripts/dev-mint-token.sh)   # vault-backed helper
curl -sS -H "Authorization: Bearer $BEARER" \
     'http://localhost:8082/query/journal-entries?limit=1' | jq
```

- `200` + json array → DB + OIDC both alive
- `401` → OIDC failed to verify token (issuer/audience mismatch)
- `502/500` with `db` in problem detail → pool cannot reach Postgres

OIDC discovery direct check:

```bash
curl -sS "$OIDC_ISSUER_URL/.well-known/openid-configuration" | jq -r .issuer
```

Must equal `$OIDC_ISSUER_URL` exactly, else discovery will fail at service start.

---

## 6. OpenAPI doc check

Spec is not served over HTTP. Not a bug — treated as source-of-truth for contract tests, not a runtime asset. Verify locally:

```bash
# render the spec
sed -n '1,20p' api/openapi.yaml

# preview UI (Redoc)
npx @redocly/cli preview-docs api/openapi.yaml
# → http://localhost:8080 (Redoc)

# lint
npx @redocly/cli lint api/openapi.yaml
```

Runtime probes for spec endpoints — expected to 404 today:

```bash
curl -sS -o /dev/null -w "%{http_code}\n" http://localhost:8081/docs         # 404 expected
curl -sS -o /dev/null -w "%{http_code}\n" http://localhost:8081/openapi.yaml # 404 expected
```

Contract drift guard (spec ↔ registered routes):

```bash
go test ./internal/handler -run Contract -race -v
```

Green = every route documented, every documented op registered, every response matches schema. Red = fix spec or handler before merge.

---

## 7. Observability smoke (optional)

Skip if `OTEL_EXPORTER_OTLP_ENDPOINT` empty. Else:

```bash
make otel-health          # collector :13133 up
make otel-metrics-smoke   # reckonna_* metrics scraped
bash scripts/otel-trace-smoke.sh
```

---

## 8. Full local mirror of CI gates

```bash
make ci   # tools-verify + build + test + lint
```

Green here → PR-safe. Red here → do NOT push.

---

## Troubleshooting

| Symptom | Root cause | Fix |
|---------|------------|-----|
| `setup telemetry: ...` fatal | bad `OTEL_EXPORTER_OTLP_ENDPOINT` | unset it for dev; empty = no export |
| `oidc discovery: ...` fatal | wrong issuer / no network | curl `.well-known/openid-configuration` first |
| `connect db: ...` fatal | `DATABASE_URL` wrong | `psql "$DATABASE_URL" -c 'select 1'` |
| both services on 8080 | forgot `PORT` override | `PORT=8081` on command, `PORT=8082` on query |
| health 200 but `/query/*` 401 | token issuer ≠ configured issuer | re-mint token from same Keycloak realm |
| contract test red after new route | spec drift | edit `api/openapi.yaml` to match, re-run |
| `make up` waits forever on Postgres | `POSTGRES_*` env unset | render from Vault; compose fails-loud on `${VAR:?}` |

---

## Definition of "dev deploy OK"

All must be true before opening PR:

- [ ] `make ci` green
- [ ] both services boot without fatal
- [ ] both `/*/health` return 200
- [ ] one authenticated `/query/*` returns 200
- [ ] `go test ./internal/handler -run Contract` green
- [ ] `redocly lint api/openapi.yaml` clean
