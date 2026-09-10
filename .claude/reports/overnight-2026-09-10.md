# Overnight Run Report — 2026-09-10

Ветка: `fix/audit-findings` от `master@3f264a9`.
PR (draft): https://github.com/ZetoOfficial/meal-planner/pull/55
Master во время работы не менялся (`origin/master` = `3f264a9` на старте и на финише).

## Выполненные группы и коммиты

| # | Коммит | ID | Что сделано |
|---|--------|-----|-------------|
| 1 | `40fb7a5` fix(planning): replace meal allocations instead of re-inserting them | B-01, T-01, T-02 | delete-then-insert allocations; slot_fill.id в ответе; порядок stale-флага; Postgres в CI; `make test-integration` на весь `./internal/...`; integration-тест + шаги quantity/swap в e2e |
| 2 | `7590de9` fix(reality): serve stored rule verdicts and stop hiding errors | B-02, B-05, B-06 | чтение `rule_evaluation_result` по едоку/basis/периоду; `Meal.PlanID`; три проглоченные ошибки |
| 3 | `ba6b9c8` refactor(planning): keep the search and the LLM out of the transaction | B-03 | solver и LLM вне транзакции; драфты и план — двумя короткими транзакциями |
| 4 | `c84ca64` fix(planning): feed the rolling-window history to the solver and swaps | B-04 | `EaterInput.History` и `swap.Input.History` из day summaries, actual поверх planned |
| 5 | `e126ae9` refactor(reality): narrow the reference ports and share the catalogue loader | A-01, A-02 | три узких порта вместо `CatalogueRepository`; `internal/usecase/reference` |
| 6 | `6395136` feat(http): add request middleware and fix four smaller defects | A-05, B-07, B-08, B-10, B-11 | recover/request-id/access-log; constant-time токен; CurrentGoal через Clock; 23503 на delete → 409; пересчёт actual в `LogMeal` |
| 7 | `73ebfd8` refactor: give the duplicated helpers one home each | A-03 | `quality.IsRollingRule`, `meal.DayOf`, `eater.Sex.OrUnspecified`, `pgvalue.NumericFields`/`Date`, `nutrition.CoverageFromScore` |
| 8 | `26e65d0` refactor: rename internal/errors to internal/apperr | A-06 | переименование каталога, алиасы импортов убраны |
| 9 | `abfcb40` refactor(api): name every inline schema and use double precision | A-04, B-09, B-13 | именованные схемы, `format: double`, `plan_usage`, удалён `openapi_values.go`, снято исключение revive |
| 10 | `9a57073` build: pin every tool in go.mod and tighten the linter | C-02, C-04, C-05 | tool-директивы + `go tool`; 7 новых линтеров и правки под них; dependabot |
| 11 | `f54b8df` build: move to Go 1.27 everywhere | C-03 | `go 1.27.0` + `toolchain go1.27.1`, `golang:1.27-alpine`, README |
| 12 | `d5d748c` chore: delete three files that no longer belong to the product | D-01, D-02, D-03 | `generate.go`, `cmd/kimi-issue-runner/`, `docs/github-roadmap.md` |
| 13 | `39c462b` docs: describe the structure that exists | D-04, D-05, D-06, D-07 | ARCHITECTURE §4/§12, README, SPEC §9, AGENTS.md; worktree и 9 веток удалены |

Все 39 находок из объёма закрыты. Ни одна группа не откатывалась.

## Пропущенные находки

**Вне объёма по решению владельца (не трогал):** B-12, B-14, B-15, A-07, A-08, A-09, T-03, C-01.

**Не воспроизвелось:** таких не было — все находки из объёма подтвердились по коду перед правкой. B-01 подтверждён падающим тестом (23505 → 409), B-02/B-05/B-06/B-03/B-04/B-07/B-08/B-10 — падающими тестами, остальные — чтением кода.

**Отклонения от формулировки задачи:**

1. **D-05 и `tests/e2e`.** Задача просила упомянуть `tests/e2e` в «Карте документации», но перенос e2e туда — это T-03, исключённый из объёма. Каталог `tests/` не создавался. В README указано фактическое расположение: `internal/api/http/e2e_integration_test.go` (integration build tag), рядом с адаптерными `internal/postgres/<capability>/integration_test.go`. Команда проверки `go test -tags=integration ./internal/... ./tests/...` запускалась как `./internal/...` — пакета `./tests/...` не существует.
2. **Два дефекта вне списка находок, исправленные в группе 1.** Без них шаги quantity и swap в e2e физически нельзя было написать: ответ плана не отдавал `slot_fill.id`, и apply-quantity возвращал `shopping_list_stale: false`. Оба описаны в теле коммита.
3. **Группа 10 подняла go-директиву до 1.26.0** как побочный эффект `go get -tool golangci-lint` (его модуль требует 1.26). Группа 11 довела до 1.27 и выровняла всё остальное.

## Статус проверок по группам

Полный набор перед каждым коммитом, все зелёные:

```
go build ./... && go vet ./... && go test ./... && make lint \
  && TEST_DATABASE_URL=... go test -tags=integration -count=1 ./internal/...
```

| Группа | build/vet | unit | lint | integration + e2e | make generate && git diff |
|--------|-----------|------|------|-------------------|---------------------------|
| 1 | ✅ | ✅ | ✅ | ✅ | — |
| 2 | ✅ | ✅ | ✅ | ✅ | — |
| 3 | ✅ | ✅ | ✅ | ✅ | — |
| 4 | ✅ | ✅ | ✅ | ✅ | — |
| 5 | ✅ | ✅ | ✅ | ✅ | — |
| 6 | ✅ | ✅ | ✅ | ✅ | — |
| 7 | ✅ | ✅ | ✅ | ✅ | — |
| 8 | ✅ | ✅ | ✅ | ✅ | — |
| 9 | ✅ | ✅ | ✅ | ✅ | ✅ без дрейфа |
| 10 | ✅ | ✅ | ✅ | ✅ | ✅ без дрейфа |
| 11 | ✅ | ✅ | ✅ | ✅ | ✅ без дрейфа |
| 12 | ✅ | ✅ | ✅ | ✅ | — |
| 13 | ✅ | ✅ | ✅ | ✅ | — |

Integration и e2e **запускались по-настоящему**: в `.env` уже был `TEST_DATABASE_URL`, контейнер `meal-planner-postgres-1` (postgres:18) работал, база `meal_planner_test` существует. Пропущенных проверок нет.

Дополнительно в группе 11: `docker build` на `golang:1.27-alpine` прошёл (образ после проверки удалён).

### Важно про линтер

Глобально установленный `golangci-lint` (v2.12.2, собран под go1.26.2) **падает** на Go 1.27: typecheck внутренних пакетов stdlib (`internal/randutil` → `math/rand/v2`) заканчивается паникой. Все прогоны шли через `make lint`, то есть через версию, закреплённую в репозитории и собираемую локальным тулчейном. После группы 10 это `go tool golangci-lint`. Если понадобится запускать линтер напрямую — надо переустановить бинарь под Go 1.27 (`go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2`).

Побочный эффект tool-директив: golangci-lint теперь собирается против дерева зависимостей **этого** модуля, а не своего собственного. Из-за MVS-подъёма `honnef.co/go/tools` и `golang.org/x/tools` staticcheck начал видеть SA5011 в `internal/solver/explain_test.go` (исправлено). Стоит держать в голове: обновление зависимостей приложения может менять поведение линтера.

## Что делать следующим

1. **Ревью PR #55 и мердж.** 13 коммитов, каждый самодостаточен и проверяем отдельно. Контрактные изменения (группа 9) стоит просмотреть особенно внимательно: `format: double` меняет тип каждого числового поля в SDK, `plan_usage` стал обязательным в `TopRecipe`.
2. **Проверить CI на PR.** Локально всё зелёное, но job `test` теперь поднимает `services: postgres:18` — это первый его запуск. Если integration-тесты в CI окажутся медленными, их можно вынести в отдельный job.
3. **C-01, оставшаяся часть.** Postgres в CI сделан (T-01). Не сделаны: `make generate && git diff --exit-code` и `go mod tidy && git diff --exit-code` как шаги CI, `govulncheck ./...`, `oasdiff breaking` на изменения `api/openapi/**`. Первые два дёшевы и стоят того — дрейф кодогенерации уже один раз случался (закоммиченный SDK был собран другим тулчейном Go).
4. **`go mod download` в Dockerfile теперь тянет и дерево инструментов** (golangci-lint со всеми анализаторами). Сборка образа проходит, но качает лишнее. Если время сборки станет заметным — стоит рассмотреть `go mod download github.com/...` только для нужных модулей или отдельный слой.
5. **Оставшиеся находки аудита**, если руки дойдут: B-12 (rolling-снапшот периода берётся без проверки `as_of_date`), B-14 (валидация `LLM_ENDPOINT`), B-15 (N+1 в List — записано как известное ограничение), A-07 (16 panic в мапперах), A-08 (зафиксировать в ARCHITECTURE, что HTTP осознанно импортирует solver и swap), A-09 (нужен ли отдельный модуль SDK), T-03 (перенос e2e в `tests/e2e`).
6. **Ветки на origin.** Локально удалены девять слитых веток. На origin остались `feat/16-quality-evaluator` и `feat/20-mvp-completion` — их я не трогал по условию задачи.
