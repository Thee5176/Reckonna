# `tests/` — Cross-cutting / non-Go test scripts

Tests that aren't Go package tests live here: security-hook tests, infra manifest
grep-tests (IT-numbered per plan), and offline script tests (stubbed `curl`/`dig`/
`tailscale`/`kubectl` — no network, no cluster).

> **Go unit and integration tests live beside the code**
> (`internal/**/*_test.go`, `db/migration/*_test.go`), per Go convention.
> End-to-end tests live in `e2e/`.
> This folder is for shell/tooling/policy tests.

## What lives here

- `no-secrets_test.sh` — `.claude/hooks/no-secrets.sh` blocks inline secrets,
  blocks `*.env` writes, allows Vault references + clean content. (plan 00 S6/S7)
- Plan 01 (postgres + tailnet): `pg-endpoint_test.sh`, `pg-probe_test.sh`,
  `tailnet-smoke` covered via pg-endpoint; `service-annotations_test.sh` (IT5),
  `vault-injector_test.sh` (IT6), `networkpolicy_test.sh` (IT3).
- Plan 02 (cloudflare tunnel): `nginx-content_test.sh` (IT6), `probes_test.sh` (IT7),
  `cloudflared-vault_test.sh` (IT5), `cloudflared-args_test.sh` (IT8),
  `tunnel-config_test.sh` (IT4 + ingress order), `tf-dns_test.sh` (IT9),
  `scripts_test.sh` (tunnel-health / tunnel-dns-check / tunnel-info, all exit codes).
- Sonar quality gate: `qualitygate/` fixtures (intentionally-bad Go — excluded from
  both Sonar scans; exercised by `scripts/test-quality-gate.sh`).

Secret-shaped strings in these tests are assembled at runtime from split literals.
The files themselves contain no secret pattern, so the hook under test and
gitleaks won't block them.

## Run

```bash
for t in tests/*_test.sh; do bash "$t"; done   # exit 0 = all pass
```

CI runs the full `tests/*_test.sh` set in the `k8s manifests + infra tests` job.
