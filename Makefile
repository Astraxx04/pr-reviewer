# PR Reviewer — Makefile (dev only)

.PHONY: up build down logs logs-app logs-web logs-postgres logs-ngrok shell test format lint hooks migrate migrate-down migrate-status migrate-new seed help

DEV_COMPOSE := docker compose -f docker-compose.dev.yml

## up: start the full dev stack (postgres + backend live-reload + frontend watch)
up:
	@test -f .env || { echo "Error: .env not found. Run: cp .env.example .env"; exit 1; }
	$(DEV_COMPOSE) up -d

## build: rebuild dev images, then start the stack (run after changing Dockerfile.dev or deps)
build:
	@test -f .env || { echo "Error: .env not found. Run: cp .env.example .env"; exit 1; }
	$(DEV_COMPOSE) up --build -d

## down: stop the dev stack (add ARGS=-v to also drop the postgres volume)
down:
	$(DEV_COMPOSE) down $(ARGS)

## logs: tail logs from all services
logs:
	$(DEV_COMPOSE) logs -f

## logs-app: tail logs from the backend container
logs-app:
	$(DEV_COMPOSE) logs -f app

## logs-web: tail logs from the frontend container
logs-web:
	$(DEV_COMPOSE) logs -f web

## logs-postgres: tail logs from the postgres container
logs-postgres:
	$(DEV_COMPOSE) logs -f postgres

## logs-ngrok: tail logs from the ngrok container
logs-ngrok:
	$(DEV_COMPOSE) logs -f ngrok

## shell: open a shell inside a running dev container (defaults to app; override with ser=web)
shell:
	$(DEV_COMPOSE) exec $(or $(ser),app) sh

## test: run Go tests (with race detector) and TypeScript type-check
test:
	$(DEV_COMPOSE) exec app go test -race ./...
	$(DEV_COMPOSE) exec web npx tsc --noEmit

## format: format Go and frontend source files
format:
	$(DEV_COMPOSE) exec app gofmt -w .
	$(DEV_COMPOSE) exec web npx prettier --write .

## lint: run golangci-lint (same linter/version as CI)
lint:
	$(DEV_COMPOSE) exec app golangci-lint run

## hooks: install the git pre-commit hook (lint/vet/tsc before each commit)
hooks:
	git config core.hooksPath .githooks
	@echo "✓ pre-commit hook installed (.githooks). Bypass once with: git commit --no-verify"

## migrate: apply all pending migrations
migrate:
	$(DEV_COMPOSE) exec app go run ./cmd/migrate up

## migrate-down: roll back the most recent app migration
migrate-down:
	$(DEV_COMPOSE) exec app go run ./cmd/migrate down

## migrate-status: show the applied vs latest migration version
migrate-status:
	$(DEV_COMPOSE) exec app go run ./cmd/migrate status

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
	$(DEV_COMPOSE) exec app go run ./cmd/seed

## help: print this help message
help:
	@echo "Available targets:"
	@grep -E '^## [a-zA-Z_-]+:' $(MAKEFILE_LIST) | \
	  awk 'BEGIN {FS = ": "}; {printf "  %-20s %s\n", $$1, $$2}' | \
	  sed 's/^  ## /  /'
