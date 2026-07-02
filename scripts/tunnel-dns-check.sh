#!/usr/bin/env bash
# scripts/tunnel-dns-check.sh — AT5: reckonna.thee5176.com resolves through the tunnel
# (its CNAME target is *.cfargotunnel.com). Queries the CNAME explicitly (a proxied record
# flattens to Cloudflare edge IPs on an A query, so ask for the CNAME too).
#
# Exit codes: 0 ok | 1 not a cfargotunnel target | 2 dig unavailable
set -euo pipefail

HOST="${RECKONNA_HOST:-reckonna.thee5176.com}"

command -v dig >/dev/null 2>&1 || { echo "tunnel-dns-check: dig not installed" >&2; exit 2; }

out=$( { dig +short CNAME "$HOST"; dig +short "$HOST"; } 2>/dev/null )
if ! printf '%s\n' "$out" | grep -q 'cfargotunnel\.com'; then
  echo "tunnel-dns-check: $HOST does not resolve via *.cfargotunnel.com (got: ${out:-<empty>})" >&2
  exit 1
fi
echo "tunnel-dns-check: OK ($HOST -> *.cfargotunnel.com)"
