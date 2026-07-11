# Meal Planner

Owner-only HTTP API для планирования домашнего meal-prep на 2-3 дня для двух
человек. Цель: убрать ежедневное "что готовить", попадать в личные БЖУ и не
собирать macro-correct junk food за счет nutrition-quality правил.

## Что делает v1

- Ведет личную базу продуктов с реальными БЖУ конкретных брендов.
- Планирует еду под цели по БЖУ, аллергии/исключения, хранение 2-3 дня,
  оборудование и ротацию.
- Учитывает quality-слой: клетчатку, натрий/соль, насыщенные жиры, добавленные
  сахара, food-группы и rolling-7d паттерны.
- Генерирует список покупок из плана.
- Отдает аналитику по macro compliance, quality compliance и coverage качества
  данных.

Бизнесовый контекст и границы скоупа: `docs/DESIGN.md`.

## Статус

Уже заложено:

- OpenAPI контракт: `api/openapi.yaml`.
- DB contract: `docs/schema.sql` + начальная tern-миграция.
- Генерация DTO/handlers через `oapi-codegen`.
- Генерация type-safe DB-кода через `sqlc`.
- HTTP-сервер на std `net/http`, `/livez` + `/readyz`, graceful shutdown.
- Postgres pool, config loading, статический `X-API-Token`.
- Opt-in Postgres integration-test harness с изолированной schema и миграциями.
- Структура domain/usecase/solver/quality/llm под DDD-lite и ports/adapters.

Пока в основном каркас / следующий слой реализации:

- use case orchestration;
- solver;
- quality evaluator;
- реальные HTTP-хендлеры поверх OpenAPI;
- реальный LLM client вместо stub;
- полноценные repositories и бизнесовые integration-сценарии.

## Быстрый старт

Требования:

- Go `1.26` (см. `go.mod`);
- Docker + Docker Compose;
- Make;
- Postgres локально нужен только для запуска без Docker.

Docker-first запуск:

```bash
cp .env.example .env
make generate
make up
curl localhost:8080/readyz
```

`make up` поднимает Postgres, одноразово прогоняет миграции через сервис
`migrate`, затем стартует API на `:8080`.

Локальный запуск без Docker:

```bash
cp .env.example .env
make tidy
make generate
make migrate
make run
```

## Конфиг

`.env.example` можно копировать в `.env`. `.env` не коммитится.

| Переменная | Что значит |
|---|---|
| `HTTP_ADDR` | Адрес HTTP-сервера, по умолчанию `:8080`. |
| `API_TOKEN` | Статический owner-only токен для `X-API-Token`. Пустое значение отключает auth. |
| `DATABASE_URL` | Полный Postgres DSN. Имеет приоритет над `DB_*`. |
| `DB_HOST`, `DB_PORT`, `DB_NAME`, `DB_USER`, `DB_PASSWORD`, `DB_SSLMODE` | Альтернатива `DATABASE_URL`. |
| `TEST_DATABASE_URL` | Opt-in URL Postgres для integration-тестов. Каждый тест создаёт отдельную schema, применяет миграции и удаляет schema при cleanup; указывайте выделенную тестовую БД, а не development database. |
| `LLM_PROVIDER` | Провайдер LLM, сейчас конфигурируется как `anthropic` / `openai`. |
| `LLM_MODEL` | Имя модели. |
| `LLM_API_KEY` | API key провайдера. Сейчас реальный client еще не подключен. |

## API

- Контракт: `api/openapi.yaml`.
- Base URL для локального запуска: `http://localhost:8080`.
- Probes без авторизации: `GET /livez` and `GET /readyz`.
- Остальной API защищается заголовком `X-API-Token`, если `API_TOKEN` не пустой.

Проверка healthcheck:

```bash
curl localhost:8080/livez
curl localhost:8080/readyz
```

Пример запроса к защищенному API после подключения хендлеров:

```bash
curl -H "X-API-Token: $API_TOKEN" localhost:8080/...
```

## Команды

```bash
make generate                 # sqlc + oapi-codegen
make migrate                  # применить tern-миграции
make migrate-status           # статус миграций
make new-migration name=add_x # создать новую миграцию
make build                    # собрать bin/server
make run                      # запустить ./cmd/api
make test-unit                # запустить только unit-тесты
make test-integration         # integration-тесты Postgres (нужен TEST_DATABASE_URL)
make test                     # unit-тесты + integration при заданном TEST_DATABASE_URL
make lint                     # golangci-lint run
make up                       # docker compose up --build
make down                     # docker compose down
```

Кодогенерация использует версии инструментов из `Makefile`.

`make test` всегда запускает unit-тесты. Если `TEST_DATABASE_URL` не задан,
после них он печатает `integration tests skipped: set TEST_DATABASE_URL to run
make test-integration`. Для integration-тестов задайте URL выделенной тестовой
БД: harness создаёт в ней уникальную schema, применяет только up-части всех
миграций и удаляет schema после теста.

## Архитектурные правила

- HTTP, OpenAPI, Postgres, sqlc и LLM providers - адаптеры.
- Solver, quality rules, plan generation, recipe slots и macro logic - core.
- Core packages не импортируют HTTP handlers, OpenAPI DTO, sqlc rows, Postgres
  repositories или provider SDKs.
- LLM только генерирует recipe candidates. Go валидирует их, solver решает
  использовать ли их, quality evaluator объясняет результат.
- Интерфейсы лежат на стороне потребителя, обычно в `usecase/*/ports.go`.
- `docs/schema.sql` и `api/openapi.yaml` - source of truth. Не добавлять поля
  и endpoints мимо них.
- v1 scope - HTTP API only. Telegram bot, Mini App, voice, vision/barcode,
  FatSecret sync, inventory, RAG, agents и fine-tuning - v2+ / out of scope.

## Структура

```text
cmd/api/              entrypoint HTTP API
internal/app/         config + composition root
internal/api/http/    net/http adapter, auth, /livez, /readyz
internal/api/openapi/ generated oapi-codegen code
internal/domain/      чистые доменные типы и правила
internal/usecase/     application scenarios + consumer-side ports
internal/solver/      deterministic planner
internal/quality/     quality rules, rolling-7d, coverage, explanations
internal/llm/         LLM interface, stub, recipe generation prompts
internal/postgres/    Postgres adapter, sqlc, repositories
internal/clock/       time abstraction
internal/errors/      application/domain errors
migrations/           tern migrations
docs/                 design/spec/schema/architecture docs
api/                  OpenAPI contract + codegen config
```

## Документы

- `docs/DESIGN.md` - бизнесовая задача, ограничения, MVP/v2 границы.
- `docs/SPEC.md` - продуктовая/поведенческая спецификация.
- `docs/ARCHITECTURE.md` - модульная архитектура и dependency rule.
- `docs/schema.sql` - DB contract для Postgres/sqlc.
- `docs/deep-research-report.md` - обоснование nutrition-quality слоя.
- `api/openapi.yaml` - HTTP contract.

## Рабочие заметки

- Схема для sqlc - `docs/schema.sql`; миграция `migrations/001_init.sql` ее
  зеркалит. При изменении схемы нужно обновлять contract и добавлять новую
  tern-миграцию.
- Сгенерированный OpenAPI код лежит в `internal/api/openapi/generated.go`.
- Сгенерированный sqlc код лежит в `internal/postgres/sqlc/`.
- `LLM` сейчас stub. Реальная интеграция должна идти через тонкий HTTP adapter,
  с timeout/retry/cache/logging, а не через solver/domain.
