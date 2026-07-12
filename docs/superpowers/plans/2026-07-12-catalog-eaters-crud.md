# M2 Catalog and Eaters CRUD Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement roadmap issues #5–#9 as the owner-only OpenAPI API for products, role tags, eaters, goals, fixed meals, and exclusions.

**Architecture:** Keep rich catalog/eater data in domain packages and expose use-case ports from their consumer packages. Postgres repositories map sqlc rows and execute multi-table aggregate writes in the existing transaction context; one HTTP adapter maps OpenAPI DTOs to use-case commands. The existing M1 HTTP wiring gap is completed only as necessary to mount these M2 routes and return existing 501 responses for non-M2 operations.

**Tech Stack:** Go 1.25, net/http, oapi-codegen, pgx v5, sqlc, Postgres, Go unit/integration tests.

**Verification environment:** The current shell is Go 1.20.5 while `go.mod` requires Go 1.25, so local `go test ./...` cannot build (`log/slog` and the Go-1.22 generated HTTP adapter are unavailable). Do not weaken module or build constraints; run every listed Go verification command through a Go 1.25 runtime before claiming it passes.

---

### Task 1: Add the role taxonomy and catalog persistence surface

**Files:**
- Create: `migrations/002_seed_role_tags.sql`
- Modify: `internal/postgres/sqlc/query/products.sql`
- Generated: `internal/postgres/sqlc/products.sql.go`, `internal/postgres/sqlc/models.go`, `internal/postgres/sqlc/querier.go`
- Create: `internal/postgres/repositories/catalog.go`
- Test: `internal/postgres/catalog_integration_test.go`

- [ ] Seed role tags in parent-before-child order using `INSERT ... ON CONFLICT (code) DO UPDATE`, with parent lookup by code. Include root kinds and MVP slot descendants: `protein`, `protein:lean`, `protein:lean:breast`, `carb`, `carb:grain`, `veg`, `veg:green`, `sauce`, `sauce:fat`, `fat`, and `other`.
- [ ] Add sqlc queries for a complete product aggregate: scalar create/get/list/update/delete; role lookup/list; product role list/delete/insert; product allergen list/delete/insert; and food-classification get/upsert. Query return rows must include all `product` columns, never encode absent quality data as zero.
- [ ] Regenerate with `make sqlc`; never edit `*.sql.go`, `models.go`, or `querier.go` manually.
- [ ] Implement `repositories.Catalog` against `Repositories.Queries(ctx)` with mapping helpers that use `pgtype.Numeric` and nullable pgtype fields. `Create` and `Update` must write the product, classification, roles and allergens in the caller transaction; update replaces collection values rather than appending duplicates.
- [ ] Write one integration round-trip that creates a product with roles, allergens, nullable nutrition facts and classification, reloads it, replaces the aggregate, then proves stale role/allergen values are gone and an omitted nutrition value remains `NULL`.
- [ ] Run: `go test -tags=integration -count=1 ./internal/postgres/...` with Go 1.25 and `TEST_DATABASE_URL`; expected: focused catalog test passes.
- [ ] Commit: `feat(catalog): add product persistence and role taxonomy`.

### Task 2: Implement catalog domain rules and use cases

**Files:**
- Modify: `internal/domain/catalog/product.go`
- Modify: `internal/domain/catalog/nutrition_facts.go`
- Modify: `internal/domain/catalog/role_tag.go`
- Modify: `internal/domain/catalog/allergen.go`
- Modify: `internal/usecase/catalog/ports.go`
- Modify: `internal/usecase/catalog/service.go`
- Test: `internal/usecase/catalog/service_test.go`

- [ ] Model a complete `catalog.Product` aggregate: identifiers, macros, optional price/unit/purchasable, nutrition facts, nutrition-data provenance, food classification, allergen labels and role tags. Keep optional quality values as pointer/nullable domain values and derive coverage from the populated hot-path nutrition fields.
- [ ] Add consumer-side repository commands for list/get/create/update/delete products and list/lookup role tags; map `pgx.ErrNoRows`, unique violations and FK failures in the adapter to application not-found/conflict errors.
- [ ] Add commands and service methods: `CreateProduct`, `ListProducts(roleCode)`, `GetProduct`, `UpdateProduct`, `DeleteProduct`, and `ListRoleTags`.
- [ ] Validate only M2 invariants: trimmed nonblank brand/name, nonnegative numeric inputs, at least one role, `0..1` classification ratios, known role codes, and a role code prefix matching its persisted role kind. Return `apperr.Validation`, `apperr.NotFound`, or `apperr.Conflict`, never HTTP errors.
- [ ] Write focused unit tests for rejected macro/quality values, a role-kind mismatch, and server-derived coverage; do not create redundant tests for plain repository pass-throughs.
- [ ] Run: `go test ./internal/domain/catalog ./internal/usecase/catalog` with Go 1.25; expected: all catalog tests pass.
- [ ] Commit: `feat(catalog): add product use cases`.

### Task 3: Add eater persistence, domain rules, and use cases

**Files:**
- Modify: `internal/domain/eater/eater.go`
- Modify: `internal/domain/eater/macro_goal.go`
- Modify: `internal/domain/eater/fixed_meal.go`
- Create: `internal/domain/eater/exclusion.go`
- Modify: `internal/usecase/eater/ports.go`
- Modify: `internal/usecase/eater/service.go` (split into focused files only if it becomes unwieldy)
- Modify: `internal/postgres/sqlc/query/eaters.sql`
- Generated: `internal/postgres/sqlc/eaters.sql.go`, `internal/postgres/sqlc/models.go`, `internal/postgres/sqlc/querier.go`
- Create: `internal/postgres/repositories/eater.go`
- Test: `internal/usecase/eater/service_test.go`, `internal/postgres/eater_integration_test.go`

- [ ] Expand the eater model to include profile allergens, macro-goal id/note/range/effective date, fixed-meal macros and optional quality fields, and exclusion target/severity/note. Validate all ranges as nonnegative minimum ≤ maximum.
- [ ] Add sqlc commands for eater create/update, allergen list and replacement, goal insertion/current lookup/history, fixed-meal insert/list, exclusion insert/list/delete-returning, and role/product target existence checks. Regenerate with `make sqlc`.
- [ ] Implement transaction-aware repository operations. `CreateEater` writes profile, initial goal and allergens atomically. Confirmed `UpdateEater` semantics update profile, replace allergens and append a goal version atomically. A fixed-meal list returns active entries only.
- [ ] Implement use cases for profile create/list/get/update; list/create goal versions; list/create fixed meals; and list/create/delete exclusions. Validate target kind precisely: allowed food-group enum, existing role code, or positive existing product id. Default omitted exclusion severity to `hard` and omitted fixed-meal quality inclusion flags to `true`.
- [ ] Preserve medical allergens as an independent collection from preference exclusions. Missing eater, role and product references must become application not-found errors; duplicate names, goal dates and exclusion targets become conflicts.
- [ ] Write focused tests for macro history order/current selection, atomic allergen replacement, invalid exclusion targets and nullable fixed-meal quality data. Add one integration round-trip covering eater → goal history → fixed meal → exclusion.
- [ ] Run: `go test ./internal/domain/eater ./internal/usecase/eater` and `go test -tags=integration -count=1 ./internal/postgres/...` with Go 1.25; expected: focused tests pass.
- [ ] Commit: `feat(eater): add profile and preference use cases`.

### Task 4: Mount OpenAPI and implement M2 HTTP mapping

**Files:**
- Create: `internal/api/http/api.go`
- Create: `internal/api/http/catalog_handlers.go`
- Create: `internal/api/http/eater_handlers.go`
- Modify: `internal/api/http/server.go`
- Modify: `cmd/api/main.go`
- Modify: `api/openapi.yaml` only to declare existing 409/404 error responses where the shared error behavior already requires them
- Generated: `internal/api/openapi/generated.go`
- Modify: `internal/api/http/server_test.go`
- Create: `internal/api/http/m2_handlers_test.go`

- [ ] Define an API implementation that satisfies generated `openapi.ServerInterface`: map all M2 operations to catalog/eater use cases and retain structured 501 responses for untouched M3+ operations.
- [ ] Convert OpenAPI pointers, `float32`, date values and enums at the HTTP boundary. Convert domain results back to every supported OpenAPI field, including product aggregate values and eater current goal/allergens/exclusions. Never let OpenAPI DTOs enter the domain or use-case layers.
- [ ] Extend the source contract only for missing status declarations already required by M2 behavior: conflicts for duplicate eater/goal/exclusion operations and not-found for eater-scoped fixed-meal operations. Regenerate with `make oapi`; do not hand-edit the generated file and do not add routes or schemas.
- [ ] Mount `openapi.HandlerFromMux` alongside unauthenticated `/livez`, wrapping API traffic in the existing `X-API-Token` middleware. Extend `httpapi.Deps` with M2 use cases and build repositories/services/transaction manager in `cmd/api/main.go`.
- [ ] Test one product create/list role-filter round trip and representative 404, 409 and 422 JSON responses; retain tests proving `/livez` needs no token and protected API routes do.
- [ ] Run: `go test ./internal/api/http ./cmd/api` with Go 1.25; expected: API tests pass.
- [ ] Commit: `feat(api): expose catalog and eater CRUD`.

### Task 5: Review, integration verification, and PR preparation

**Files:**
- Modify only files required by review findings.

- [ ] Run generated-code consistency checks: `make generate` and `git diff --exit-code`.
- [ ] Run unit/build verification through Go 1.25: `go test ./...`, `go build ./cmd/api`, and `make lint`.
- [ ] If `TEST_DATABASE_URL` is configured, run `go test -tags=integration -count=1 ./internal/postgres/...`; otherwise record the explicit skipped integration condition without claiming integration verification.
- [ ] Request separate spec-compliance and code-quality reviews; fix every critical or important finding and re-run the affected commands.
- [ ] Inspect `git diff --check`, `git status --short`, and the final diff against `master`; ensure scope is limited to issues #5–#9 plus required M1 wiring and OpenAPI response declarations.
- [ ] Commit review fixes with a Conventional Commit if needed, then push `feat/catalog-eaters-crud` and open a draft PR whose body contains `Closes #5`, `Closes #6`, `Closes #7`, `Closes #8`, and `Closes #9` on separate lines.
