# Meal Planner — Architecture

> Статус: архитектурное решение для MVP v1.
> Связанные документы: `docs/DESIGN.md`, `docs/SPEC.md`, `docs/schema.sql`, `api/openapi.yaml`.

---

## 1. Решение в одну строку

Для проекта используется **модульный монолит** с **DDD-lite** внутри сложных доменных областей и **hexagonal / ports-and-adapters** на внешних границах.

Это не микросервисы, не enterprise Clean Architecture на каждый CRUD-эндпоинт и не `handlers/services/repositories` как единственная структура. Clean Architecture здесь используется как правило направления зависимостей:

> HTTP, OpenAPI, Postgres, sqlc и LLM — снаружи.  
> Solver, quality rules, recipe slots, plan generation и macro logic — внутри.  
> Зависимости смотрят внутрь.

---

## 2. Почему именно так

Проект выглядит маленьким по deployment-модели: owner-only, два едока, один Go-сервис, HTTP API, Postgres. Но внутри это не простой CRUD.

Ключевая сложность живёт в домене:

- рецепт — это шаблон со слотами, а не фиксированный список ингредиентов;
- слот заполняется конкретным продуктом из личной БД, и БЖУ пересчитываются на лету;
- планировщик должен учитывать hard constraints, макро-цели, storage profile, оборудование, ротацию, fixed meals;
- nutrition quality — отдельный слой поверх БЖУ, с soft rules, warnings, coverage и rolling-7d;
- свопы имеют blast radius и могут ломать prep dependency graph;
- LLM в MVP — только внешний генератор recipe candidates, а не агент и не главный decision maker.

Поэтому архитектура должна защищать доменное ядро от инфраструктуры, но не должна раздувать MVP лишней церемонией.

---

## 3. Высокоуровневая схема

```text
HTTP / OpenAPI
    ↓
Use cases
    ↓
Domain + Solver + Quality
    ↓
Ports/interfaces
    ↑
Postgres/sqlc adapters
LLM adapter
```

Главное правило:

```text
domain, solver, quality
  НЕ импортируют http, openapi, postgres, sqlc, anthropic/openai SDK
```

---

## 4. Рекомендуемая структура проекта

```text
cmd/
  api/
    main.go

internal/
  app/
    app.go
    config.go

  api/
    openapi/
      generated.go
    http/
      server.go
      eater_handlers.go
      eater_mapping.go
      catalog_handlers.go
      catalog_mapping.go
      recipe_handlers.go
      recipe_mapping.go
      planning_handlers.go
      planning_mapping.go
      analytics_handlers.go
      analytics_mapping.go

  domain/
    eater/
      eater.go
      macro_goal.go
      fixed_meal.go
    catalog/
      product.go
      role_tag.go
      allergen.go
      nutrition_facts.go
    recipe/
      template.go
      slot.go
      storage.go
    planning/
      plan.go
      meal.go
      slot_fill.go
      swap.go
      prep_dependency.go
    nutrition/
      macros.go
      summary.go
      rule_result.go
      coverage.go
    feedback/
      feedback.go
    shopping/
      shopping_list.go

  usecase/
    eater/
      service.go
      ports.go
    catalog/
      service.go
      ports.go
    recipe/
      service.go
      ports.go
    planning/
      generate_plan.go
      preview_swap.go
      apply_swap.go
      ports.go
    meal_log/
      log_meal.go
      ports.go
    feedback/
      create_feedback.go
      ports.go
    analytics/
      get_macros.go
      get_quality.go
      top_recipes.go
      ports.go

  solver/
    planner.go
    candidates.go
    constraints.go
    objective.go
    result.go
    explain.go

  quality/
    ruleset.go
    evaluator.go
    rolling.go
    coverage.go
    explain.go

  llm/
    client.go
    recipe_generator.go
    prompt.go
    prompts/
      recipe_generation.md

  postgres/
    db.go
    store.go
    tx.go
    sqlc/
      generated.go
    pgvalue/
      numeric.go
    dberror/
      error.go
    catalog/
      repository.go
      mapping.go
    eater/
      repository.go
      mapping.go
    recipe/
      repository.go
      mapping.go
    planning/
      repository.go
      mapping.go
    analytics/
      repository.go
      mapping.go
    llm/
      repository.go
      mapping.go

  clock/
  errors/
```

Эта структура не обязана появиться вся в первый день. Если папка пустая или содержит один файл «на будущее», её лучше пока не создавать.

---

## 5. Доменные модули

### 5.1 `catalog`

Личная база продуктов, роли, аллергены, food classification, nutrition facts.

Типичные сущности:

```text
Product
RoleTag
ProductRole
Allergen
FoodClassification
NutritionFacts
```

`catalog` не должен знать про планировщик. Он отвечает за то, что продукт корректно описан и может быть использован как fill для слота.

---

### 5.2 `recipe`

Шаблоны рецептов и слоты.

Типичные сущности:

```text
RecipeTemplate
RecipeSlot
RecipeStatus
StorageProfile
Equipment
Cuisine
CookingMethod
```

Основные правила:

- рецепт состоит из слотов;
- слот требует `role_tag`;
- заполнение слота конкретным продуктом происходит уже в плане;
- `draft` рецепт после готовки и одобрения может стать `permanent`;
- только `permanent` участвует в reuse-first без LLM.

---

### 5.3 `eater`

Профили едоков, макро-цели, аллергены, fixed meals, quality profile.

Типичные сущности:

```text
Eater
MacroGoal
MacroRange
FixedMeal
QualityProfile
```

Важно: **macro goals и quality rules не смешиваются**.

- macro goals — личные цели пользователя;
- quality rules — доказательные правила качества рациона;
- macro goals версионируются;
- fixed meals не генерируются и не свопаются, но учитываются в БЖУ, quality analytics и warnings.

---

### 5.4 `planning`

Центральный доменный модуль.

Типичные сущности:

```text
Plan
PlanRecipe
SlotFill
Meal
Portion
SwapPreview
PrepDependency
```

Здесь живут:

- генерация плана;
- раскладка по датам, приёмам пищи и едокам;
- planned vs actual на уровне meal;
- preview swap;
- apply swap;
- blast radius;
- prep dependency graph.

---

### 5.5 `nutrition`

Общие типы для БЖУ, нутриентов, агрегатов, coverage и результатов правил.

Типичные сущности:

```text
Macros
MacrosPer100g
NutritionFacts
NutritionSummary
Coverage
RuleEvaluationResult
```

Этот модуль содержит типы и простые операции: сложение БЖУ, проверку диапазонов, конвертацию соли в натрий, расчёт kcal из macro values и т.п.

---

### 5.6 `quality`

`quality` — это не просто CRUD-модуль, а deterministic rule evaluator.

Он отвечает за:

- evaluation quality rules;
- rolling-7d правила;
- coverage / uncertainty handling;
- quality score;
- explainability для warnings;
- статусы `pass`, `soft_fail`, `warn`, `unknown`.

Правила в MVP живут в коде/конфиге, а не в generic guideline engine. В БД хранятся profile binding, overrides, summaries и evaluation results.

---

### 5.7 `solver`

Solver — алгоритмический сервис, который строит план из кандидатов.

Он должен зависеть от доменных типов, но не от инфраструктуры.

```text
solver → domain/eater
solver → domain/catalog
solver → domain/recipe
solver → domain/planning
solver → domain/nutrition
```

Solver не должен импортировать:

```text
api/openapi
api/http
postgres
postgres/sqlc
llm/provider-specific packages
```

Целевая функция — лексикографическая:

```text
1. hard safety / feasibility
2. macro feasibility
3. strongest quality rules
4. rolling weekly pattern
5. cost / reuse / storage / novelty
6. coverage
```

---

### 5.8 `shopping`

Список покупок — производная от плана.

Он агрегирует `slot_fill` по продукту, считает количество и стоимость. После свопа список должен уметь перегенерироваться без потери отметок по неизменившимся позициям.

---

### 5.9 `feedback`

Feedback меняет вес/вероятность появления рецепта или продукта, а не банит.

В MVP комментарий хранится как текст. LLM-парсинг feedback — v2.

---

## 6. Use cases

Use case — это сценарий приложения, а не «сервис со всем подряд».

Примеры use cases:

```text
CreateProduct
UpdateProduct
CreateEater
CreateMacroGoalVersion
GeneratePlan
PreviewSwap
ApplySwap
BuildShoppingList
LogMeal
CreateFeedback
PromoteRecipe
GetMacroAnalytics
GetNutritionQualityAnalytics
GetTopRecipes
```

Use case может:

- открыть транзакцию;
- прочитать данные через repository ports;
- вызвать solver;
- вызвать quality evaluator;
- вызвать LLM recipe generator при необходимости;
- сохранить результат;
- вернуть application-level result.

Use case не должен превращаться в огромный файл с математикой solver-а или правилами quality.

---

## 7. Ports and adapters

Интерфейсы нужны на внешних границах и размещаются на стороне потребителя, обычно в `usecase/*/ports.go`.

Пример:

```go
type ProductRepository interface {
    ListUsableProducts(ctx context.Context) ([]catalog.Product, error)
    Get(ctx context.Context, id catalog.ProductID) (catalog.Product, error)
    Save(ctx context.Context, p catalog.Product) error
}

type RecipeRepository interface {
    ListPermanentCandidates(ctx context.Context, q RecipeQuery) ([]recipe.Template, error)
    SaveDraft(ctx context.Context, r recipe.Template) error
    Promote(ctx context.Context, id recipe.ID) error
}

type PlanRepository interface {
    Save(ctx context.Context, p planning.Plan) error
    Get(ctx context.Context, id planning.PlanID) (planning.Plan, error)
}

type RecipeGenerator interface {
    Generate(ctx context.Context, input RecipeGenerationInput) ([]recipe.TemplateDraft, error)
}
```

Postgres/sqlc реализует repository ports.

LLM adapter реализует `RecipeGenerator`.

---

## 8. Граница LLM

LLM не строит финальный план.

Правильный pipeline:

```text
LLM → recipe candidates → Go validator → solver → plan → quality evaluation
```

Неправильно:

```text
LLM → final plan
```

LLM может вернуть только структурированный draft-кандидат. После этого Go-код обязан:

1. проверить, что все продукты существуют в БД;
2. проверить роли слотов;
3. проверить storage profile и equipment;
4. посчитать БЖУ самостоятельно;
5. отбраковать невалидное;
6. залогировать prompt/response/error/tokens/latency/model/temperature;
7. сохранить валидный draft только после валидации.

---

## 9. HTTP / OpenAPI layer

`api/http` — delivery adapter.

Он отвечает за:

- routing;
- auth middleware для owner-only токена;
- decode request;
- validate request на уровне OpenAPI;
- mapping OpenAPI DTO → usecase command;
- mapping usecase result → OpenAPI DTO;
- error mapping.

В handlers нельзя класть:

- solver logic;
- SQL;
- quality rule evaluation;
- LLM prompts;
- business decisions.

---

## 10. Postgres / sqlc layer

`postgres` — infrastructure adapter.

Он отвечает за:

- подключение к БД;
- транзакции;
- sqlc queries;
- mapping sqlc rows → domain objects;
- сохранение domain objects в таблицы.

sqlc-типы не должны протекать в domain/usecase/solver/quality.

Допустимо:

```text
postgres/catalog/repository.go imports postgres/sqlc and domain/catalog
```

Недопустимо:

```text
domain/catalog imports postgres/sqlc
usecase/planning exposes sqlc.Product row
solver accepts openapi.PlanGenerateRequest
```

---

## 11. Транзакции

Транзакционная граница находится в use case layer.

Примерно:

```go
func (uc *GeneratePlan) Handle(ctx context.Context, cmd Command) (planning.Plan, error) {
    return uc.tx.WithTx(ctx, func(ctx context.Context) (planning.Plan, error) {
        eaters, err := uc.eaters.GetMany(ctx, cmd.EaterIDs)
        if err != nil {
            return planning.Plan{}, err
        }

        products, err := uc.products.ListUsable(ctx)
        if err != nil {
            return planning.Plan{}, err
        }

        recipes, err := uc.recipes.ListPermanentCandidates(ctx)
        if err != nil {
            return planning.Plan{}, err
        }

        candidates := recipes

        if uc.needLLM(candidates, cmd.NoveltyLevel) {
            drafts, err := uc.recipeGenerator.Generate(ctx, RecipeGenerationInput{...})
            if err != nil {
                return planning.Plan{}, err
            }

            validated := uc.recipeValidator.ValidDrafts(drafts, products)
            candidates = append(candidates, validated...)
        }

        result, err := uc.solver.BuildPlan(ctx, solver.Input{
            Eaters:   eaters,
            Products: products,
            Recipes:  candidates,
            Dates:    cmd.Dates,
            Novelty:  cmd.NoveltyLevel,
        })
        if err != nil {
            return planning.Plan{}, err
        }

        quality := uc.qualityEvaluator.EvaluatePlan(ctx, result.Plan)
        result.Plan.AttachQuality(quality)

        if err := uc.plans.Save(ctx, result.Plan); err != nil {
            return planning.Plan{}, err
        }

        return result.Plan, nil
    })
}
```

---

## 12. Mapping endpoints to modules

```text
/eaters
/eaters/{eaterId}
/eaters/{eaterId}/goals
/eaters/{eaterId}/fixed-meals
  → usecase/eater

/products
/products/{productId}
/role-tags
  → usecase/catalog

/recipes
/recipes/{recipeId}
/recipes/{recipeId}/promote
  → usecase/recipe

/plans
/plans/{planId}
/plans/{planId}/swap/preview
/plans/{planId}/swap
  → usecase/planning

/plans/{planId}/shopping-list
/shopping-list-items/{itemId}
  → usecase/shopping

/plans/{planId}/meals
/meals/{mealId}/log
  → usecase/meal_log

/feedback
  → usecase/feedback

/analytics/macros
/analytics/nutrition-quality
/analytics/top-recipes
  → usecase/analytics
```

---

## 13. Read models and analytics

Полный CQRS, event sourcing и брокеры не нужны.

Но read models для аналитики уместны:

```text
day_nutrition_summary
rolling_nutrition_summary
rule_evaluation_result
```

Идея простая:

- write-side хранит планы, meals, slot fills, feedback;
- read-side хранит агрегаты и результаты правил;
- пересчёт агрегатов может быть синхронным в MVP;
- если потом появятся тяжёлые пересчёты, их можно вынести в background job, не меняя доменную модель.

---

## 14. Когда создавать новый пакет

Новый пакет оправдан, если выполняется хотя бы одно:

- есть отдельный язык предметной области;
- есть отдельные инварианты;
- есть несколько файлов, которые меняются вместе;
- пакет можно протестировать без HTTP и Postgres;
- пакет уменьшает количество импортов, а не увеличивает хаос.

Новый пакет не нужен, если:

- в нём будет один файл «на будущее»;
- это `util`, `common`, `types`, `interfaces` без понятной доменной ответственности;
- пакет создаётся только потому, что так выглядит Clean Architecture на картинке.

---

## 15. Что не делать

Не превращать проект в:

```text
internal/controllers
internal/services
internal/repositories
internal/entities
internal/dto
internal/interfaces
```

Такая структура кажется простой, но быстро размазывает доменные связи по техническим папкам.

Не делать:

- микросервисы;
- generic framework для rules engine;
- RAG/vector DB;
- agentic framework;
- fine-tuning;
- LLM-планировщик вместо deterministic solver;
- универсальный `common` пакет;
- интерфейсы для каждой реализации «на всякий случай»;
- протекание OpenAPI DTO или sqlc rows в domain/solver/quality.

---

## 16. Тестовая стратегия

Минимальный набор тестов:

```text
Domain tests
  - macros arithmetic
  - macro goal ranges
  - role compatibility
  - allergen checks
  - storage profile checks

Quality tests
  - sodium / salt conversion
  - unknown != 0
  - coverage penalties
  - rolling-7d meat/legume rules
  - macro-pass + quality-fail returns warning, not hard block

Solver tests
  - feasible plan generation
  - impossible constraints return diagnostic 422-level error
  - fixed meals included in macros and quality
  - novelty level affects candidate selection

Swap tests
  - fill→fill preview
  - protein→protein preview
  - recipe→recipe prep dependency invalidation
  - shopping list dirty/regeneration behavior

Adapter tests
  - postgres repository mapping
  - OpenAPI handler mapping
  - LLM invalid product rejected by validator
```

Solver и quality должны хорошо тестироваться без Postgres, HTTP и LLM.

---

## 17. Практический порядок реализации

1. Поднять `cmd/api` как composition root: загрузить config из `internal/app`,
   подключить Postgres и запустить migrations.
2. Сгенерировать OpenAPI handlers/DTO и sqlc queries.
3. Реализовать `domain/nutrition`, `domain/catalog`, `domain/eater`.
4. Реализовать CRUD products/eaters/goals/fixed meals.
5. Реализовать `domain/recipe` и recipe CRUD/promote.
6. Реализовать минимальный deterministic solver без LLM.
7. Реализовать quality evaluator и coverage.
8. Реализовать `GeneratePlan` reuse-first.
9. Добавить LLM recipe generator только как fallback/source of draft candidates.
10. Добавить swap preview/apply и prep dependency graph.
11. Добавить shopping list.
12. Добавить meal logging, feedback, analytics.

---

## 18. Короткое правило для будущих изменений

Если появляется новая фича, сначала определить:

```text
Это domain logic?
  → domain / solver / quality

Это пользовательский сценарий?
  → usecase

Это HTTP contract?
  → api/http + api/openapi mapper

Это SQL/storage?
  → postgres/sqlc adapter

Это внешний сервис?
  → adapter behind a port

Это аналитический отчёт?
  → read model / analytics usecase
```

Если непонятно, куда положить код, скорее всего надо уточнить границу ответственности, а не создавать `common`.
