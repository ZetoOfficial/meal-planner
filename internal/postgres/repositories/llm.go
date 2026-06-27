// Package repositories — адаптеры Postgres: маппинг sqlc rows ↔ domain.
// sqlc-типы НЕ должны протекать в domain/usecase/solver/quality (ARCHITECTURE §10).
package repositories

// LLMRepository — лог вызовов LLM и кэш по хешу промпта (SPEC §11).
// TODO: добавить *sqlc.Queries после `make generate` и реализовать методы порта.
type LLMRepository struct{}
