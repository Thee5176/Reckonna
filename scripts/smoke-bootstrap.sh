#!/usr/bin/env bash
# AT1 — Acceptance test for plan 00 bootstrap.
# Given a fresh clone. When `make generate && make test`. Then toolchain resolves + build green.
# Empty-state safe: generate/test skip cleanly when db/query and Go packages are absent
# (domain code lands in plan 01). Run from repo root.
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

echo "── AT1 smoke: make generate ──────────────────────────────────────────"
make generate

echo "── AT1 smoke: make test ──────────────────────────────────────────────"
make test

echo "── AT1 smoke: PASS ───────────────────────────────────────────────────"
