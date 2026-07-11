package llm

import (
	"context"

	recipedom "github.com/ZetoOfficial/meal-planner/internal/domain/recipe"
)

type RecipeGenerator struct {
	client Client
}

func NewRecipeGenerator(c Client) *RecipeGenerator { return &RecipeGenerator{client: c} }

func (g *RecipeGenerator) Generate(_ context.Context, _ any) ([]recipedom.TemplateDraft, error) {
	return nil, ErrNotImplemented
}
