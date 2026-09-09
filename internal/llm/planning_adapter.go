package llm

import (
	"context"

	recipedom "github.com/ZetoOfficial/meal-planner/internal/domain/recipe"
	usecaseplanning "github.com/ZetoOfficial/meal-planner/internal/usecase/planning"
)

// PlanningGenerator adapts the recipe generator to the planning use case port,
// keeping provider types out of the core.
type PlanningGenerator struct {
	generator *RecipeGenerator
}

func NewPlanningGenerator(generator *RecipeGenerator) *PlanningGenerator {
	return &PlanningGenerator{generator: generator}
}

func (a *PlanningGenerator) GenerateCandidates(
	ctx context.Context,
	request usecaseplanning.RecipeCandidateRequest,
) ([]recipedom.Template, error) {
	return a.generator.Generate(ctx, RecipeGeneratorInput{
		Days:       request.Days,
		Count:      request.Count,
		Novelty:    request.Novelty,
		MealTypes:  request.MealTypes,
		Equipment:  request.Equipment,
		Allergens:  request.Allergens,
		Exclusions: request.Exclusions,
		RoleTags:   request.RoleTags,
		Products:   request.Products,
	})
}
