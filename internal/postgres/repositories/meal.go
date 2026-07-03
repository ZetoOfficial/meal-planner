// Package repositories — адаптеры Postgres: маппинг sqlc rows ↔ domain.
// sqlc-типы НЕ должны протекать в domain/usecase/solver/quality (ARCHITECTURE §10).
package repositories

import (
	"context"

	"github.com/ZetoOfficial/meal-planner/internal/postgres/sqlc"
)

// MealRepository реализует соответствующий usecase-порт поверх sqlc.
type MealRepository struct {
	q sqlc.Querier
}

func NewMealRepository(q sqlc.Querier) *MealRepository {
	return &MealRepository{q: q}
}

func (r *MealRepository) LogActual(context.Context, int64, any) error {
	return ErrNotImplemented
}
