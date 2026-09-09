package planning

import (
	"time"

	eaterdom "github.com/ZetoOfficial/meal-planner/internal/domain/eater"
	mealdom "github.com/ZetoOfficial/meal-planner/internal/domain/meal"
	"github.com/ZetoOfficial/meal-planner/internal/domain/nutrition"
)

type MealID int64

type MealLogStatus string

const (
	MealLogStatusPlanned        MealLogStatus = "planned"
	MealLogStatusEatenAsPlanned MealLogStatus = "eaten_as_planned"
	MealLogStatusEatenModified  MealLogStatus = "eaten_modified"
	MealLogStatusSkipped        MealLogStatus = "skipped"
)

func (s MealLogStatus) Valid() bool {
	switch s {
	case MealLogStatusPlanned, MealLogStatusEatenAsPlanned, MealLogStatusEatenModified, MealLogStatusSkipped:
		return true
	default:
		return false
	}
}

type Meal struct {
	ID          MealID
	LogStatus   MealLogStatus
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
