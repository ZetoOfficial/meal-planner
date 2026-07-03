// Package repositories — адаптеры Postgres: маппинг sqlc rows ↔ domain.
// sqlc-типы НЕ должны протекать в domain/usecase/solver/quality (ARCHITECTURE §10).
package repositories

import (
	"context"

	recipedom "github.com/ZetoOfficial/meal-planner/internal/domain/recipe"
	"github.com/ZetoOfficial/meal-planner/internal/postgres/sqlc"
)

// RecipeRepository реализует соответствующий usecase-порт поверх sqlc.
type RecipeRepository struct {
	q sqlc.Querier
}

func NewRecipeRepository(q sqlc.Querier) *RecipeRepository {
	return &RecipeRepository{q: q}
}

func (r *RecipeRepository) ListPermanentCandidates(context.Context) ([]recipedom.Template, error) {
	return nil, ErrNotImplemented
}

func (r *RecipeRepository) SaveDraft(context.Context, recipedom.Template) error {
	return ErrNotImplemented
}

func (r *RecipeRepository) Promote(context.Context, recipedom.ID) error {
	return ErrNotImplemented
}
