// Package repositories — адаптеры Postgres: маппинг sqlc rows ↔ domain.
// sqlc-типы НЕ должны протекать в domain/usecase/solver/quality (ARCHITECTURE §10).
package repositories

// RecipeRepository реализует соответствующий usecase-порт поверх sqlc.
// TODO: добавить *sqlc.Queries после `make generate` и реализовать методы порта.
type RecipeRepository struct{}
