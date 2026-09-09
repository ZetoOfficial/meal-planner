package quality

import (
	"testing"
	"time"

	catalogdom "github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
	eaterdom "github.com/ZetoOfficial/meal-planner/internal/domain/eater"
	mealdom "github.com/ZetoOfficial/meal-planner/internal/domain/meal"
	"github.com/ZetoOfficial/meal-planner/internal/domain/nutrition"
	planningdom "github.com/ZetoOfficial/meal-planner/internal/domain/planning"
	recipedom "github.com/ZetoOfficial/meal-planner/internal/domain/recipe"
)

func TestBuildPlanDaysRebuildsGeneratedAndFixedMeals(t *testing.T) {
	days, err := BuildPlanDays(storedPlan(), storedCatalogue())
	if err != nil {
		t.Fatalf("BuildPlanDays() error = %v", err)
	}

	if len(days) != 1 {
		t.Fatalf("days = %d, want one eater-day", len(days))
	}
	day := days[0]
	if day.EaterID != 1 || !day.Date.Equal(planDate()) {
		t.Fatalf("day = %#v, want the stored eater-day key", day)
	}
	if len(day.Input.Meals) != 2 {
		t.Fatalf("meals = %d, want the fixed and generated meal", len(day.Input.Meals))
	}

	fixed := day.Input.Meals[0]
	if fixed.Fixed == nil || fixed.Fixed.Macros.Kcal != 300 || !fixed.Fixed.Quality.IncludeInWarnings {
		t.Fatalf("fixed meal = %#v, want the stored fixed meal quality", fixed)
	}
	generated := day.Input.Meals[1]
	if len(generated.Components) != 1 || generated.Components[0].PortionFactor != 0.5 {
		t.Fatalf("generated meal = %#v, want the stored allocation", generated)
	}
	slot := generated.Components[0].Slots[0]
	if slot.PlannedInputQtyG != 200 || slot.PreparedQtyFactor == nil || *slot.PreparedQtyFactor != 0.8 {
		t.Fatalf("slot = %#v, want the batch quantity and the template prepared factor", slot)
	}
	if slot.Facts.FiberPer100G == nil || *slot.Facts.FiberPer100G != 5 {
		t.Fatalf("slot facts = %#v, want the product nutrition facts", slot.Facts)
	}
}

func TestBuildPlanDaysProducesASummarisableDay(t *testing.T) {
	days, err := BuildPlanDays(storedPlan(), storedCatalogue())
	if err != nil {
		t.Fatalf("BuildPlanDays() error = %v", err)
	}

	summary, err := BuildDaySummary(days[0].Input, SummaryPurposeWarnings)
	if err != nil {
		t.Fatalf("BuildDaySummary() error = %v", err)
	}

	// 200 g batch at half a portion is 100 g of product with 5 g fibre per 100 g,
	// plus the 3 g the fixed meal contributes.
	if summary.Totals.FiberG != 8 {
		t.Fatalf("fibre = %v g, want 8 g from the batch share and the fixed meal", summary.Totals.FiberG)
	}
}

func TestBuildPlanDaysRejectsUnknownProducts(t *testing.T) {
	catalogue := storedCatalogue()
	delete(catalogue.Products, 4)

	if _, err := BuildPlanDays(storedPlan(), catalogue); err == nil {
		t.Fatal("BuildPlanDays() = nil, want an error for a product missing from the catalogue")
	}
}

func planDate() time.Time {
	return time.Date(2026, time.August, 10, 0, 0, 0, 0, time.UTC)
}

func storedPlan() planningdom.Plan {
	fixedID := eaterdom.FixedMealID(2)
	return planningdom.Plan{
		ID:        7,
		StartDate: planDate(),
		Days:      1,
		CookDate:  planDate(),
		EaterDays: []planningdom.EaterDay{{
			EaterID: 1, Date: planDate(), MacroGoalID: 3,
			GeneratedMealTypes: []mealdom.Type{mealdom.TypeLunch},
		}},
		Recipes: []planningdom.PlanRecipe{{
			ID: 11, TemplateID: 5, ScaleFactor: 2,
			Fills: []planningdom.SlotFill{{SlotID: 9, ProductID: 4, PlannedInputQtyG: 200}},
		}},
		Meals: []planningdom.Meal{
			{
				ID: 20, EaterID: 1, Date: planDate(), Type: mealdom.TypeBreakfast,
				FixedMealID: &fixedID,
				Planned:     nutrition.Macros{ProteinG: 20, FatG: 8, CarbG: 30, Kcal: 300},
			},
			{
				ID: 21, EaterID: 1, Date: planDate(), Type: mealdom.TypeLunch,
				Planned:    nutrition.Macros{ProteinG: 30, FatG: 10, CarbG: 40, Kcal: 400},
				Components: []planningdom.ComponentAllocation{{PlanRecipeIndex: 0, PortionFactor: 0.5}},
			},
		},
	}
}

func storedCatalogue() Catalogue {
	fiber := 5.0
	preparedFactor := 0.8
	fixedFiber := 3.0
	return Catalogue{
		Products: map[catalogdom.ProductID]catalogdom.Product{
			4: {
				ID: 4, Name: "Chicken breast",
				ProteinPer100G: 23, FatPer100G: 2, CarbPer100G: 0, KcalPer100G: 110,
				Nutrition: catalogdom.NutritionFacts{FiberPer100G: &fiber},
			},
		},
		Templates: map[recipedom.ID]recipedom.Template{
			5: {ID: 5, Slots: []recipedom.Slot{{ID: 9, RoleTagCode: "protein:lean", QtyG: 100, PreparedQtyFactor: &preparedFactor}}},
		},
		Eaters: map[eaterdom.ID]eaterdom.Eater{
			1: {ID: 1, Sex: eaterdom.SexFemale},
		},
		FixedMeals: map[eaterdom.FixedMealID]eaterdom.FixedMeal{
			2: {
				ID: 2, EaterID: 1, MealType: mealdom.TypeBreakfast,
				Macros:  nutrition.Macros{ProteinG: 20, FatG: 8, CarbG: 30, Kcal: 300},
				Quality: eaterdom.FixedMealQuality{FiberG: &fixedFiber, IncludeInWarnings: true, IncludeInAnalytics: true},
				Active:  true,
			},
		},
	}
}

func TestBuildPlanDaysForActualBasisSkipsSkippedMeals(t *testing.T) {
	plan := storedPlan()
	plan.Meals[1].LogStatus = planningdom.MealLogStatusSkipped

	days, err := BuildPlanDaysForBasis(plan, storedCatalogue(), nutrition.SummaryBasisActual)
	if err != nil {
		t.Fatalf("BuildPlanDaysForBasis() error = %v", err)
	}

	if len(days[0].Input.Meals) != 1 {
		t.Fatalf("meals = %d, want the skipped meal left out of the actual day", len(days[0].Input.Meals))
	}
}

func TestBuildPlanDaysForActualBasisLowersCoverageForModifiedMeals(t *testing.T) {
	plan := storedPlan()
	actual := nutrition.Macros{ProteinG: 25, FatG: 12, CarbG: 35, Kcal: 380}
	plan.Meals[1].LogStatus = planningdom.MealLogStatusEatenModified
	plan.Meals[1].Actual = &actual

	days, err := BuildPlanDaysForBasis(plan, storedCatalogue(), nutrition.SummaryBasisActual)
	if err != nil {
		t.Fatalf("BuildPlanDaysForBasis() error = %v", err)
	}
	summary, err := BuildDaySummary(days[0].Input, SummaryPurposeAnalytics)
	if err != nil {
		t.Fatalf("BuildDaySummary() error = %v", err)
	}

	if summary.Macros.Kcal != 680 {
		t.Fatalf("kcal = %d, want the logged macros counted alongside the fixed meal", summary.Macros.Kcal)
	}
	if got := summary.RuleCoverage[RuleFiber].Score(); got >= 1 {
		t.Fatalf("fibre coverage = %v, want the unknown meal to lower confidence", got)
	}
	if summary.Totals.FiberG != 3 {
		t.Fatalf("fibre = %v g, want only the fixed meal's known fibre", summary.Totals.FiberG)
	}
}

func TestBuildPlanDaysForActualBasisKeepsMealsEatenAsPlanned(t *testing.T) {
	plan := storedPlan()
	plan.Meals[1].LogStatus = planningdom.MealLogStatusEatenAsPlanned

	days, err := BuildPlanDaysForBasis(plan, storedCatalogue(), nutrition.SummaryBasisActual)
	if err != nil {
		t.Fatalf("BuildPlanDaysForBasis() error = %v", err)
	}
	summary, err := BuildDaySummary(days[0].Input, SummaryPurposeAnalytics)
	if err != nil {
		t.Fatalf("BuildDaySummary() error = %v", err)
	}

	if summary.Totals.FiberG != 8 {
		t.Fatalf("fibre = %v g, want the planned structured facts reused", summary.Totals.FiberG)
	}
}
