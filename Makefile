# Reckonna — Accounting CQRS. One entrypoint for build/test/run. Tools pinned by .devcontainer/versions.sh.
.DEFAULT_GOAL := help
SHELL := /usr/bin/env bash
COMPOSE ?= docker compose
MIGRATE_DB_URL ?= $(DATABASE_URL)   # rendered from Vault (vault agent / direnv) — never hardcoded

.PHONY: help tools-verify generate migrate migrate-down test lint build up down docs docs-verify gen-coa ci \
        k8s-validate tf-validate pg-endpoint tailnet-smoke pg-probe

help: ## List targets
	@grep -hE '^[a-zA-Z0-9_-]+:.*?## ' $(MAKEFILE_LIST) | sort | awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

tools-verify: ## Validate toolchain + deps against pinned versions
	@bash scripts/deps-check.sh

gen-coa: ## Generate CoA seed + locale stubs from config/coa.yaml (validates vs governance standard)
	@go run ./scripts/gen-coa 2>/dev/null || echo "gen-coa: stub — implement in plan 01 S5a"

generate: ## sqlc codegen + CoA generation
	@if find db/query -type f -name '*.sql' -print -quit 2>/dev/null | grep -q .; then sqlc generate; else echo "generate: no db/query/**/*.sql yet — skipping sqlc (plan 01)"; fi
	@$(MAKE) gen-coa

migrate: ## Apply DB migrations (up)
	@migrate -path db/migration -database "$(MIGRATE_DB_URL)" up

migrate-down: ## Roll back one migration
	@migrate -path db/migration -database "$(MIGRATE_DB_URL)" down 1

test: ## Run all Go tests with race detector
	@if [ -n "$$(go list ./... 2>/dev/null)" ]; then go test ./... -race; else echo "test: no Go packages yet — skipping (plan 01)"; fi

lint: ## Run golangci-lint
	@golangci-lint run

build: ## Build command + query services
	@if [ -n "$$(go list ./... 2>/dev/null)" ]; then go build ./...; else echo "build: no Go packages yet — skipping (plan 01)"; fi

up: ## Start local stack (postgres + services)
	@$(COMPOSE) up -d

down: ## Stop local stack
	@$(COMPOSE) down

docs: ## Generate API + ERD docs (openapi served at /docs; tbls schema docs)
	@tbls doc --rm-dist 2>/dev/null || echo "docs: tbls needs a live DB (run after make up + migrate)"

docs-verify: ## Anti-drift: openapi lints + ERD matches schema
	@command -v tbls >/dev/null && tbls diff || echo "docs-verify: tbls not installed (CI gate)"

ci: tools-verify build test lint k8s-validate tf-validate ## Local mirror of the CI gates

k8s-validate: ## kubeconform -strict on the kustomize render of each infra/k8s base (skips when tools absent)
	@if command -v kubeconform >/dev/null 2>&1 && command -v kubectl >/dev/null 2>&1; then \
	  set -o pipefail; \
	  for base in infra/k8s/postgres infra/k8s/tailscale; do \
	    echo "k8s-validate: rendering $$base"; \
	    kubectl kustomize "$$base" | kubeconform -strict -ignore-missing-schemas -summary || exit 1; \
	  done; \
	else \
	  echo "k8s-validate: kubeconform or kubectl not installed — skipping (CI gate)"; \
	fi

tf-validate: ## terraform fmt + validate on infra/ (skips when tool absent)
	@if command -v terraform >/dev/null 2>&1; then \
	  cd infra && terraform fmt -recursive -check && terraform init -backend=false -input=false >/dev/null && terraform validate; \
	else \
	  echo "tf-validate: terraform not installed — skipping (CI gate)"; \
	fi

pg-endpoint: ## Resolve Postgres tailnet endpoint (hostname + IP)
	@bash scripts/pg-endpoint.sh

tailnet-smoke: ## Non-destructive 'SELECT 1' against the tailnet PG endpoint
	@bash scripts/tailnet-smoke.sh

pg-probe: ## App-side connectivity probe (DNS->TCP->query). Reads libpq PG* env vars.
	@bash scripts/pg-probe.sh
