.DEFAULT_GOAL := help

# Версии кодген-инструментов (воспроизводимая сборка).
SQLC        := go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1
OAPI        := go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.7.1
MOCKGEN     := go run go.uber.org/mock/mockgen@v0.6.0
GOLANGCI    := go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2
# tern: закрепи конкретную версию после `go list -m -versions github.com/jackc/tern/v2`
TERN        := go run github.com/jackc/tern/v2@latest

-include .env
export

.PHONY: help
help: ## Список целей
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-18s\033[0m %s\n", $$1, $$2}'

.PHONY: generate
generate: sqlc oapi ## Прогнать всю кодогенерацию

.PHONY: sqlc
sqlc: ## Сгенерировать type-safe DB-код (схема docs/schema.sql + запросы internal/postgres/sqlc/query)
	$(SQLC) generate

.PHONY: oapi
oapi: ## Сгенерировать HTTP-сервер/DTO из api/openapi.yaml
	$(OAPI) -config api/oapi-codegen.yaml api/openapi.yaml

.PHONY: mocks
mocks: ## Сгенерировать Uber mockgen-моки
	$(MOCKGEN) -source=internal/api/http/api.go -destination=internal/api/http/mocks/usecases.go -package=mocks
	$(MOCKGEN) -source=internal/usecase/catalog/ports.go -destination=internal/usecase/catalog/mocks/ports.go -package=mocks
	$(MOCKGEN) -source=internal/usecase/eater/ports.go -destination=internal/usecase/eater/mocks/ports.go -package=mocks
	$(MOCKGEN) -source=internal/postgres/tx.go -imports=pgx=github.com/jackc/pgx/v5 -destination=internal/postgres/mocks/transaction.go -package=mocks
	$(MOCKGEN) -destination=internal/postgres/mocks/pgx.go -package=mocks github.com/jackc/pgx/v5 Tx,Row
	$(MOCKGEN) -source=internal/postgres/sqlc/db.go -imports=pgx=github.com/jackc/pgx/v5,pgconn=github.com/jackc/pgx/v5/pgconn -destination=internal/postgres/mocks/dbtx.go -package=mocks

.PHONY: migrate
migrate: ## Применить tern-миграции (использует DB_* из .env)
	$(TERN) migrate --migrations ./migrations --config ./migrations/tern.conf

.PHONY: migrate-status
migrate-status: ## Показать статус миграций
	$(TERN) status --migrations ./migrations --config ./migrations/tern.conf

.PHONY: new-migration
new-migration: ## Новая миграция: make new-migration name=add_x
	$(TERN) new --migrations ./migrations $(name)

.PHONY: build
build: ## Собрать бинарь
	CGO_ENABLED=0 go build -o bin/server ./cmd/api

.PHONY: run
run: ## Запустить локально
	go run ./cmd/api

.PHONY: test-unit
test-unit: ## Запустить unit-тесты
	go test ./...

.PHONY: test-integration
test-integration: ## Запустить Postgres integration-тесты (нужен TEST_DATABASE_URL)
	go test -tags=integration -count=1 -v ./internal/postgres/...

.PHONY: test
test: test-unit ## Запустить unit-тесты и integration-тесты при TEST_DATABASE_URL
	@if [ -n "$(TEST_DATABASE_URL)" ]; then \
		$(MAKE) test-integration; \
	else \
		echo "integration tests skipped: set TEST_DATABASE_URL to run make test-integration"; \
	fi

.PHONY: lint
lint: ## golangci-lint (конфиг .golangci.yml)
	$(GOLANGCI) run --timeout 5m

.PHONY: tidy
tidy: ## go mod tidy
	go mod tidy

.PHONY: hooks
hooks: ## Подключить git-хуки из .githooks (валидация имени ветки на push)
	git config core.hooksPath .githooks
	chmod +x .githooks/*

.PHONY: up
up: ## docker compose up (postgres + миграции + app)
	docker compose up --build

.PHONY: down
down: ## Остановить стек
	docker compose down
