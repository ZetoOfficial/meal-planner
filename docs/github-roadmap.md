# GitHub Roadmap

Backlog for turning the current scaffold into the MVP described in `docs/SPEC.md`,
`docs/ARCHITECTURE.md`, `docs/schema.sql`, and `api/openapi/api/api.yaml`.

The intended issue size is 0.5-2 focused engineering days. Issues are grouped by
capability milestones, not by individual files or endpoints, so each closed issue
should leave the system more runnable or more useful.

## Issue Template

```md
## Context
- Docs and contract references.

## Scope
What this issue implements.

## Acceptance Criteria
- Observable behavior.
- Persistence/API expectations.
- Test expectations.

## Out of Scope
What must not be pulled into this issue.
```

## Labels

- `type:feature`
- `type:test`
- `type:infra`
- `type:docs`
- `area:api`
- `area:db`
- `area:domain`
- `area:catalog`
- `area:eater`
- `area:recipe`
- `area:planning`
- `area:quality`
- `area:solver`
- `area:llm`
- `area:shopping`
- `area:meals`
- `area:feedback`
- `area:analytics`
- `priority:p0`
- `priority:p1`
- `priority:p2`

## Milestone M1: Runnable API Foundation

Goal: make the scaffold a real runnable API shell with generated OpenAPI wiring,
consistent errors, repository plumbing, and repeatable local verification.

### Issue 1: Wire generated OpenAPI handlers into the HTTP server

Labels: `type:feature`, `area:api`, `priority:p0`

Context:
- `api/openapi/api/api.yaml`
- `internal/api/openapi/generated.go`
- `internal/api/http/server.go`
- `docs/ARCHITECTURE.md` sections 9 and 12

Scope:
- Mount the generated oapi-codegen server into `internal/api/http`.
- Introduce a concrete API implementation type with method stubs returning
  structured not-implemented responses for endpoints that are not implemented yet.
- Keep `/healthz` unauthenticated and all API routes behind `X-API-Token`.

Acceptance Criteria:
- `GET /healthz` still works without an API token.
- OpenAPI routes are reachable and protected by auth when `API_TOKEN` is set.
- Unimplemented endpoints return a consistent JSON error, not router 404s.
- No handler contains SQL, solver logic, quality logic, or LLM prompts.
- `go test ./...` passes.

Out of Scope:
- Implementing business behavior for the endpoints.
- Changing the OpenAPI contract except for defects found during wiring.

### Issue 2: Implement shared application errors and HTTP error mapping

Labels: `type:feature`, `area:api`, `area:domain`, `priority:p0`

Context:
- `docs/SPEC.md` section 6
- `docs/ARCHITECTURE.md` section 9
- `internal/errors/errors.go`

Scope:
- Define application error classes for validation, not found, conflict,
  unauthorized, unavailable, and impossible plan diagnostics.
- Add HTTP mapping to OpenAPI-compatible error responses.
- Ensure use cases can return errors without importing HTTP or OpenAPI DTOs.

Acceptance Criteria:
- Validation failures map to 422.
- Missing resources map to 404.
- LLM/provider unavailability can map to 503.
- Auth failures still map to 401.
- Tests cover at least one mapping for each error class.

Out of Scope:
- Full validation implementation for every domain object.
- Solver diagnostic generation.

### Issue 3: Finish Postgres transaction and repository foundation

Labels: `type:infra`, `area:db`, `priority:p0`

Context:
- `docs/ARCHITECTURE.md` sections 7, 10, and 11
- `internal/postgres/db.go`
- `internal/postgres/tx.go`
- `internal/postgres/repositories/*`
- `internal/postgres/sqlc/*`

Scope:
- Implement a transaction runner that allows use cases to execute repository
  operations inside a single transaction.
- Establish repository construction in the composition root.
- Keep sqlc types inside the Postgres adapter.

Acceptance Criteria:
- Use cases can depend on repository interfaces, not sqlc queries.
- Repositories can be constructed with pool-backed and transaction-backed query executors.
- A transaction rolls back when the callback returns an error.
- Tests cover commit and rollback behavior.

Out of Scope:
- Implementing every repository method.
- Introducing a generic repository framework.

### Issue 4: Add repeatable integration test harness

Labels: `type:test`, `area:db`, `area:api`, `priority:p0`

Context:
- `Makefile`
- `docker-compose.yml`
- `migrations/001_init.sql`
- `docs/ARCHITECTURE.md` section 16

Scope:
- Add a practical integration test setup for Postgres-backed repository and HTTP tests.
- Document the required environment variables and commands.
- Keep the harness simple enough for local owner-only development.

Acceptance Criteria:
- Integration tests can create an isolated database schema or database.
- Migrations are applied before tests that need the database.
- Tests can be skipped with a clear message when no test database is configured.
- `make test` or a documented equivalent runs unit tests and reports integration-test status.

Out of Scope:
- Full end-to-end product flow testing.
- CI provider configuration unless it is already present.

## Milestone M2: Catalog and Eaters CRUD

Goal: make the user-editable seed data real: eaters, macro goals, fixed meals,
exclusions, products, allergens, role tags, and quality fields.

### Issue 5: Implement product CRUD with roles, allergens, and quality fields

Labels: `type:feature`, `area:catalog`, `area:api`, `area:db`, `priority:p0`

Context:
- `docs/SPEC.md` FR-1
- `docs/schema.sql`: `product`, `product_food_classification`,
  `product_role`, `product_allergen`
- `api/openapi/api/api.yaml`: `/products`, `/products/{productId}`

Scope:
- Implement create, list, get, update, and delete for products.
- Persist macros, price, unit, allergens, role assignments, food classification,
  optional quality nutrients, and data-quality metadata.
- Preserve the rule that unknown quality fields are NULL, not zero.

Acceptance Criteria:
- Products round-trip through API and Postgres with all OpenAPI fields supported.
- Product role assignments are persisted and returned.
- Product allergens are persisted and returned.
- Optional quality fields can be absent without being stored as zero.
- Repository and handler tests cover create/list/get/update/delete and validation errors.

Out of Scope:
- Recipe slot validation.
- Solver candidate selection.

### Issue 6: Implement role tag listing and seed role taxonomy

Labels: `type:feature`, `area:catalog`, `area:db`, `priority:p0`

Context:
- `docs/SPEC.md` section 4
- `docs/schema.sql`: `role_tag`
- `api/openapi/api/api.yaml`: `/role-tags`

Scope:
- Seed the initial role taxonomy needed by MVP recipes and products.
- Implement role tag listing.
- Preserve hierarchical codes such as `protein:lean:breast`.

Acceptance Criteria:
- Migrations or seed logic create the starter role tags needed for MVP examples.
- `GET /role-tags` returns stable role ids, codes, names, kind, and parent references.
- Role kind and role code validation are enforced when assigning product roles.
- Tests cover parent-child role mapping and filtering behavior if exposed.

Out of Scope:
- A UI for role taxonomy editing.
- Automatic role inference.

### Issue 7: Implement eater profile CRUD and allergen management

Labels: `type:feature`, `area:eater`, `area:api`, `area:db`, `priority:p0`

Context:
- `docs/SPEC.md` FR-2 and HC-1/HC-2
- `docs/schema.sql`: `eater`, `eater_allergen`
- `api/openapi/api/api.yaml`: `/eaters`, `/eaters/{eaterId}`

Scope:
- Implement create, list, get, and update for eaters.
- Persist `sex` for fiber defaults.
- Persist medical allergens separately from preference exclusions.

Acceptance Criteria:
- Eater profiles round-trip through API and Postgres.
- Allergen lists can be created and updated as part of the eater profile behavior
  supported by the current OpenAPI contract.
- `sex=unspecified` is accepted and remains the default.
- Tests cover duplicate eater names, not-found behavior, and allergen persistence.

Out of Scope:
- Macro goal versioning.
- Solver allergen filtering.

### Issue 8: Implement macro goal versioning

Labels: `type:feature`, `area:eater`, `area:api`, `area:db`, `priority:p0`

Context:
- `docs/SPEC.md` FR-2 and HC-8
- `docs/schema.sql`: `eater_macro_goal`
- `api/openapi/api/api.yaml`: `/eaters/{eaterId}/goals`

Scope:
- Implement listing macro goals from newest to oldest.
- Implement creating a new macro goal version without overwriting history.
- Add usecase/domain validation for min/max ranges.

Acceptance Criteria:
- Creating a goal inserts a new row with `effective_from`.
- Existing goals are not modified when a new version is created.
- Current-goal selection can resolve the max `effective_from <= date`.
- Invalid ranges return 422.
- Tests cover history ordering, uniqueness per date, and current-goal lookup.

Out of Scope:
- Recalculating existing plans after a goal change.
- UI for comparing goal versions.

### Issue 9: Implement fixed meals and eater exclusions

Labels: `type:feature`, `area:eater`, `area:api`, `area:db`, `priority:p0`

Context:
- `docs/SPEC.md` FR-2, HC-5, HC-10
- `docs/schema.sql`: `fixed_meal`, `eater_exclusion`
- `api/openapi/api/api.yaml`: `/eaters/{eaterId}/fixed-meals`,
  `/eaters/{eaterId}/exclusions`, `/exclusions/{exclusionId}`

Scope:
- Implement CRUD paths exposed by the OpenAPI contract for fixed meals and exclusions.
- Preserve the distinction between medical allergens and preference exclusions.
- Support hard and soft exclusions by food group, role tag, or product.

Acceptance Criteria:
- Fixed meals persist macros, meal type, and analytics/warnings flags.
- Exclusions persist target type, target value, severity, and note.
- Deleting an exclusion removes it without affecting allergens.
- Invalid exclusion targets return 422.
- Tests cover fixed meal persistence and hard/soft exclusion persistence.

Out of Scope:
- Applying fixed meals in solver calculations.
- Applying exclusions in solver calculations.

## Milestone M3: Recipe Library

Goal: make recipes and slots concrete enough for reuse-first planning and later
LLM draft validation.

### Issue 10: Implement recipe template CRUD

Labels: `type:feature`, `area:recipe`, `area:api`, `area:db`, `priority:p0`

Context:
- `docs/SPEC.md` sections 4 and FR-9
- `docs/schema.sql`: `recipe_template`, `recipe_slot`
- `api/openapi/api/api.yaml`: `/recipes`, `/recipes/{recipeId}`

Scope:
- Implement create, list, get, update, and delete for recipe templates.
- Persist recipe metadata, status, source, storage profile, equipment, weight, and slots.
- Keep slots role-based rather than product-specific.

Acceptance Criteria:
- Recipe templates with slots round-trip through API and Postgres.
- Draft and permanent statuses are represented correctly.
- Storage profile and equipment are persisted.
- Deleting or updating recipes preserves referential integrity.
- Tests cover CRUD and slot persistence.

Out of Scope:
- LLM generation.
- Plan persistence.

### Issue 11: Implement recipe and slot domain validation

Labels: `type:feature`, `area:recipe`, `area:domain`, `priority:p0`

Context:
- `docs/SPEC.md` HC-3, HC-4, HC-6
- `docs/ARCHITECTURE.md` sections 5.2 and 8
- `internal/domain/recipe/*`

Scope:
- Add domain validation for recipe slots, storage profiles, equipment, and role compatibility.
- Ensure recipes cannot be considered usable if their slots reference invalid roles.
- Keep validation independent of HTTP, OpenAPI DTOs, and sqlc rows.

Acceptance Criteria:
- Empty-slot recipes are rejected for planning use.
- Slots with missing or incompatible role tags are rejected.
- `eat_same_day` is distinguishable from 3-day-compatible storage profiles.
- Tests cover valid recipe, missing role, incompatible role kind, and storage validation.

Out of Scope:
- Product fill selection.
- Solver objective scoring.

### Issue 12: Implement draft to permanent recipe promotion

Labels: `type:feature`, `area:recipe`, `area:api`, `area:db`, `priority:p1`

Context:
- `docs/SPEC.md` FR-9
- `docs/schema.sql`: `recipe_template`
- `api/openapi/api/api.yaml`: `/recipes/{recipeId}/promote`

Scope:
- Implement the promotion endpoint and use case.
- Set `status=permanent`, `promoted_at`, and update `cooked_count` according to the schema and spec.
- Ensure only valid draft/manual templates can be promoted.

Acceptance Criteria:
- Promoting a draft marks it permanent.
- Promoting an already permanent recipe is idempotent or returns a clear conflict,
  according to the chosen usecase behavior.
- Rejected or missing recipes do not promote silently.
- Tests cover successful promotion and invalid promotion attempts.

Out of Scope:
- Feedback-driven promotion.
- Meal logging side effects beyond fields required by FR-9.

### Issue 13: Implement recipe candidate queries for reuse-first planning

Labels: `type:feature`, `area:recipe`, `area:planning`, `area:db`, `priority:p0`

Context:
- `docs/SPEC.md` FR-3 and FR-9
- `docs/ARCHITECTURE.md` sections 6, 7, and 17
- `docs/schema.sql`: `recipe_template`, `recipe_slot`

Scope:
- Add repository methods to list permanent candidate recipes with their slots.
- Keep draft recipes excluded from reuse-first candidate queries.
- Return domain recipe objects to use cases.

Acceptance Criteria:
- Candidate query returns only `status=permanent` recipes.
- Candidate query includes slots needed by the solver.
- sqlc row types do not leave the Postgres adapter.
- Tests cover permanent vs draft filtering.

Out of Scope:
- LLM fallback.
- Solver feasibility.

## Milestone M4: Nutrition and Solver Foundations

Goal: establish the deterministic macro-feasible planning path and the pure nutrition
quality logic it consumes, before persistence and HTTP orchestration.

Implementation order:

1. Issue 43 locks the shared planning and nutrition-quality contracts.
2. Issue 14 implements macro arithmetic.
3. Issue 18 defines solver inputs and hard filtering.
4. Issue 19 proves macro-feasible deterministic search.
5. Issues 15 and 16 add uncertainty handling and pure quality evaluation.
6. Issue 17 adds pure day and rolling-7d projections.

### Issue 43: Lock planning and nutrition-quality contracts

Labels: `type:docs`, `area:domain`, `area:quality`, `area:planning`, `area:db`, `priority:p0`

Context:
- `docs/DESIGN.md`
- `docs/SPEC.md`
- `docs/ARCHITECTURE.md`
- `docs/schema.sql`
- `api/openapi/api/api.yaml`

Scope:
- Resolve the shared contracts that block deterministic planning and quality evaluation.
- Make eater count dynamic and store explicit generated meal types plus the effective
  macro-goal version for each eater/day.
- Model each recipe template as one independently prepared batch component with an
  explicit kind, base serving count, normalized gram inputs, and optional prepared yield.
- Define how a meal references multiple prepared components and records each eater's
  canonical `portion_factor`; keep unallocated batch share as visible reserve.
- Define bounded deterministic search semantics, worst-eater-first fairness, and the
  distinction between exhaustive infeasibility and a search-budget stop.
- Define the units and aggregation semantics for cooked-equivalent red meat,
  whole-grain ratios, legume components/servings, and percentage-of-kcal rules.
- Define per-rule coverage, aggregate coverage, hard violations, soft statuses,
  quality scoring, and summary identity/idempotency.
- Update the source-of-truth documents and contracts consistently before application
  implementation starts.

Acceptance Criteria:
- The contracts support one, two, or more eaters without named-user branches, and
  different eaters may request different generated meal types.
- Persisted plan data can reconstruct per-eater/day macro and quality inputs without
  guessing a share of a recipe batch.
- Separate components can receive independent portions; a `mixed_main` scales
  proportionally as one component.
- Unknown nutrient and classification data remains distinguishable from known zero
  or known false values.
- Every MVP quality rule has a computable input, unit, boundary, and coverage policy.
- Hard rejection is represented separately from soft evaluation outcomes.
- Planned summary snapshots have an unambiguous identity and idempotent replacement
  behavior when plans are regenerated.
- `DESIGN.md`, `SPEC.md`, `ARCHITECTURE.md`, `schema.sql`, and `openapi.yaml` agree.

Out of Scope:
- Implementing solver search, repositories, handlers, or analytics.
- Adding v2 rule overrides or a generic nutrient engine.

### Issue 14: Implement macro arithmetic and macro goal checks

Labels: `type:feature`, `area:domain`, `area:quality`, `priority:p0`

Dependencies:
- Follows the numeric and percentage-base decisions in Issue 43.

Context:
- `docs/SPEC.md` HC-8
- `docs/ARCHITECTURE.md` section 16
- `internal/domain/nutrition/macros.go`
- `internal/domain/eater/macro_goal.go`

Scope:
- Complete macro value objects and arithmetic.
- Add range checks for protein, fat, carbs, and kcal.
- Define deterministic scaling, rounding, and boundary behavior.
- Keep macro calculations independent of persistence.

Acceptance Criteria:
- Macros can be added, scaled, and compared to goal ranges.
- Boundary values equal to min or max pass.
- Negative macro values and negative scale factors are rejected before planning use.
- Scaling and kcal rounding follow one documented deterministic policy.
- Unit tests cover arithmetic, scaling, rounding, and range checks.

Out of Scope:
- Quality nutrients.
- Solver search.

### Issue 18: Implement solver input model and hard-constraint filtering

Labels: `type:feature`, `area:solver`, `area:planning`, `priority:p0`

Dependencies:
- Blocked by Issues 14 and 43.

Context:
- `docs/SPEC.md` HC-1 through HC-10
- `docs/ARCHITECTURE.md` sections 5.7 and 16
- `internal/solver/constraints.go`
- `internal/solver/candidates.go`

Scope:
- Define solver input structures for eaters, macro goals, products, recipes,
  fixed meals, exclusions, dates, and novelty.
- Filter unusable candidates for allergens, hard exclusions, storage, equipment,
  role compatibility, and explicit industrial trans fat.
- Keep soft quality scoring outside this issue.

Acceptance Criteria:
- Products violating eater allergens are filtered out for that eater.
- Hard preference exclusions are applied separately from allergens.
- Recipes with incompatible storage profiles are excluded from multi-day planning.
- Fixed meals are present in solver input but are not swappable candidates.
- Explicit PHO or industrial trans fat produces a typed hard rejection.
- Unit tests cover each hard constraint.

Out of Scope:
- Search/objective implementation.
- Soft quality ranking.
- LLM candidate generation.

### Issue 19: Implement macro-feasible deterministic plan search

Labels: `type:feature`, `area:solver`, `area:planning`, `priority:p0`

Dependencies:
- Blocked by Issues 14 and 18.

Context:
- `docs/SPEC.md` FR-3
- `docs/ARCHITECTURE.md` sections 5.7 and 17
- `internal/solver/planner.go`
- `internal/solver/objective.go`

Scope:
- Implement a pragmatic deterministic search that can assemble a 3-day plan from
  permanent recipe candidates and products.
- Enforce hard filtering and daily macro feasibility, including fixed meals.
- Use stable deterministic tie-breaking until quality-aware ranking is added.

Acceptance Criteria:
- Solver can produce a plan for a small controlled fixture.
- Daily eater macros, including fixed meals, land inside target ranges.
- Repeated search over the same ordered input returns the same result.
- Infeasible fixtures return a typed solver result suitable for later diagnostics.
- Tests cover feasible, infeasible, and deterministic fixtures.

Out of Scope:
- Quality, rolling-pattern, cost, novelty, and coverage ranking.
- Persistence and HTTP orchestration.
- LLM fallback.
- Advanced optimization algorithms beyond MVP needs.

### Issue 15: Implement nutrition facts coverage and unknown handling

Labels: `type:feature`, `area:quality`, `area:catalog`, `priority:p0`

Dependencies:
- Blocked by Issue 43.

Context:
- `docs/SPEC.md` sections 3.2 and FR-1
- `docs/deep-research-report.md`
- `internal/domain/catalog/nutrition_facts.go`
- `internal/domain/nutrition/coverage.go`

Scope:
- Model effective hot-path facts and classification knownness in domain types.
- Implement per-rule coverage in addition to the derived aggregate product score.
- Normalize alternative sources such as sodium versus converted salt without
  double-counting coverage.
- Preserve the invariant `unknown != 0`.

Acceptance Criteria:
- Missing fiber does not count as zero fiber credit.
- Missing sodium does not count as safe sodium.
- Sodium and salt contribute one effective sodium fact.
- A known rule input remains evaluable even when unrelated product fields are missing.
- Unknown classification is distinguishable from a known negative classification.
- Aggregate coverage is derived by the server and cannot produce a false rule pass.
- Tests cover known, partially known, alternative-source, and unknown facts.

Out of Scope:
- Full micronutrient optimizer.
- Generic nutrient table.
- Rule threshold evaluation.

### Issue 16: Implement MVP quality rules evaluator

Labels: `type:feature`, `area:quality`, `priority:p0`

Dependencies:
- Blocked by Issues 15 and 43.

Context:
- `docs/SPEC.md` section 3.2
- `docs/ARCHITECTURE.md` sections 5.6 and 16
- `internal/quality/ruleset.go`
- `internal/quality/evaluator.go`

Scope:
- Implement the pure MVP ruleset for fiber, sodium, added sugar, saturated fat,
  industrial trans fat, fruit/veg, whole grains, legumes, red/processed meat, and UPF.
- Evaluate daily, plan, and supplied rolling inputs without Postgres or HTTP.
- Return per-rule status, severity, coverage, score contribution, and explanation.
- Represent explicit industrial trans fat/PHO as a typed hard violation separate from
  soft rule outcomes.

Acceptance Criteria:
- Soft rules return `pass`, `soft_fail`, `warn`, or `unknown` according to their
  documented boundaries and per-rule coverage.
- Explicit industrial trans fat/PHO returns a hard violation suitable for solver use.
- Macro-pass with a soft quality failure returns warnings rather than a hard block.
- The aggregate score is deterministic and does not replace the lexicographic
  objective used by the solver.
- Tests cover each rule family plus representative hard, soft-fail, warn, and unknown cases.

Out of Scope:
- Solver integration and plan ranking.
- Persistence.
- Per-eater public rule override API.
- Data-driven guideline engine.

### Issue 17: Implement pure day and rolling-7d quality summaries

Labels: `type:feature`, `area:quality`, `area:domain`, `priority:p0`

Dependencies:
- Blocked by Issues 15, 16, and 43.

Context:
- `docs/SPEC.md` FR-3
- `docs/ARCHITECTURE.md` sections 5.6, 13, and 16
- `internal/domain/nutrition/summary.go`
- `internal/quality/rolling.go`

Scope:
- Compute day-level summaries from per-eater meal component allocations and fixed meals.
- Project rolling-7d summaries from supplied historical day facts plus a candidate plan.
- Keep calculations pure and independent of Postgres, actual meal logging, and analytics APIs.

Acceptance Criteria:
- Red meat, processed meat, and legumes can be evaluated over rolling 7-day windows.
- Day summaries contain macro and hot-path quality aggregates with per-rule coverage.
- Fixed meals respect `include_in_analytics` and `include_in_warnings`.
- Tests cover a supplied six-day history plus the current candidate plan.
- Tests cover missing historical facts without treating them as zero or safe.

Out of Scope:
- Summary and rule-result persistence.
- Actual-basis recomputation.
- Analytics API responses.
- Background jobs.

## Milestone M5: Deterministic Plan Generation

Goal: complete quality-aware plan selection, persist the plan and its planned quality
snapshots, and expose reuse-first generation without LLM.

Implementation order:

1. Issue 44 integrates quality into the solver objective.
2. Issue 21 persists plans and per-eater meal component allocations.
3. Issue 45 persists planned summaries and rule evaluations.
4. Issue 20 exposes the transactional `GeneratePlan` use case and endpoint.
5. Issue 22 can proceed after Issue 19 and must be complete before the endpoint is final.

### Issue 44: Integrate quality evaluation into solver ranking

Labels: `type:feature`, `area:solver`, `area:quality`, `area:planning`, `priority:p0`

Dependencies:
- Blocked by Issues 16, 17, and 19.

Context:
- `docs/SPEC.md` FR-3
- `docs/ARCHITECTURE.md` sections 5.6 and 5.7
- `internal/solver/objective.go`
- `internal/quality/evaluator.go`

Scope:
- Evaluate candidate plans during search, not only after a plan has been selected.
- Implement the documented lexicographic objective: hard safety, macro feasibility,
  strongest daily quality rules, rolling weekly pattern, secondary preferences, coverage.
- Reuse the same evaluation result for returned explanations so ranking and response
  cannot disagree.

Acceptance Criteria:
- Between macro-feasible candidates, stronger daily quality rules affect selection.
- Rolling red/processed meat and legume projections affect selection after daily quality.
- Soft quality failures lower rank but do not make a macro-feasible plan impossible.
- Hard violations remain infeasible.
- Coverage is compared at the documented final lexicographic level.
- Tests prove each objective level without collapsing the hierarchy into one score.

Out of Scope:
- Persistence and HTTP mapping.
- LLM fallback.
- Advanced optimization algorithms beyond MVP needs.

### Issue 21: Persist plans, meals, component allocations, and slot fills

Labels: `type:feature`, `area:planning`, `area:db`, `priority:p0`

Dependencies:
- Blocked by Issues 19 and 43.

Context:
- `docs/schema.sql`: `plan`, `plan_recipe`, `slot_fill`, `meal` and the
  meal-component allocation defined by Issue 43
- `docs/SPEC.md` FR-3 and FR-6
- `docs/ARCHITECTURE.md` sections 10 and 11

Scope:
- Implement persistence for generated plans and all records needed to retrieve them.
- Store planned macros and each eater's component portions on meals.
- Keep transaction boundaries at the usecase layer.

Acceptance Criteria:
- A generated plan can be saved and retrieved by id.
- Meals include eater, date, type, planned macros, and one or more component allocations.
- Persisted allocations can reconstruct per-eater planned quality inputs.
- Slot fills retain product quantities and role-slot relationships.
- Partial persistence rolls back when any save step fails.
- Repository tests cover save and get.

Out of Scope:
- Planned summary and rule-result persistence.
- Actual meal logging.
- Shopping list generation.

### Issue 45: Persist planned nutrition summaries and rule evaluations

Labels: `type:feature`, `area:quality`, `area:planning`, `area:analytics`, `area:db`, `priority:p0`

Dependencies:
- Blocked by Issues 17, 21, and 44.

Context:
- `docs/SPEC.md` FR-3
- `docs/schema.sql`: `day_nutrition_summary`,
  `rolling_nutrition_summary`, `rule_evaluation_result`
- `docs/ARCHITECTURE.md` sections 11 and 13

Scope:
- Persist planned day and rolling summaries plus rule evaluation results when a plan is saved.
- Use the snapshot identity and replacement semantics defined by Issue 43.
- Keep summary writes in the same usecase transaction as plan persistence.

Acceptance Criteria:
- Planned summaries and rule results can be queried by plan, entity, and eater.
- Regenerating or reevaluating the same plan does not create duplicate rule results.
- Multiple plans covering the same eater/date do not overwrite each other's snapshots.
- A failed summary write rolls back the generated plan transaction.
- Repository integration tests cover save, replacement, and collision boundaries.

Out of Scope:
- Actual-basis summaries.
- Analytics API responses.
- Background jobs.

### Issue 20: Implement GeneratePlan reuse-first use case without LLM

Labels: `type:feature`, `area:planning`, `area:solver`, `area:api`, `priority:p0`

Dependencies:
- Blocked by Issues 21, 22, 44, and 45.

Context:
- `docs/SPEC.md` FR-3
- `docs/ARCHITECTURE.md` section 11
- `api/openapi/api/api.yaml`: `/plans`
- `internal/usecase/planning/generate_plan.go`

Scope:
- Implement transactional `GeneratePlan` using permanent recipe candidates first.
- Call the quality-aware solver and persist the selected plan, allocations, summaries,
  and rule evaluations.
- Return an application-level result suitable for OpenAPI mapping.

Acceptance Criteria:
- `POST /plans` creates a persisted plan without calling LLM.
- Plan response includes macro totals and quality evaluation data required by OpenAPI.
- `novelty_level` is accepted and passed through even if only deterministic behavior is implemented.
- A persistence failure does not leave a partial plan.
- Tests cover successful generation, validation failure, and transactional rollback.

Out of Scope:
- LLM fallback.
- Swap behavior.

### Issue 22: Return useful impossible-plan diagnostics

Labels: `type:feature`, `area:solver`, `area:api`, `priority:p1`

Dependencies:
- Blocked by Issue 19.

Context:
- `docs/SPEC.md` section 6
- `docs/ARCHITECTURE.md` section 16
- `internal/solver/explain.go`

Scope:
- Produce human-readable diagnostics when no feasible plan can be generated.
- Identify the likely failing class: missing role, allergen/exclusion conflict,
  storage conflict, equipment conflict, or macro infeasibility.
- Map diagnostics to 422 responses.

Acceptance Criteria:
- Impossible solver results include structured reason codes and readable messages.
- API returns 422 with diagnostics instead of generic internal errors.
- Tests cover at least three impossible-plan classes.

Out of Scope:
- Automatic product or recipe suggestions.
- LLM recovery behavior.

## Milestone M6: LLM Recipe Candidate Fallback

Goal: add LLM only as a source of draft recipe candidates, keeping Go validation
and solver decision-making deterministic.

### Issue 23: Implement LLM HTTP client with cache, retry, timeout, and logging

Labels: `type:feature`, `area:llm`, `area:db`, `priority:p1`

Context:
- `docs/DESIGN.md` section 11
- `docs/SPEC.md` sections 6 and 7
- `docs/schema.sql`: `llm_call`, `llm_cache`
- `internal/llm/client.go`

Scope:
- Replace the stub with an HTTP-based provider adapter.
- Add timeout, retry for transient provider failures, prompt hash cache, and call logging.
- Keep provider SDKs out unless there is a clear reason to add one.

Acceptance Criteria:
- Client supports configured model, temperature, timeout, and endpoint/key.
- Successful, failed, and cached calls are logged.
- 429/529-style transient failures retry with bounded exponential backoff.
- Exhausted provider failures return an application error that can map to 503.
- Tests use a local fake HTTP server.

Out of Scope:
- Prompt design.
- Recipe candidate validation.

### Issue 24: Implement recipe generation prompt and structured parsing

Labels: `type:feature`, `area:llm`, `area:recipe`, `priority:p1`

Context:
- `docs/DESIGN.md` section 11
- `docs/SPEC.md` FR-3 and section 8
- `internal/llm/prompts/recipe_generation.md`
- `internal/llm/recipe_generator.go`

Scope:
- Build the prompt input from available products, constraints, role tags, and novelty.
- Parse structured recipe candidate output into draft domain objects.
- Keep prompt files versioned in the repository.

Acceptance Criteria:
- Prompt includes only v1-supported constraints and available products.
- Parsed candidates do not become plans directly.
- Invalid structured output returns a validation error with call logging.
- Tests cover prompt construction and parser behavior with fixed fixtures.

Out of Scope:
- Vision, voice, feedback parsing, RAG, agents, or fine-tuning.
- Saving drafts to Postgres.

### Issue 25: Validate and integrate LLM drafts into GeneratePlan fallback

Labels: `type:feature`, `area:llm`, `area:planning`, `area:recipe`, `priority:p1`

Context:
- `docs/SPEC.md` FR-3 and section 6
- `docs/ARCHITECTURE.md` section 8
- `internal/usecase/planning/generate_plan.go`

Scope:
- Call LLM only when permanent candidates are insufficient or `novelty_level = 3`
  explicitly requests a new draft candidate.
- Validate LLM drafts against products, slots, roles, storage, equipment, and macros.
- Save valid candidates as draft recipes and pass them to the solver as candidates.

Acceptance Criteria:
- LLM is not called when reuse-first candidates are enough and `novelty_level < 3`.
- LLM output containing products outside the database is rejected.
- Valid LLM drafts are persisted as `draft`, not `permanent`.
- Solver still chooses the final plan.
- Tests cover no-call, call-and-valid, and call-and-invalid paths.

Out of Scope:
- LLM-generated final plans.
- Promotion of drafts after cooking.

## Milestone M7: Plan Operations

Goal: support the core post-generation operations: swap preview, confirmed swap,
planned batch quantity adjustments, prep dependency effects, and shopping list
regeneration.

### Issue 26: Implement swap preview with macro and quality diff

Labels: `type:feature`, `area:planning`, `area:quality`, `area:api`, `priority:p1`

Context:
- `docs/SPEC.md` FR-4
- `docs/DESIGN.md` section 6
- `api/openapi/api/api.yaml`: `/plans/{planId}/swap/preview`
- `internal/usecase/planning/preview_swap.go`

Scope:
- Implement preview for fill-to-fill and protein-to-protein swaps.
- Return affected meals, macro diff, quality diff, and warnings before applying changes.

Acceptance Criteria:
- Preview does not mutate the plan.
- Macro diff includes affected day/eater totals.
- Quality diff includes hot-path nutrients and coverage changes.
- Invalid swap targets return 422 with explanation.
- Tests cover no-op-like swap and materially different swap.

Out of Scope:
- Recipe-to-recipe or full-rotation swaps if they require broader solver work.
- Shopping list mutation.

### Issue 27: Implement prep dependency invalidation in swaps

Labels: `type:feature`, `area:planning`, `area:db`, `priority:p1`

Context:
- `docs/SPEC.md` FR-4
- `docs/DESIGN.md` section 6
- `docs/schema.sql`: `prep_dependency`
- `internal/domain/planning/prep_dependency.go`

Scope:
- Model and persist prep dependencies between plan recipes.
- Include invalidated dependent dishes in swap preview.
- Prevent silent breakage when a source component changes.

Acceptance Criteria:
- Prep dependencies can be saved and loaded with a plan.
- Swap preview reports dependent meals/recipes that become invalid.
- Applying a swap that breaks dependencies requires explicit confirmation.
- Tests cover a dependency chain with at least one invalidated dependent dish.

Out of Scope:
- Automatic repair of dependency chains.
- Full plan regeneration.

### Issue 28: Implement shopping list generation and regeneration after swaps

Labels: `type:feature`, `area:shopping`, `area:planning`, `area:api`, `priority:p1`

Context:
- `docs/SPEC.md` FR-5
- `docs/schema.sql`: `shopping_list`, `shopping_list_item`
- `api/openapi/api/api.yaml`: `/plans/{planId}/shopping-list`,
  `/shopping-list-items/{itemId}`

Scope:
- Generate a shopping list from plan slot fills.
- Aggregate one row per product and estimate cost.
- Preserve checked status for unchanged items during regeneration.
- Mark and regenerate shopping lists when confirmed swaps change products.

Acceptance Criteria:
- One product appears once with summed quantity.
- Checked items can be toggled.
- Regeneration preserves checked state for unchanged product/quantity rows.
- Swaps that change products mark the list stale or regenerate according to chosen usecase behavior.
- Tests cover aggregation, checked state, and regeneration.

Out of Scope:
- Inventory tracking.
- Store-specific purchasing integrations.

### Issue 46: Implement planned batch quantity adjustment preview and apply

Labels: `type:feature`, `area:planning`, `area:solver`, `area:quality`,
`area:shopping`, `area:api`, `area:db`, `priority:p1`

Context:
- `docs/SPEC.md` FR-3, FR-4, and FR-5
- `docs/DESIGN.md` section 6
- `docs/schema.sql`: `slot_fill` and planned nutrition snapshots
- `api/openapi/api/api.yaml`: existing plan-operation preview patterns
- Blocked by Issues 20, 21, 26, 28, and 45.

Scope:
- Define the API/use-case contract for previewing and applying a planned slot-fill
  quantity change before cooking.
- Treat the entered quantity as a locked planning input and ask the solver to rebalance
  the affected plan with the smallest practical blast radius.
- Recalculate component allocations, eater/day macros, nutrition-quality results, and
  shopping-list quantities.
- Return affected meals, portion changes, macro and quality diffs, shopping-list diff,
  warnings, and infeasibility diagnostics in preview.
- Apply the accepted preview transactionally after explicit confirmation.

Acceptance Criteria:
- Previewing a quantity change does not mutate the plan.
- A change such as chicken `1,200 g -> 988 g` shows the resulting per-eater portion,
  macro, quality, and shopping-list changes.
- When the existing plan can remain feasible, the solver returns a deterministic
  rebalanced preview.
- When it cannot remain feasible, the API returns typed 422 diagnostics explaining the
  unsatisfied constraints and possible repair directions.
- Applying a confirmed preview atomically updates the planned slot-fill quantity,
  component allocations, planned nutrition summaries/rule evaluations, and shopping
  list state.
- Tests cover feasible rebalance, infeasible preview, non-mutating preview, and
  transaction rollback.

Out of Scope:
- Household inventory or package catalogs.
- Store integrations or automatic package-size selection.
- Actual ingredient usage, cooked yield, container weighing, or meal logging.
- Background replanning without explicit owner confirmation.

## Milestone M8: Reality Loop and Analytics

Goal: close the loop after planning: actual food logging, feedback weights, and
owner-facing analytics.

### Issue 29: Implement meal logging planned vs actual

Labels: `type:feature`, `area:meals`, `area:api`, `area:db`, `priority:p1`

Context:
- `docs/SPEC.md` FR-6
- `docs/schema.sql`: `meal`
- `api/openapi/api/api.yaml`: `/plans/{planId}/meals`, `/meals/{mealId}/log`

Scope:
- List meals for a plan.
- Log meal status as planned, eaten as planned, modified, or skipped.
- Store actual macros only for modified meals.
- Treat actual quality as unknown when there is no structured replacement.

Acceptance Criteria:
- `actual_*` NULL means analytics use planned values.
- `eaten_modified` can store actual macros.
- Modified meals without structured replacement reduce actual quality coverage.
- `cooked_by` and `logged_by` are persisted where supported by schema/API.
- Tests cover each meal log status.

Out of Scope:
- Voice/photo logging.
- Manual actual quality nutrient entry.

### Issue 30: Implement feedback events and weight adjustment

Labels: `type:feature`, `area:feedback`, `area:recipe`, `area:catalog`, `priority:p1`

Context:
- `docs/SPEC.md` FR-7
- `docs/schema.sql`: `feedback_event`, `recipe_template`
- `api/openapi/api/api.yaml`: `/feedback`

Scope:
- Persist feedback events for recipe templates, products, meals, and slot fills.
- Apply simple weight adjustment for recipe/product targets.
- Keep feedback as probability influence, not a blacklist.

Acceptance Criteria:
- Rating range is enforced.
- Comment text is stored.
- Weight changes are bounded and do not delete or ban targets.
- Tests cover recipe feedback, product feedback, and invalid targets.

Out of Scope:
- LLM parsing of feedback comments.
- Usage-based inventory weights.

### Issue 31: Implement macro analytics

Labels: `type:feature`, `area:analytics`, `area:api`, `area:db`, `priority:p1`

Context:
- `docs/SPEC.md` FR-8
- `api/openapi/api/api.yaml`: `/analytics/macros`
- `docs/schema.sql`: `meal`, `day_nutrition_summary`

Scope:
- Implement macro analytics by eater, period, and basis.
- Support planned and actual views.
- Return averages and compliance against goal ranges where supported by OpenAPI.

Acceptance Criteria:
- Query parameters validate eater, date range, and basis.
- Planned basis uses planned values.
- Actual basis uses actual values when present and planned values when actual is absent according to FR-6.
- Tests cover empty period, planned period, and modified actual period.

Out of Scope:
- Quality analytics.
- Data export.

### Issue 32: Implement nutrition-quality and top-recipe analytics

Labels: `type:feature`, `area:analytics`, `area:quality`, `area:recipe`, `priority:p1`

Dependencies:
- Blocked by Issues 17, 29, and 45.

Context:
- `docs/SPEC.md` FR-8
- `api/openapi/api/api.yaml`: `/analytics/nutrition-quality`,
  `/analytics/top-recipes`
- `docs/schema.sql`: `rule_evaluation_result`, `day_nutrition_summary`,
  `rolling_nutrition_summary`, `recipe_template`, `meal`

Scope:
- Recompute and persist actual-basis day and rolling summaries from meal log state.
- Treat modified meals without a structured replacement as unknown quality rather
  than copying planned facts.
- Implement nutrition-quality analytics with coverage and rule results.
- Implement top recipes and product frequency analytics.
- Include fixed meal quality load where the data is available.

Acceptance Criteria:
- Actual summaries use known structured facts and reduce per-rule coverage when a
  modified meal's quality is unknown.
- Recomputing the same actual period replaces summaries and rule results idempotently.
- Quality analytics returns hot-path averages, rule statuses, and coverage.
- Rolling meat/legume exposure can be reported for a period.
- Top recipes sort by cooked count or meal usage according to OpenAPI behavior.
- Product frequency can be derived from meals/slot fills where required.
- Tests cover quality analytics with low coverage and top recipe ordering.

Out of Scope:
- Charts or UI.
- Advanced cohort analytics.

## Milestone M9: MVP Hardening

Goal: prove the full owner-only workflow works end-to-end and align docs with the
implemented behavior.

### Issue 33: Add starter seed data for local MVP testing

Labels: `type:feature`, `area:db`, `area:catalog`, `area:eater`, `priority:p2`

Context:
- `docs/SPEC.md` section 2
- `docs/DESIGN.md` sections 1 and 8
- `docs/schema.sql`

Scope:
- Add seed data for Pasha/Sasha profiles, macro goals, allergens, Sasha fixed breakfast,
  core role tags, and a small realistic product/recipe fixture set.
- Keep seed data explicitly local/dev-oriented.

Acceptance Criteria:
- A fresh local database can be seeded into a useful demo state.
- Seed data respects known allergens and exclusions.
- Seed products include enough quality coverage to exercise quality warnings.
- Documentation explains how to apply seeds locally.

Out of Scope:
- Production data import.
- Inventory or shopping history.

### Issue 34: Add end-to-end API smoke test and docs consistency sweep

Labels: `type:test`, `type:docs`, `area:api`, `priority:p0`

Context:
- `README.md`
- `docs/SPEC.md`
- `docs/ARCHITECTURE.md`
- `api/openapi/api/api.yaml`

Scope:
- Add an end-to-end smoke test covering the MVP path:
  product/eater setup, recipe, plan generation, shopping list, meal log, analytics.
- Review docs and README for behavior that diverged during implementation.
- Keep v2 features out of MVP docs.

Acceptance Criteria:
- Smoke test exercises the main HTTP API workflow against a migrated database.
- `make generate`, `go test ./...`, and the documented local startup path are verified.
- Docs do not claim unimplemented v2 functionality as MVP behavior.
- OpenAPI, schema, and README references are consistent.

Out of Scope:
- Performance testing.
- Public deployment automation.
