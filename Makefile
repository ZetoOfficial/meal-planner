.DEFAULT_GOAL := help

# Версии кодген-инструментов (воспроизводимая сборка).
SQLC        := go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1
OAPI        := go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.7.1
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

.PHONY: test
test: ## Тесты
	go test ./...

.PHONY: lint
lint: ## golangci-lint (конфиг .golangci.yml)
	golangci-lint run

.PHONY: tidy
tidy: ## go mod tidy
	go mod tidy

.PHONY: up
up: ## docker compose up (postgres + миграции + app)
	docker compose up --build

.PHONY: down
down: ## Остановить стек
	docker compose down
