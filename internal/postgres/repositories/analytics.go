// Package repositories — адаптеры Postgres: маппинг sqlc rows ↔ domain.
// sqlc-типы НЕ должны протекать в domain/usecase/solver/quality (ARCHITECTURE §10).
package repositories

import (
	"context"

	"github.com/ZetoOfficial/meal-planner/internal/postgres/sqlc"
)

// AnalyticsRepository реализует соответствующий usecase-порт поверх sqlc.
type AnalyticsRepository struct {
	q sqlc.Querier
}

func NewAnalyticsRepository(q sqlc.Querier) *AnalyticsRepository {
	return &AnalyticsRepository{q: q}
}

func (r *AnalyticsRepository) MacroAverages(context.Context, int64, string, string) (any, error) {
	return nil, ErrNotImplemented
}

func (r *AnalyticsRepository) QualityAverages(context.Context, int64, string, string) (any, error) {
	return nil, ErrNotImplemented
}

func (r *AnalyticsRepository) TopRecipes(context.Context, int) (any, error) {
	return nil, ErrNotImplemented
}
