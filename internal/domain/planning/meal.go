package planning

import (
	"time"

	eaterdom "github.com/ZetoOfficial/meal-planner/internal/domain/eater"
	mealdom "github.com/ZetoOfficial/meal-planner/internal/domain/meal"
	"github.com/ZetoOfficial/meal-planner/internal/domain/nutrition"
)

type Meal struct {
	EaterID     eaterdom.ID
	Date        time.Time
	Type        mealdom.Type
	FixedMealID *eaterdom.FixedMealID
	Planned     nutrition.Macros
	Components  []ComponentAllocation
}

type ComponentAllocation struct {
	// PlanRecipeIndex addresses the unsaved aggregate without inventing database IDs.
	PlanRecipeIndex int
	PortionFactor   float64
	PlannedServingG *float64
}
