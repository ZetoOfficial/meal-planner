// Package recipe (usecase) — сценарии библиотеки рецептов и повышения draft→permanent.
package recipe

import (
	"context"

	recipedom "github.com/ZetoOfficial/meal-planner/internal/domain/recipe"
)

// RecipeRepository — порт доступа к шаблонам рецептов.
type RecipeRepository interface {
	ListPermanentCandidates(ctx context.Context) ([]recipedom.Template, error)
	SaveDraft(ctx context.Context, r recipedom.Template) error
	Promote(ctx context.Context, id recipedom.ID) error
}
