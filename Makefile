# PR Reviewer — Makefile (dev only)

.PHONY: dev dev-build dev-down dev-logs dev-shell test fmt lint hooks migrate migrate-down migrate-status migrate-new seed help

DEV_COMPOSE := docker compose -f docker-compose.dev.yml

## dev: start the full dev stack in Docker (postgres + backend live-reload + frontend watch)
dev:
	@test -f .env || { echo "Error: .env not found. Run: cp .env.example .env"; exit 1; }
	$(DEV_COMPOSE) up

## dev-build: rebuild dev images, then start the stack (run after changing Dockerfile.dev or deps)
dev-build:
	@test -f .env || { echo "Error: .env not found. Run: cp .env.example .env"; exit 1; }
	$(DEV_COMPOSE) up --build

## dev-down: stop the dev stack (add ARGS=-v to also drop the postgres volume)
dev-down:
	$(DEV_COMPOSE) down $(ARGS)

## dev-logs: tail logs from the dev stack
dev-logs:
	$(DEV_COMPOSE) logs -f

## dev-shell: open a shell inside a running dev container (defaults to app; override with SVC=web)
dev-shell:
	$(DEV_COMPOSE) exec $(or $(SVC),app) sh

## test: run Go tests (with race detector) and TypeScript type-check
test:
	@echo "Running Go tests..."
	go test -race ./...
	@echo "Type-checking frontend..."
	cd web && npx tsc --noEmit

## format: format Go and frontend source files
format:
	gofmt -w .
	cd web && npx prettier --write .

## lint: run golangci-lint (same linter/version as CI)
lint:
	golangci-lint run

## hooks: install the git pre-commit hook (lint/vet/tsc before each commit)
hooks:
	git config core.hooksPath .githooks
	@echo "✓ pre-commit hook installed (.githooks). Bypass once with: git commit --no-verify"

## migrate: apply all pending migrations (app schema + River queue)
migrate:
	go run ./cmd/migrate up

## migrate-down: roll back the most recent app migration
migrate-down:
	go run ./cmd/migrate down

## migrate-status: show the applied vs latest migration version
migrate-status:
	go run ./cmd/migrate status

## migrate-new: scaffold a new migration pair (usage: make migrate-new name=add_foo)
migrate-new:
	@test -n "$(name)" || { echo "usage: make migrate-new name=<description>"; exit 1; }
	@last=$$(ls internal/db/migrations 2>/dev/null | grep -oE '^[0-9]{6}' | sort -n | tail -1); \
	 if [ -z "$$last" ]; then next=000001; else next=$$(printf "%06d" $$((10#$$last + 1))); fi; \
	 up=internal/db/migrations/$${next}_$(name).up.sql; \
	 down=internal/db/migrations/$${next}_$(name).down.sql; \
	 printf -- '-- %s (up)\n' "$(name)" > $$up; \
	 printf -- '-- %s (down)\n' "$(name)" > $$down; \
	 echo "created $$up and $$down"

## seed: seed the database with sample data
seed:
	go run ./cmd/seed

## help: print this help message
help:
	@echo "Available targets:"
	@grep -E '^## [a-zA-Z_-]+:' $(MAKEFILE_LIST) | \
	  awk 'BEGIN {FS = ": "}; {printf "  %-20s %s\n", $$1, $$2}' | \
	  sed 's/^  ## /  /'
