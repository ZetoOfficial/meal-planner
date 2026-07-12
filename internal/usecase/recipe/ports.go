package recipe

import (
	"context"

	recipedom "github.com/ZetoOfficial/meal-planner/internal/domain/recipe"
)

type RecipeRepository interface {
	ListPermanentCandidates(ctx context.Context) ([]recipedom.Template, error)
	SaveDraft(ctx context.Context, r recipedom.Template) error
	Promote(ctx context.Context, id recipedom.ID) error
}
