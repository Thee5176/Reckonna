#!/usr/bin/env bash
# scripts/bootstrap-redis-otel.sh — single-keystroke human-only bootstrap for
# the plan-03 deliverables. Bundles every step the AI cannot run (per
# .claude/rules/devops.md: terraform apply, kubectl apply, vault writes).
#
# Run AFTER the plan-02 bootstrap (PG + Vault Agent Injector + Tailscale
# Operator must already be live). Pre-flights everything, prints what it is
# about to do, then walks the chain idempotently. Re-run-safe.
#
# Usage:
#   scripts/bootstrap-redis-otel.sh                # interactive (prompts for otel creds)
#   OTEL_ENDPOINT=... OTEL_API_KEY=... scripts/bootstrap-redis-otel.sh   # env-driven
#   scripts/bootstrap-redis-otel.sh --dry-run      # print the chain, run nothing
#
# Exit codes:
#   0  all stages OK; redis PONG + otel-smoke 2xx
#   2  preflight failed (missing tool, no auth, no kubeconfig)
#   3  vault writes failed
#   4  terraform apply failed
#   5  kubectl apply / rollout failed
#   6  verification (redis-smoke / otel-smoke) failed
#
set -euo pipefail

DRY=0
[[ "${1:-}" == "--dry-run" ]] && DRY=1

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

step() { printf '\n\033[36m==> %s\033[0m\n' "$*"; }
run()  { if [[ $DRY -eq 1 ]]; then printf '  [dry] %s\n' "$*"; else eval "$@"; fi; }

# ── 1. Preflight ─────────────────────────────────────────────────────────────
step "1/6 preflight"
for tool in vault kubectl terraform jq openssl; do
  command -v "$tool" >/dev/null || { echo "missing: $tool" >&2; exit 2; }
done
vault token lookup >/dev/null 2>&1 || { echo "vault: not authenticated (run 'vault login')" >&2; exit 2; }
kubectl version --request-timeout=3s >/dev/null 2>&1 || { echo "kubectl: cluster unreachable" >&2; exit 2; }
kubectl get ns vault     >/dev/null 2>&1 || { echo "preflight: namespace 'vault' missing — apply plan-02 first" >&2; exit 2; }
kubectl get ns tailscale >/dev/null 2>&1 || { echo "preflight: namespace 'tailscale' missing — apply plan-02 first" >&2; exit 2; }
echo "  ok: tools + auth + plan-02 baseline present"

# ── 2. Vault seed (A4 + A5) ─────────────────────────────────────────────────
step "2/6 seed vault (secret/app/redis + secret/app/otel/exporter)"

if vault kv get -mount=secret app/redis >/dev/null 2>&1; then
  echo "  skip: secret/app/redis already present"
else
  REDIS_PW="$(openssl rand -base64 24)"
  run "vault kv put -mount=secret app/redis password=\"$REDIS_PW\" >/dev/null"
  unset REDIS_PW
  echo "  ok: secret/app/redis seeded"
fi

if vault kv get -mount=secret app/otel/exporter >/dev/null 2>&1; then
  echo "  skip: secret/app/otel/exporter already present"
elif [[ $DRY -eq 1 ]]; then
  echo "  [dry] would prompt for OTEL_ENDPOINT + OTEL_API_KEY and vault kv put"
else
  if [[ -z "${OTEL_ENDPOINT:-}" ]]; then
    echo -n "  OTEL_ENDPOINT (e.g. otlp.backend.example:443): "
    read -r OTEL_ENDPOINT
  fi
  if [[ -z "${OTEL_API_KEY:-}" ]]; then
    echo -n "  OTEL_API_KEY (paste bearer; input hidden): "
    read -rs OTEL_API_KEY
    echo
  fi
  [[ -n "$OTEL_ENDPOINT" && -n "$OTEL_API_KEY" ]] || { echo "  empty OTEL_* — aborting" >&2; exit 3; }
  run "vault kv put -mount=secret app/otel/exporter endpoint=\"$OTEL_ENDPOINT\" api_key=\"$OTEL_API_KEY\" >/dev/null"
  unset OTEL_ENDPOINT OTEL_API_KEY
  echo "  ok: secret/app/otel/exporter seeded"
fi

# ── 3. Vault policy + role (B4 + B5) ────────────────────────────────────────
step "3/6 wire vault policies + kubernetes roles"
run 'vault policy write reckonna-redis - <<"POL"
path "secret/data/app/redis" { capabilities = ["read"] }
POL'
run 'vault policy write reckonna-otel-collector - <<"POL"
path "secret/data/app/otel/exporter" { capabilities = ["read"] }
POL'
run 'vault write auth/kubernetes/role/reckonna-redis bound_service_account_names=redis bound_service_account_namespaces=redis policies=reckonna-redis ttl=1h >/dev/null'
run 'vault write auth/kubernetes/role/reckonna-otel-collector bound_service_account_names=collector bound_service_account_namespaces=otel policies=reckonna-otel-collector ttl=1h >/dev/null'
echo "  ok: policies + roles bound"

# ── 4. Terraform apply (namespaces) ─────────────────────────────────────────
step "4/6 terraform apply (redis + otel namespaces)"
run "cd infra && terraform init -input=false -upgrade >/dev/null && terraform apply -auto-approve -input=false && cd .."
echo "  ok: terraform converged"

# ── 5. kubectl apply (workloads) ────────────────────────────────────────────
step "5/6 apply redis statefulset + otel daemonset"
run "kubectl apply -k infra/k8s/redis"
run "kubectl -n redis rollout status statefulset/redis --timeout=2m"
run "kubectl apply -k infra/k8s/otel"
run "kubectl -n otel rollout status daemonset/otel-collector --timeout=2m"
echo "  ok: pods rolled out"

# ── 6. Verify (AT2 + AT4) ───────────────────────────────────────────────────
step "6/6 verify redis-smoke + otel-smoke"
if [[ $DRY -eq 0 ]]; then
  "$ROOT/scripts/redis-smoke.sh" || { echo "redis-smoke FAILED" >&2; exit 6; }
  NODE_IP="$(kubectl get node -o jsonpath='{.items[0].status.addresses[?(@.type=="InternalIP")].address}')"
  OTEL_TARGET="$NODE_IP:4318" "$ROOT/scripts/otel-smoke.sh" || { echo "otel-smoke FAILED" >&2; exit 6; }
fi

echo
echo "bootstrap-redis-otel: DONE — redis PONG + otel span accepted"
