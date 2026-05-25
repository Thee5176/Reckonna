# `tests/` — Cross-cutting / non-Go test scripts

Tests that aren't Go package tests live here. Today: the security-hook tests.

> **Go unit & integration tests live BESIDE the code** (`internal/**/*_test.go`,
> `db/migration/*_test.go`), per Go convention — NOT in this folder. End-to-end tests live in
> `e2e/`. This folder is for shell/tooling/policy tests.

## What lives here
| File | Verifies |
|------|----------|
| `no-secrets_test.sh` | `.claude/hooks/no-secrets.sh` blocks inline secrets (quoted **and** unquoted), blocks `*.env` writes, allows Vault references + clean content (plan 00 S6/S7). |

Secret-shaped strings in these tests are **assembled at runtime from split literals** — the test files
themselves contain no secret pattern, so the very hook under test (and gitleaks) won't block them.

## Run
```bash
bash tests/no-secrets_test.sh      # exit 0 = all pass
```
CI runs these as part of the security gate.
