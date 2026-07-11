package solver

import (
	"context"

	catalogdom "github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
	eaterdom "github.com/ZetoOfficial/meal-planner/internal/domain/eater"
	recipedom "github.com/ZetoOfficial/meal-planner/internal/domain/recipe"
)

type Input struct {
	Eaters   []eaterdom.Eater
	Products []catalogdom.Product
	Recipes  []recipedom.Template
	Novelty  int
}

type Planner struct{}

func New() *Planner { return &Planner{} }

func (p *Planner) BuildPlan(_ context.Context, _ Input) (Result, error) { return Result{}, nil }
