package planning

import (
	"time"

	eaterdom "github.com/ZetoOfficial/meal-planner/internal/domain/eater"
	mealdom "github.com/ZetoOfficial/meal-planner/internal/domain/meal"
	recipedom "github.com/ZetoOfficial/meal-planner/internal/domain/recipe"
)

type PlanID int64

type Plan struct {
	ID        PlanID
	StartDate time.Time
	Days      int
	CookDate  time.Time
	Novelty   int
	EaterDays []EaterDay
	Recipes   []PlanRecipe
	Meals     []Meal
}

type EaterDay struct {
	EaterID            eaterdom.ID
	Date               time.Time
	MacroGoalID        eaterdom.MacroGoalID
	GeneratedMealTypes []mealdom.Type
}

type PlanRecipe struct {
	TemplateID      recipedom.ID
	ScaleFactor     float64
	ExpectedOutputG *float64
	Fills           []SlotFill
}
