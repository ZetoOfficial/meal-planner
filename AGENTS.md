# Agent Notes

This is an owner-only Go modular monolith for meal planning, not a SaaS platform and not a microservice system. Keep the implementation boring, explicit, and close to the current MVP scope.

Architecture: **domain-first modular monolith + DDD-lite + ports/adapters**. Use Clean Architecture as a dependency rule, not as boilerplate folder ceremony.

Read these before making structural changes:

- `docs/DESIGN.md`
- `docs/SPEC.md`
- `docs/ARCHITECTURE.md`
- `docs/schema.sql`
- `api/openapi.yaml`

Core rules:

- HTTP, OpenAPI, Postgres, sqlc and LLM providers are adapters.
- Solver, quality rules, plan generation, recipe slots and macro logic are core.
- Core packages must not import HTTP handlers, OpenAPI DTOs, sqlc rows, Postgres repositories or provider SDKs.
- LLM only generates recipe candidates. Go validates them, the solver decides whether to use them, and the quality evaluator explains the result.
- Prefer small Go packages with domain names over generic folders like `services`, `utils`, `common`, `interfaces`.
- Put interfaces on the consumer side, usually in `usecase/*/ports.go`.
- Do not invent fields or endpoints. `docs/schema.sql` and `api/openapi.yaml` are the source of truth.
- Keep v1 scope: HTTP API only. Telegram bot, Mini App, voice, vision/barcode, FatSecret sync, inventory, RAG, agents and fine-tuning are v2+ or explicitly out of scope.

Git conventions:

- Commits follow **Conventional Commits**: `type(scope): summary` (e.g. `feat(planning): reuse-first plan generation`). Types: `feat`, `fix`, `chore`, `docs`, `refactor`, `test`, `perf`, `build`, `ci`.
- Branches mirror the commit type: `type/short-description`, kebab-case, e.g. `feat/eater-exclusions`; prefix the issue number when there is one: `feat/42-generate-plan`.

When unsure, choose the simplest implementation that preserves the important boundary: **deterministic Go core first, external adapters second**.
