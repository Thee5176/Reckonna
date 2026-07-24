# How to Run the Backend Validation Suites

Run the plan-03 unit, integration, and end-to-end tests locally against a real
PostgreSQL, so the DB-level 借方=貸方 trigger and migrations are exercised, not
mocked.

Reference (what the suites prove): [reference-backend-validation.md](reference-backend-validation.md).

## Prerequisites

- Go 1.23, from the repo root (`feat/03-backend-cqrs-core`).
- A real Postgres for integration/e2e, via **one** of:
  - **Container runtime** (testcontainers-go): Docker, or rootless Podman.
  - **Shared PG**: `RECKONNA_TEST_DATABASE_URL`, rendered from Vault. Each test
    gets its own isolated schema (via `search_path`), so a shared DB is safe.

The test DB is resolved in this order (`internal/testsupport/postgres.go`):
`RECKONNA_TEST_DATABASE_URL` → `scripts/render-test-db-url.sh` (auto in
`make test`) → testcontainers-go.

## Option 1 — rootless Podman (testcontainers)

1. Start the Podman API socket:

   ```bash
   systemctl --user start podman.socket
   ```

2. Point testcontainers at it. Either export env per-run:

   ```bash
   export DOCKER_HOST="unix:///run/user/$(id -u)/podman/podman.sock"
   export TESTCONTAINERS_RYUK_DISABLED=true
   ```

   or make it durable + env-free in `~/.testcontainers.properties`:

   ```properties
   docker.host=unix:///run/user/1000/podman/podman.sock
   ryuk.disabled=true
   ```

   Ryuk (the resource-reaper sidecar) cannot bind under rootless Podman, so it
   must be disabled; tests self-clean their schema on completion.

## Option 2 — shared PG from Vault

```bash
export RECKONNA_TEST_DATABASE_URL="$(scripts/render-test-db-url.sh)"
```

The script reads `secret/app/database` from Vault and prints a pgx DSN to stdout
only (never writes a secret to disk). Requires a valid Vault token
(`vault login`). `make test` runs this automatically when the env var is unset.

## Steps

1. Unit + integration (all `internal/...`, race detector):

   ```bash
   make test
   ```

2. End-to-end acceptance (real HTTP handlers in-process + real Postgres):

   ```bash
   go test -tags e2e ./e2e/... -count=1
   ```

3. A single criterion, e.g. the money-precision acceptance (AT11 / IT18):

   ```bash
   go test -tags e2e ./e2e/... -run TestE2E_MoneyPrecision -v
   go test ./internal/domain/... -run TestNewEntry_ExcessivePrecision -v
   ```

4. Migration reversibility (IT8):

   ```bash
   go test ./internal/testsupport/... -run TestMigrations_UpDownUp_Idempotent -v
   ```

## Verification

- `make test` prints `ok` per package, no `FAIL`.
- The e2e suite finishes green (~35s) with per-AT subtests PASS, e.g.
  `TestE2E_SemanticRejections/unbalanced_->_422_unbalanced_entry_(AT2)`.
- Coverage of a package: `go test ./internal/handler/problem/... -cover`.

## Troubleshooting

**`failed to create Docker provider` / `dial unix .../podman.sock: connect: no such file or directory`**
The Podman API socket is not running. `systemctl --user start podman.socket`,
then confirm `DOCKER_HOST` (or `~/.testcontainers.properties`) points at
`unix:///run/user/$(id -u)/podman/podman.sock`. The socket is socket-activated
and can stop between sessions — restart it.

**`render-test-db-url: vault sealed or no valid token`**
Run `vault login` (or set `VAULT_TOKEN`). Or skip Vault entirely and use Option 1
(testcontainers), or set `RECKONNA_TEST_NO_VAULT=1` to force the container path.

**e2e tests fail fast (<0.1s) at setup**
That is a DB-provider failure, not an assertion failure — see the Docker-provider
item above. Real assertion failures take seconds (a container spins up first).

**Ryuk container errors under Podman**
Set `TESTCONTAINERS_RYUK_DISABLED=true` (or `ryuk.disabled=true` in the
properties file).

## Related
- [reference-backend-validation.md](reference-backend-validation.md) — criteria + error codes
- [explanation-balance-invariant.md](explanation-balance-invariant.md) — why the 4dp reject policy
