package planning

import (
	"context"

	catalogdom "github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
	mealdom "github.com/ZetoOfficial/meal-planner/internal/domain/meal"
	recipedom "github.com/ZetoOfficial/meal-planner/internal/domain/recipe"
)

// RecipeCandidateRequest is everything a generator may see. The planning core
// owns this shape so no provider type leaks into the use case.
type RecipeCandidateRequest struct {
	Days       int
	Count      int
	Novelty    int
	MealTypes  []mealdom.Type
	Equipment  []string
	Allergens  []string
	Exclusions []string
	RoleTags   []catalogdom.RoleTag
	Products   []catalogdom.Product
}

// RecipeGenerator proposes new recipe candidates. It only proposes: Go
// validates every candidate and the solver decides whether to use one.
type RecipeGenerator interface {
	GenerateCandidates(ctx context.Context, request RecipeCandidateRequest) ([]recipedom.Template, error)
}
