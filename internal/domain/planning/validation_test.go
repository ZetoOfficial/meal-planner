package planning

import (
	"testing"
	"time"

	eaterdom "github.com/ZetoOfficial/meal-planner/internal/domain/eater"
	mealdom "github.com/ZetoOfficial/meal-planner/internal/domain/meal"
	"github.com/ZetoOfficial/meal-planner/internal/domain/nutrition"
)

func TestValidateAcceptsAGeneratedPlan(t *testing.T) {
	if err := validPlan().Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want a valid plan", err)
	}
}

func TestValidateRejectsGeneratedMealWithoutAllocation(t *testing.T) {
	plan := validPlan()
	plan.Meals[1].Components = nil

	if err := plan.Validate(); err == nil {
		t.Fatal("Validate() = nil, want an error for a generated meal without component allocations")
	}
}

func TestValidateRejectsFixedMealWithAllocation(t *testing.T) {
	plan := validPlan()
	plan.Meals[0].Components = []ComponentAllocation{{PlanRecipeIndex: 0, PortionFactor: 0.1}}

	if err := plan.Validate(); err == nil {
		t.Fatal("Validate() = nil, want an error for a fixed meal carrying allocations")
	}
}

func TestValidateRejectsAllocationOutsideThePlan(t *testing.T) {
	plan := validPlan()
	plan.Meals[1].Components[0].PlanRecipeIndex = 7

	if err := plan.Validate(); err == nil {
		t.Fatal("Validate() = nil, want an error for an allocation pointing outside the plan")
	}
}

func TestValidateRejectsOverAllocatedBatch(t *testing.T) {
	plan := validPlan()
	plan.Meals[1].Components[0].PortionFactor = 0.7
	plan.Meals[2].Components[0].PortionFactor = 0.7

	if err := plan.Validate(); err == nil {
		t.Fatal("Validate() = nil, want an error when portion factors exceed the cooked batch")
	}
}

func TestValidateAcceptsBatchReserve(t *testing.T) {
	plan := validPlan()
	plan.Meals[1].Components[0].PortionFactor = 0.4
	plan.Meals[2].Components[0].PortionFactor = 0.4

	if err := plan.Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want an unallocated batch remainder to be allowed", err)
	}
}

func TestValidateRejectsMealOutsideTheEaterDays(t *testing.T) {
	plan := validPlan()
	plan.Meals[1].Date = plan.StartDate.AddDate(0, 0, 5)

	if err := plan.Validate(); err == nil {
		t.Fatal("Validate() = nil, want an error for a meal without a matching plan eater day")
	}
}

func validPlan() Plan {
	start := time.Date(2026, time.August, 10, 0, 0, 0, 0, time.UTC)
	fixedID := eaterdom.FixedMealID(1)
	macros := nutrition.Macros{ProteinG: 30, FatG: 10, CarbG: 40, Kcal: 400}
	return Plan{
		StartDate: start,
		Days:      2,
		CookDate:  start,
		Status:    PlanStatusDraft,
		EaterDays: []EaterDay{
			{EaterID: 1, Date: start, MacroGoalID: 1, GeneratedMealTypes: []mealdom.Type{mealdom.TypeLunch}},
			{EaterID: 1, Date: start.AddDate(0, 0, 1), MacroGoalID: 1, GeneratedMealTypes: []mealdom.Type{mealdom.TypeLunch}},
		},
		Recipes: []PlanRecipe{{
			TemplateID:  1,
			ScaleFactor: 2,
			Fills:       []SlotFill{{SlotID: 1, ProductID: 1, PlannedInputQtyG: 200}},
		}},
		Meals: []Meal{
			{EaterID: 1, Date: start, Type: mealdom.TypeBreakfast, FixedMealID: &fixedID, Planned: macros},
			{
				EaterID:    1,
				Date:       start,
				Type:       mealdom.TypeLunch,
				Planned:    macros,
				Components: []ComponentAllocation{{PlanRecipeIndex: 0, PortionFactor: 0.5}},
			},
			{
				EaterID:    1,
				Date:       start.AddDate(0, 0, 1),
				Type:       mealdom.TypeLunch,
				Planned:    macros,
				Components: []ComponentAllocation{{PlanRecipeIndex: 0, PortionFactor: 0.5}},
			},
		},
	}
}
