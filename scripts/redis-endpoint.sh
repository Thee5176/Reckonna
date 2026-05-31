#!/usr/bin/env bash
# scripts/redis-endpoint.sh — resolve the tailnet endpoint for the Reckonna Redis service.
#
# Usage:
#   scripts/redis-endpoint.sh                # hostname + IP, exit 0 on success
#   scripts/redis-endpoint.sh --hostname     # hostname only
#   scripts/redis-endpoint.sh --ip           # tailnet IP only
#   scripts/redis-endpoint.sh --url          # redis:// URL (no creds)
#
# Resolution order (first hit wins):
#   1. `tailscale status --json` — works from any tailnet-joined host without kubeconfig.
#   2. `kubectl get service` — fallback when run from a CI box with kubeconfig but no tailnet.
#
# Exit codes:
#   0  ok
#   1  bad argument
#   2  no resolver available (no tailscale + no kubectl)
#   3  hostname not yet propagated by the operator (retry later)
#
set -euo pipefail

DEVICE="${RECKONNA_REDIS_DEVICE:-redis-reckonna}"
SERVICE_NS="${RECKONNA_REDIS_NS:-redis}"
SERVICE_NAME="${RECKONNA_REDIS_SVC:-redis}"
MODE="full"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --hostname) MODE="hostname" ;;
    --ip)       MODE="ip" ;;
    --url)      MODE="url" ;;
    -h|--help)  sed -n '2,18p' "$0"; exit 0 ;;
    *) echo "redis-endpoint: unknown arg '$1'" >&2; exit 1 ;;
  esac
  shift
done

resolve_from_tailscale() {
  command -v tailscale >/dev/null 2>&1 || return 1
  command -v jq        >/dev/null 2>&1 || return 1
  local json
  json=$(tailscale status --json 2>/dev/null) || return 1
  local host ip
  host=$(echo "$json" | jq -r --arg n "$DEVICE" '
    .Peer[]? | select((.HostName // "") == $n) | .DNSName // ""
  ' | head -1 | sed 's/\.$//')
  ip=$(echo "$json" | jq -r --arg n "$DEVICE" '
    .Peer[]? | select((.HostName // "") == $n) | (.TailscaleIPs[0] // "")
  ' | head -1)
  [[ -n "$host" && -n "$ip" ]] || return 1
  echo "$host" "$ip"
}

resolve_from_kubectl() {
  command -v kubectl >/dev/null 2>&1 || return 1
  local host
  host=$(kubectl get service "$SERVICE_NAME" -n "$SERVICE_NS" \
    -o jsonpath='{.metadata.annotations.tailscale\.com/hostname}' 2>/dev/null) || return 1
  [[ -n "$host" ]] || return 1
  local ip
  ip=$(getent hosts "$host" 2>/dev/null | awk '{print $1}' | head -1 || true)
  echo "$host" "${ip:-unknown}"
}

read -r host ip < <(resolve_from_tailscale || resolve_from_kubectl || echo "")
if [[ -z "${host:-}" ]]; then
  if ! command -v tailscale >/dev/null 2>&1 && ! command -v kubectl >/dev/null 2>&1; then
    echo "redis-endpoint: neither 'tailscale' nor 'kubectl' on PATH. Install one (see docs/redis-tailnet.md)." >&2
    exit 2
  fi
  echo "redis-endpoint: device '$DEVICE' not visible yet — operator may not have published it. Retry in 30s." >&2
  exit 3
fi

case "$MODE" in
  full)     printf 'hostname=%s\nip=%s\n' "$host" "$ip" ;;
  hostname) printf '%s\n' "$host" ;;
  ip)       printf '%s\n' "$ip" ;;
  url)      printf 'redis://%s:6379/\n' "$host" ;;
esac
