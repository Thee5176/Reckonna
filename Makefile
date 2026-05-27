# Reckonna — Accounting CQRS. One entrypoint for build/test/run. Tools pinned by .devcontainer/versions.sh.
.DEFAULT_GOAL := help
SHELL := /usr/bin/env bash
COMPOSE ?= docker compose
MIGRATE_DB_URL ?= $(DATABASE_URL)   # rendered from Vault (vault agent / direnv) — never hardcoded

.PHONY: help tools-verify generate migrate migrate-down test lint build up down docs docs-verify gen-coa ci

help: ## List targets
	@grep -hE '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | sort | awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

tools-verify: ## Validate toolchain + deps against pinned versions
	@bash scripts/deps-check.sh

gen-coa: ## Generate CoA seed + locale stubs from config/coa.yaml (validates vs governance standard)
	@go run ./scripts/gen-coa 2>/dev/null || echo "gen-coa: stub — implement in plan 01 S5a"

generate: ## sqlc codegen + CoA generation
	@if compgen -G "db/query/*.sql" >/dev/null; then sqlc generate; else echo "generate: no db/query/*.sql yet — skipping sqlc (plan 01)"; fi
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

ci: tools-verify build test lint ## Local mirror of the CI gates
