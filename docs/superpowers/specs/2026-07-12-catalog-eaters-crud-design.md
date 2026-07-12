# M2 Catalog and Eaters CRUD Design

## Scope

Implement roadmap issues 5–9: product CRUD with role, allergen, quality and
food-classification data; seeded role-tag listing; eater profiles with medical
allergens; immutable macro-goal versions; and fixed meals plus preference
exclusions.

M2 is an owner-only input boundary for later recipe and planning work. It does
not implement solver behavior, quality evaluation, recipe validation, LLM
integration, or new HTTP endpoints.

## Contract and HTTP behavior

The existing `api/openapi.yaml` is authoritative. No routes are added.

- Products support create, list (optionally filtered by role), get, update, and
  delete.
- Role tags are seeded and exposed read-only through `GET /role-tags`.
- Eaters support create, list, get, and update. The input allergen list is the
  complete desired medical-allergen set, so create and update replace it.
- An eater input includes a goal. Creating an eater writes the first goal;
  updating an eater creates a new goal version rather than modifying history.
- Goals are listed newest first and can be appended through the dedicated goal
  endpoint.
- Fixed meals have list/create behavior only, because the OpenAPI contract does
  not expose update or delete operations.
- Exclusions have list/create/delete behavior. They remain distinct from
  medical allergens.

Handlers only decode and map OpenAPI values to use-case commands, map results
back to DTOs, and pass application errors through the shared error mapper.

## Core and persistence boundaries

`domain/catalog` models complete products, nutrition facts, food
classification, and role tags. `domain/eater` models profiles, macro ranges and
versions, fixed meals, and preference exclusions. Neither package imports HTTP,
OpenAPI, sqlc, or Postgres types.

`usecase/catalog` and `usecase/eater` own repository ports and command-level
validation. Postgres repositories implement those ports and hide sqlc rows.
Multi-table replacement operations run atomically:

- product roles, allergens, and classification are saved together;
- eater data and its allergen set are saved together;
- first-goal creation and profile creation are saved together.

## Validation and errors

- Required names and product identifiers must be non-empty; all macro and
  nutrition amounts must be non-negative.
- Macro range minimums must not exceed maximums.
- Product role codes must exist and their code prefix must agree with their
  stored role kind.
- An exclusion target must refer to a valid food-group enum, an existing role
  code, or an existing product id, according to `target_kind`.
- Nullable nutrition and fixed-meal quality values are preserved as unknown
  (`NULL`), never converted to zero. Product nutrition coverage is derived by
  the server.
- Duplicate eater names, product brand/name pairs, effective goal dates, and
  eater exclusion targets map to conflict (409). Invalid commands map to 422,
  missing resources map to 404.

## Role taxonomy

A migration seeds a small stable taxonomy needed by MVP recipe slots, including
parent/child entries such as `protein`, `protein:lean`, and
`protein:lean:breast`, plus carb, vegetable, sauce, fat and other roles. Seeds
are idempotent and role ids/codes remain stable for product assignments and
OpenAPI responses.

## Verification strategy

Tests are risk-focused rather than mechanically exhaustive:

- macro-goal ranges, history ordering, effective-date uniqueness and current
  goal lookup;
- replacement and round-trip behavior for related product/eater data, including
  `NULL` nutrition fields;
- validation of exclusion targets;
- HTTP mapping for representative 404, 409 and 422 paths;
- a small number of Postgres/API round-trips proving adapter wiring.

The database constraints, sqlc generation and OpenAPI types cover routine
storage and DTO mechanics not specific to M2 invariants.

## Non-goals

No public-SaaS concerns, new API routes, role-taxonomy editing, automatic role
or food-classification inference, recipe slots, solver candidate selection,
allergen/exclusion enforcement in planning, quality evaluation, LLM behavior,
or UI work are included.
