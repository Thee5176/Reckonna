#!/usr/bin/env bash
# scripts/deploy.sh — plan-02 Cloudflare Tunnel ingress deploy runbook (HUMAN-ONLY).
#
# `terraform apply` and `kubectl apply` are human-only (devops.md). This script is the
# vehicle a human runs to perform them safely, in the documented order, with preflight
# gates + post-deploy smoke. It executes NOTHING destructive by default.
#
# Canonical procedure: docs/cloudflare-tunnel-setup.md. This wraps it — it does not diverge.
# No secret value is ever written here or printed: Terraform reads the Cloudflare API token
# from Vault (cloudflare-providers.tf data source); cloudflared gets its connector token via
# the Vault Agent Injector. This script only *checks* Vault has them, never reads their values.
#
# Usage:
#   scripts/deploy.sh preflight   # checks only (tools, kube context, Vault secret+role, manifests valid)
#   scripts/deploy.sh plan        # preflight + `terraform plan` + `kubectl diff` (READ-ONLY)  [default]
#   scripts/deploy.sh apply       # the real deploy: tf apply + kubectl apply -k + rollout wait + smoke
#   scripts/deploy.sh verify      # post-deploy smoke only (AT5 dns, AT1 health, AT2 hello, pods)
#   scripts/deploy.sh rollback    # apex-safe: destroy the reckonna CNAME + scale connector to 0
#
# Env: VAULT_ADDR + a valid Vault login (token) required for the Terraform Vault data source.
#      Override the public URL for smoke with RECKONNA_URL (defaults to prod).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)" || exit 1
TF_DIR="$ROOT/infra/terraform"
APP_KUSTOMIZE="$ROOT/infra/k8s/reckonna-app"
CFD_KUSTOMIZE="$ROOT/infra/k8s/cloudflared"
APEX_URL="https://thee5176.com/"
WORK="$(mktemp -d)"; trap 'rm -rf "$WORK"' EXIT

c_red=$'\033[31m'; c_grn=$'\033[32m'; c_ylw=$'\033[33m'; c_rst=$'\033[0m'
info() { printf '%s[deploy]%s %s\n' "$c_grn" "$c_rst" "$1"; }
warn() { printf '%s[deploy] WARN:%s %s\n' "$c_ylw" "$c_rst" "$1" >&2; }
die()  { printf '%s[deploy] FAIL:%s %s\n' "$c_red" "$c_rst" "$1" >&2; exit 1; }

need() { command -v "$1" >/dev/null 2>&1 || die "required tool not found: $1"; }

preflight() {
  info "preflight — tools"
  for t in terraform kubectl vault make curl; do need "$t"; done

  info "preflight — kube context reachable"
  kubectl cluster-info >/dev/null 2>&1 || die "kubectl cannot reach a cluster; check your context (homelab k3s)"
  info "  context: $(kubectl config current-context 2>/dev/null || echo '?')"

  info "preflight — Vault reachable + authenticated"
  [ -n "${VAULT_ADDR:-}" ] || die "VAULT_ADDR unset — Terraform's vault provider needs it"
  vault token lookup >/dev/null 2>&1 || die "not logged in to Vault (vault token lookup failed)"

  info "preflight — Vault secret has required fields (values NOT printed)"
  for f in api_token account_id token; do
    vault kv get -mount=secret -field="$f" app/cloudflare/tunnel >/dev/null 2>&1 \
      || die "secret/app/cloudflare/tunnel missing field '$f' — seed it (docs/cloudflare-tunnel-setup.md § Seed Vault)"
  done

  info "preflight — Vault k8s-auth role reckonna-cloudflared exists"
  vault read auth/kubernetes/role/reckonna-cloudflared >/dev/null 2>&1 \
    || die "Vault role reckonna-cloudflared missing — create it (docs § Vault k8s-auth role)"

  info "preflight — manifests + terraform validate (offline gate)"
  make -C "$ROOT" k8s-validate tf-validate >/dev/null 2>&1 || die "make k8s-validate/tf-validate failed — fix before deploying"

  info "preflight OK"
}

capture_apex() { curl -sf --max-time 10 "$APEX_URL" > "$1" 2>/dev/null || : ; }  # AT6 baseline; apex may 000 — that's fine, we compare before/after

apex_check() {
  # AT6: the apply must not change the apex. Loud if it did.
  if diff -q "$WORK/apex-before" "$WORK/apex-after" >/dev/null 2>&1; then
    info "AT6 apex unchanged ($APEX_URL)"
  else
    warn "AT6 apex response CHANGED across the apply — investigate; Terraform must never write the apex"
  fi
}

do_plan() {
  preflight
  info "terraform init + plan (read-only)"
  terraform -chdir="$TF_DIR" init -input=false >/dev/null
  terraform -chdir="$TF_DIR" plan -input=false
  info "kubectl diff (read-only; nonzero exit just means 'differs from live')"
  kubectl diff -k "$APP_KUSTOMIZE" || true
  kubectl diff -k "$CFD_KUSTOMIZE" || true
  info "plan complete — no changes applied. Run 'scripts/deploy.sh apply' to deploy."
}

do_apply() {
  preflight
  printf '%s[deploy]%s About to terraform apply + kubectl apply to %s. Type "apply" to proceed: ' "$c_ylw" "$c_rst" "$(kubectl config current-context)"
  read -r confirm || confirm=""
  [ "$confirm" = "apply" ] || die "aborted (got '$confirm', expected 'apply')"

  capture_apex "$WORK/apex-before"

  info "1/3 terraform — cloudflare zone/DNS/tunnel/ingress"
  terraform -chdir="$TF_DIR" init -input=false >/dev/null
  terraform -chdir="$TF_DIR" apply -input=false -auto-approve

  info "2/3 kubectl — reckonna-app (tunnel target), then cloudflared (connector)"
  kubectl apply -k "$APP_KUSTOMIZE"
  kubectl apply -k "$CFD_KUSTOMIZE"

  info "waiting for rollouts"
  kubectl -n reckonna-app rollout status deployment/reckonna-app --timeout=120s
  kubectl -n cloudflared rollout status deployment/cloudflared --timeout=120s

  capture_apex "$WORK/apex-after"; apex_check
  info "3/3 verify"
  do_verify || warn "smoke checks not all green yet — DNS/tunnel may need up to ~60s to propagate; re-run 'scripts/deploy.sh verify'"
}

do_verify() {
  local rc=0
  make -C "$ROOT" tunnel-dns-check || rc=1     # AT5
  make -C "$ROOT" tunnel-health    || rc=1     # AT1
  RECKONNA_URL="${RECKONNA_URL:-https://reckonna.thee5176.com}" bash "$ROOT/tests/hello_e2e_test.sh" || rc=1  # AT2
  kubectl get pods -n cloudflared -o wide || rc=1
  if [ "$rc" -eq 0 ]; then info "verify: all smoke checks green"; else warn "verify: one or more checks failed"; fi
  return "$rc"
}

do_rollback() {
  preflight
  warn "apex-safe rollback: drops ONLY the reckonna subdomain CNAME + scales the connector to 0. Apex untouched."
  printf '%s[deploy]%s Type "rollback" to proceed: ' "$c_ylw" "$c_rst"
  read -r confirm || confirm=""
  [ "$confirm" = "rollback" ] || die "aborted"
  terraform -chdir="$TF_DIR" destroy -input=false -auto-approve -target=cloudflare_record.reckonna
  kubectl scale deployment/cloudflared -n cloudflared --replicas=0
  info "rollback done — subdomain removed, connector stopped. apex ($APEX_URL) untouched."
}

case "${1:-plan}" in
  preflight) preflight ;;
  plan)      do_plan ;;
  apply)     do_apply ;;
  verify)    do_verify ;;
  rollback)  do_rollback ;;
  *) die "unknown command '$1' — use: preflight | plan | apply | verify | rollback" ;;
esac
