// Package repositories — адаптеры Postgres: маппинг sqlc rows ↔ domain.
// sqlc-типы НЕ должны протекать в domain/usecase/solver/quality (ARCHITECTURE §10).
package repositories

// PlanRepository реализует соответствующий usecase-порт поверх sqlc.
// TODO: добавить *sqlc.Queries после `make generate` и реализовать методы порта.
type PlanRepository struct{}
