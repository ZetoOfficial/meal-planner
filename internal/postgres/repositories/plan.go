// Package repositories — адаптеры Postgres: маппинг sqlc rows ↔ domain.
// sqlc-типы НЕ должны протекать в domain/usecase/solver/quality (ARCHITECTURE §10).
package repositories

import (
	"context"

	"github.com/ZetoOfficial/meal-planner/internal/postgres/sqlc"
)

// PlanRepository реализует соответствующий usecase-порт поверх sqlc.
type PlanRepository struct {
	q sqlc.Querier
}

func NewPlanRepository(q sqlc.Querier) *PlanRepository {
	return &PlanRepository{q: q}
}

func (r *PlanRepository) Save(context.Context, any) error {
	return ErrNotImplemented
}

func (r *PlanRepository) Get(context.Context, int64) (any, error) {
	return nil, ErrNotImplemented
}
