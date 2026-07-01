#!/usr/bin/env bash
# scripts/tunnel-info.sh — print the reckonna tunnel wiring: static facts plus live cloudflared
# pod status when a kubeconfig is present. Read-only; no secrets printed.
set -euo pipefail

HOST="${RECKONNA_HOST:-reckonna.thee5176.com}"

echo "hostname:     $HOST"
echo "app_service:  http://reckonna-app.reckonna-app.svc.cluster.local:80"
echo "dns_target:   <tunnel-uuid>.cfargotunnel.com (proxied CNAME)"
echo "ingress:      remote-managed (cloudflared pulls config from the Cloudflare API)"

if command -v kubectl >/dev/null 2>&1; then
  echo "--- cloudflared pods (ns cloudflared) ---"
  kubectl get pods -n cloudflared -l app.kubernetes.io/name=cloudflared 2>/dev/null \
    || echo "(cloudflared namespace not reachable)"
else
  echo "(kubectl not on PATH — skipping live pod status)"
fi
