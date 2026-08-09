package solver

import (
	"context"
	"time"

	catalogdom "github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
	eaterdom "github.com/ZetoOfficial/meal-planner/internal/domain/eater"
	mealdom "github.com/ZetoOfficial/meal-planner/internal/domain/meal"
	recipedom "github.com/ZetoOfficial/meal-planner/internal/domain/recipe"
)

type EaterDayInput struct {
	Date               time.Time
	Goal               eaterdom.MacroGoal
	GeneratedMealTypes []mealdom.Type
}

type EaterInput struct {
	Eater      eaterdom.Eater
	Days       []EaterDayInput
	FixedMeals []eaterdom.FixedMeal
}

type Input struct {
	StartDate          time.Time
	Days               int
	CookDate           time.Time
	Novelty            int
	AvailableEquipment []string
	Eaters             []EaterInput
	Products           []catalogdom.Product
	Recipes            []recipedom.Template
}

type Planner struct{}

func New() *Planner { return &Planner{} }

func (p *Planner) BuildPlan(_ context.Context, _ Input) (Result, error) { return Result{}, nil }
