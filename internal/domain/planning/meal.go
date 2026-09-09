package planning

import (
	"errors"
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
	// Actual is filled in only when the meal was eaten differently. NULL means
	// analytics use the planned values (FR-6).
	Actual     *nutrition.Macros
	CookedBy   *eaterdom.ID
	CookedAt   *time.Time
	LoggedBy   *eaterdom.ID
	LoggedAt   *time.Time
	Components []ComponentAllocation
}

// EffectiveMacros is what the eater actually got: the logged macros when the
// meal deviated, the planned ones otherwise.
func (m Meal) EffectiveMacros() nutrition.Macros {
	if m.Actual != nil {
		return *m.Actual
	}
	return m.Planned
}

// Eaten reports whether the meal counts towards actual analytics at all.
func (m Meal) Eaten() bool {
	return m.LogStatus != MealLogStatusSkipped
}

// QualityKnown is false for a meal eaten differently than planned: the owner
// enters macros by hand, never quality nutrients, so its quality data is
// unknown rather than assumed to match the plan (FR-6).
func (m Meal) QualityKnown() bool {
	return m.LogStatus != MealLogStatusEatenModified
}

type ComponentAllocation struct {
	// PlanRecipeIndex addresses the unsaved aggregate without inventing database IDs.
	PlanRecipeIndex int
	PortionFactor   float64
	PlannedServingG *float64
}

// MealLog is what the owner reports after eating. Actual macros are entered by
// hand only when the meal deviated; quality nutrients never are.
type MealLog struct {
	Status   MealLogStatus
	LoggedBy eaterdom.ID
	CookedBy *eaterdom.ID
	Actual   *nutrition.Macros
	At       time.Time
}

// Validate keeps the log honest: only a modified meal carries actual macros,
// and a modified meal without them would claim a deviation it cannot describe.
func (l MealLog) Validate() error {
	if !l.Status.Valid() || l.Status == MealLogStatusPlanned {
		return errors.New("meal log status must be eaten_as_planned, eaten_modified or skipped")
	}
	if !l.LoggedBy.Valid() {
		return errors.New("meal log needs the eater who logged it")
	}
	if l.Status == MealLogStatusEatenModified && l.Actual == nil {
		return errors.New("a modified meal needs its actual macros")
	}
	if l.Status != MealLogStatusEatenModified && l.Actual != nil {
		return errors.New("actual macros belong to a modified meal only")
	}
	if l.Actual != nil && !l.Actual.Valid() {
		return errors.New("actual macros must be finite and nonnegative")
	}
	return nil
}
