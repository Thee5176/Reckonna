#!/usr/bin/env bash
# tests/tf-dns_test.sh — IT9: the DNS record is a proxied CNAME on subdomain "reckonna" (NOT the
# apex). Exactly one cloudflare_record exists, so the apex thee5176.com stays untouched (AT6).
# Static grep test — no cloud, no terraform apply.
set -euo pipefail

HERE="$(cd "$(dirname "$0")" && pwd)"
TF="$HERE/../infra/terraform/cloudflare.tf"

fail() { echo "tf-dns: FAIL: $1" >&2; exit 1; }

grep -q 'resource "cloudflare_record"' "$TF" || fail "no cloudflare_record"
grep -Eq 'name[[:space:]]*=[[:space:]]*"reckonna"' "$TF" || fail "record name is not the reckonna subdomain"
grep -Eq 'type[[:space:]]*=[[:space:]]*"CNAME"' "$TF" || fail "record type is not CNAME"
grep -Eq 'proxied[[:space:]]*=[[:space:]]*true' "$TF" || fail "record is not proxied"

# Apex untouched: exactly one record, and none named "@". (Zone name="thee5176.com" is a
# data source, not a record, so counting records is the safe apex check.)
REC_COUNT=$(grep -c 'resource "cloudflare_record"' "$TF")
[ "$REC_COUNT" -eq 1 ] || fail "expected exactly 1 cloudflare_record, found $REC_COUNT (apex record?)"
if grep -Eq 'name[[:space:]]*=[[:space:]]*"@"' "$TF"; then
  fail "apex record (name = \"@\") present — apex must stay untouched"
fi

echo "tf-dns: OK (IT9 - single reckonna CNAME proxied, apex untouched)"
