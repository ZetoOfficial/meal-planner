package planning

import (
	"time"

	eaterdom "github.com/ZetoOfficial/meal-planner/internal/domain/eater"
	mealdom "github.com/ZetoOfficial/meal-planner/internal/domain/meal"
	recipedom "github.com/ZetoOfficial/meal-planner/internal/domain/recipe"
)

type PlanID int64

func (id PlanID) Valid() bool { return id > 0 }

type PlanStatus string

const (
	PlanStatusDraft    PlanStatus = "draft"
	PlanStatusActive   PlanStatus = "active"
	PlanStatusArchived PlanStatus = "archived"
)

func (s PlanStatus) Valid() bool {
	switch s {
	case PlanStatusDraft, PlanStatusActive, PlanStatusArchived:
		return true
	default:
		return false
	}
}

type Plan struct {
	ID        PlanID
	Label     *string
	Status    PlanStatus
	StartDate time.Time
	Days      int
	CookDate  time.Time
	Novelty   int
	// QualityScore and QualityWarningsCount snapshot the evaluation that ranked
	// the plan so a later profile change cannot rewrite its explanation.
	QualityScore         *float64
	QualityWarningsCount int
	ShoppingListStale    bool
	EaterDays            []EaterDay
	Recipes              []PlanRecipe
	Meals                []Meal
	PrepDependencies     []PrepDependency
}

type EaterDay struct {
	EaterID            eaterdom.ID
	Date               time.Time
	MacroGoalID        eaterdom.MacroGoalID
	GeneratedMealTypes []mealdom.Type
}

type PlanRecipeID int64

type PlanRecipe struct {
	ID              PlanRecipeID
	Label           *string
	TemplateID      recipedom.ID
	ScaleFactor     float64
	ExpectedOutputG *float64
	Fills           []SlotFill
}
