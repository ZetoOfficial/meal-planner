.DEFAULT_GOAL := help

SQLC := go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1
OAPI := go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.7.1
TERN := go run github.com/jackc/tern/v2@latest

-include .env
export

.PHONY: help
help: ## List targets
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-18s\033[0m %s\n", $$1, $$2}'

.PHONY: generate
generate: sqlc oapi ## Run all code generation

.PHONY: sqlc
sqlc: ## Generate type-safe DB code
	$(SQLC) generate

.PHONY: oapi
oapi: ## Generate OpenAPI server and DTO code
	$(OAPI) -config api/oapi-codegen.yaml api/openapi.yaml

.PHONY: migrate
migrate: ## Apply tern migrations using DB_* from .env
	$(TERN) migrate --migrations ./migrations --config ./migrations/tern.conf

.PHONY: migrate-status
migrate-status: ## Show migration status
	$(TERN) status --migrations ./migrations --config ./migrations/tern.conf

.PHONY: new-migration
new-migration: ## Create a migration: make new-migration name=add_x
	$(TERN) new --migrations ./migrations $(name)

.PHONY: build
build: ## Build API binary
	CGO_ENABLED=0 go build -o bin/server ./cmd/api

.PHONY: run
run: ## Run API locally
	go run ./cmd/api

.PHONY: test
test: test-unit ## Unit tests + integration-test status
	@if [ -z "$$TEST_DATABASE_URL" ]; then \
		echo "integration tests skipped: set TEST_DATABASE_URL to run make test-integration"; \
	else \
		$(MAKE) test-integration; \
	fi

.PHONY: test-unit
test-unit: ## Unit tests
	go test ./...

.PHONY: test-integration
test-integration: ## Postgres integration tests (requires TEST_DATABASE_URL)
	go test -tags=integration -count=1 -v ./internal/postgres/...

.PHONY: test-db-up
test-db-up: ## Start local Postgres for integration tests
	docker compose up -d postgres

.PHONY: lint
lint: ## Run golangci-lint
	golangci-lint run

.PHONY: tidy
tidy: ## Run go mod tidy
	go mod tidy

.PHONY: hooks
hooks: ## Подключить git-хуки из .githooks (валидация имени ветки на push)
	git config core.hooksPath .githooks
	chmod +x .githooks/*

.PHONY: up
up: ## Start local stack
	docker compose up --build

.PHONY: down
down: ## Stop local stack
	docker compose down
