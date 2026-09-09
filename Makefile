.DEFAULT_GOAL := help

# Инструменты закреплены tool-директивами в go.mod: одна версия на репозиторий,
# одинаковая локально, в CI и в docker compose.
SQLC        := go tool sqlc
OAPI        := go tool oapi-codegen
GOLANGCI    := go tool golangci-lint
TERN        := go tool tern

-include .env
export

.PHONY: help
help: ## Список целей
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-18s\033[0m %s\n", $$1, $$2}'

.PHONY: generate
generate: sqlc oapi ## Прогнать всю кодогенерацию
	$(MAKE) mocks

.PHONY: sqlc
sqlc: ## Сгенерировать type-safe DB-код (схема docs/schema.sql + запросы internal/postgres/sqlc/query)
	$(SQLC) generate

.PHONY: oapi
oapi: ## Сгенерировать SDK-типы и HTTP-сервер/DTO из api/openapi/{model,api}/api.yaml
	$(OAPI) -config api/cfg.openapi.model.yaml api/openapi/model/api.yaml
	$(OAPI) -config api/cfg.openapi.api.yaml api/openapi/api/api.yaml

.PHONY: mocks
mocks: ## Сгенерировать GoMock-моки из go:generate-директив
	go generate ./...

.PHONY: migrate
migrate: ## Применить tern-миграции (использует DB_* из .env)
	$(TERN) migrate --migrations ./migrations --config ./migrations/tern.conf

.PHONY: seed
seed: ## Загрузить локальные демо-данные (только для разработки)
	psql "$$DATABASE_URL" -v ON_ERROR_STOP=1 -f migrations/seed/dev_seed.sql

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
test-integration: ## Запустить integration- и e2e-тесты (нужен TEST_DATABASE_URL)
	go test -tags=integration -count=1 ./internal/...

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
