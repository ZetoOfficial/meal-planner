# Meal Planner

Owner-only сервис планирования компонентной готовки под цели по БЖУ и
evidence-based качество рациона. Подробности — в `docs/` (`DESIGN.md`, `SPEC.md`,
`ARCHITECTURE.md`, `schema.sql`, `deep-research-report.md`) и контракт `api/openapi.yaml`.

Стек: Go (std `net/http`), Postgres, **sqlc** (type-safe queries), **oapi-codegen**
(OpenAPI-first), **tern** (миграции), Docker.

## Структура (по `docs/ARCHITECTURE.md`)

Модульный монолит, DDD-lite, ports-and-adapters. Зависимости смотрят внутрь:
`domain / solver / quality` не импортируют HTTP, OpenAPI, Postgres, sqlc, LLM SDK.

```
cmd/api/              точка входа (HTTP + graceful shutdown)
internal/app/         config + composition root (wiring)
internal/api/http/    delivery-адаптер: роутер net/http, auth, /healthz  (package httpapi)
internal/api/openapi/ сгенерированный oapi-codegen (generated.go)
internal/domain/      доменные модули: eater, catalog, recipe, planning,
                      nutrition, feedback, shopping (чистые типы и правила)
internal/usecase/     сценарии приложения + ports.go (интерфейсы на стороне потребителя)
internal/solver/      детерминированный планировщик (лексикографическая цель)
internal/quality/     evaluator quality-правил, rolling-7d, coverage
internal/llm/         обёртка над LLM (интерфейс + stub + recipe generator + prompts)
internal/postgres/    адаптер: пул, транзакции, sqlc/, repositories/
internal/clock/       абстракция времени
internal/errors/      доменные ошибки (package apperr)
migrations/           tern: tern.conf + 001_init.sql (зеркалит docs/schema.sql)
docs/                 ТЗ: DESIGN, SPEC, ARCHITECTURE, schema.sql, deep-research-report
api/                  openapi.yaml (контракт) + oapi-codegen.yaml (конфиг кодогена)
```

Большинство файлов в `domain/usecase/solver/quality` — пустышки-каркасы (package +
doc + минимальный placeholder), реализация добавляется по `docs/ARCHITECTURE.md §17`.

## Быстрый старт (Docker)

```bash
cp .env.example .env        # при желании поправить значения
make generate               # sqlc + oapi-codegen (нужен Go-тулчейн)
make up                     # postgres + tern-миграции + app на :8080
curl localhost:8080/healthz
```

`make up` поднимает Postgres, одноразово прогоняет миграции (сервис `migrate`),
затем стартует приложение.

## Локально (без Docker)

```bash
cp .env.example .env
make tidy
make generate
make migrate                # применить миграции к локальной БД из .env
make run
```

## Команды

```bash
make generate        # вся кодогенерация (sqlc + oapi)
make migrate         # применить миграции (tern)
make migrate-status  # статус миграций
make new-migration name=add_x
make build / run / test / lint
```

## Заметки

- Контракт `api/openapi.yaml` — источник истины для хендлеров/DTO. Сгенерированные
  хендлеры монтируются в `internal/api/http/server.go` (package `httpapi`, см. `New`).
- Схема для sqlc — `docs/schema.sql`; миграция `001_init.sql` её зеркалит.
  При изменении схемы: правим обе (или генерим schema.sql из миграций) и
  добавляем новую tern-миграцию.
- LLM сейчас — `llm.StubClient` (возвращает `ErrNotImplemented`). Реальная
  имплементация (net/http к `/v1/messages`) + кэш/retry/timeout — следующий шаг.
- Auth: статический `X-API-Token` (owner-only). Пустой `API_TOKEN` отключает проверку.
