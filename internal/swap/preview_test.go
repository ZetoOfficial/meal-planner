package swap

import (
	"errors"
	"testing"
	"time"

	catalogdom "github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
	eaterdom "github.com/ZetoOfficial/meal-planner/internal/domain/eater"
	mealdom "github.com/ZetoOfficial/meal-planner/internal/domain/meal"
	"github.com/ZetoOfficial/meal-planner/internal/domain/nutrition"
	planningdom "github.com/ZetoOfficial/meal-planner/internal/domain/planning"
	recipedom "github.com/ZetoOfficial/meal-planner/internal/domain/recipe"
	"github.com/ZetoOfficial/meal-planner/internal/quality"
)

func TestComputeDoesNotMutateThePlan(t *testing.T) {
	input := swapInput()

	if _, err := Compute(input, Request{SlotFillID: 1, NewProductID: 2}); err != nil {
		t.Fatalf("Compute() error = %v", err)
	}

	if got := input.Plan.Recipes[0].Fills[0].ProductID; got != 1 {
		t.Fatalf("plan fill product = %d, want the preview to leave the plan untouched", got)
	}
}

func TestComputeReportsANoOpLikeSwapAsImmaterial(t *testing.T) {
	input := swapInput()

	preview, err := Compute(input, Request{SlotFillID: 1, NewProductID: 2})
	if err != nil {
		t.Fatalf("Compute() error = %v", err)
	}

	for _, delta := range preview.MacroDiff {
		if absolute(delta.Delta.ProteinG) > materialMacroDelta {
			t.Fatalf("macro delta = %#v, want a near-identical swap", delta)
		}
	}
	if len(preview.QualityDiff.ChangedRules) != 0 {
		t.Fatalf("changed rules = %#v, want none for a like-for-like swap", preview.QualityDiff.ChangedRules)
	}
	if preview.Summary == "" {
		t.Fatal("summary is empty, want a readable explanation")
	}
}

func TestComputeReportsAMateriallyDifferentSwap(t *testing.T) {
	input := swapInput()

	preview, err := Compute(input, Request{SlotFillID: 1, NewProductID: 3})
	if err != nil {
		t.Fatalf("Compute() error = %v", err)
	}

	if len(preview.MacroDiff) != 1 {
		t.Fatalf("macro diff = %#v, want one eater-day", preview.MacroDiff)
	}
	// The replacement carries 10 g more protein per 100 g over a 200 g batch
	// eaten at half a portion.
	if got := preview.MacroDiff[0].Delta.ProteinG; got < 9.9 || got > 10.1 {
		t.Fatalf("protein delta = %v g, want about 10 g", got)
	}
	if preview.QualityDiff.FiberG >= 0 {
		t.Fatalf("fibre delta = %v, want the swap to lose fibre", preview.QualityDiff.FiberG)
	}
	if preview.QualityDiff.CoverageScore == 0 {
		t.Fatalf("coverage delta = %v, want the missing facts to lower coverage", preview.QualityDiff.CoverageScore)
	}
	if len(preview.AffectedMeals) != 1 || preview.AffectedMeals[0] != 21 {
		t.Fatalf("affected meals = %#v, want the meal eating from the batch", preview.AffectedMeals)
	}
	if preview.Level != LevelProteinChange {
		t.Fatalf("level = %q, want a protein change", preview.Level)
	}
}

func TestComputeReportsChangedRuleVerdicts(t *testing.T) {
	input := swapInput()

	preview, err := Compute(input, Request{SlotFillID: 1, NewProductID: 4})
	if err != nil {
		t.Fatalf("Compute() error = %v", err)
	}

	if len(preview.QualityDiff.ChangedRules) == 0 {
		t.Fatalf("changed rules = %#v, want the fibre verdict to move", preview.QualityDiff.ChangedRules)
	}
	change := preview.QualityDiff.ChangedRules[0]
	if change.Before.Status == change.After.Status {
		t.Fatalf("rule change = %#v, want a different verdict on each side", change)
	}
}

func TestComputeReportsShoppingDelta(t *testing.T) {
	input := swapInput()

	preview, err := Compute(input, Request{SlotFillID: 1, NewProductID: 3})
	if err != nil {
		t.Fatalf("Compute() error = %v", err)
	}

	if len(preview.ShoppingDelta.Added) != 1 || preview.ShoppingDelta.Added[0] != 3 {
		t.Fatalf("added products = %#v, want the replacement", preview.ShoppingDelta.Added)
	}
	if len(preview.ShoppingDelta.Removed) != 1 || preview.ShoppingDelta.Removed[0] != 1 {
		t.Fatalf("removed products = %#v, want the replaced product", preview.ShoppingDelta.Removed)
	}
}

func TestComputeKeepsAProductStillUsedElsewhereOffTheShoppingDelta(t *testing.T) {
	input := swapInput()
	input.Plan.Recipes = append(input.Plan.Recipes, planningdom.PlanRecipe{
		ID: 12, TemplateID: 5, ScaleFactor: 1,
		Fills: []planningdom.SlotFill{{ID: 2, SlotID: 9, ProductID: 1, PlannedInputQtyG: 100}},
	})

	preview, err := Compute(input, Request{SlotFillID: 1, NewProductID: 3})
	if err != nil {
		t.Fatalf("Compute() error = %v", err)
	}

	if len(preview.ShoppingDelta.Removed) != 0 {
		t.Fatalf("removed products = %#v, want none while another fill still uses it", preview.ShoppingDelta.Removed)
	}
}

func TestComputeReportsInvalidatedDependentDishes(t *testing.T) {
	input := swapInput()
	note := "uses yesterday's roast chicken"
	input.Plan.Recipes = append(input.Plan.Recipes,
		planningdom.PlanRecipe{
			ID: 12, TemplateID: 5, ScaleFactor: 1,
			Fills: []planningdom.SlotFill{{ID: 2, SlotID: 9, ProductID: 2, PlannedInputQtyG: 100}},
		},
		planningdom.PlanRecipe{
			ID: 13, TemplateID: 5, ScaleFactor: 1,
			Fills: []planningdom.SlotFill{{ID: 3, SlotID: 9, ProductID: 2, PlannedInputQtyG: 100}},
		},
	)
	input.Plan.PrepDependencies = []planningdom.PrepDependency{
		{DependentRecipeID: 12, SourceRecipeID: 11, Note: &note},
		{DependentRecipeID: 13, SourceRecipeID: 12},
	}

	preview, err := Compute(input, Request{SlotFillID: 1, NewProductID: 3})
	if err != nil {
		t.Fatalf("Compute() error = %v", err)
	}

	if len(preview.InvalidatedRecipes) != 2 {
		t.Fatalf("invalidated = %#v, want the whole dependency chain", preview.InvalidatedRecipes)
	}
	if preview.InvalidatedRecipes[0].Reason != note {
		t.Fatalf("reason = %q, want the stored dependency note", preview.InvalidatedRecipes[0].Reason)
	}
}

func TestComputeRejectsAnUnknownSlotFill(t *testing.T) {
	_, err := Compute(swapInput(), Request{SlotFillID: 99, NewProductID: 2})

	if !errors.Is(err, errUnknownFill) {
		t.Fatalf("Compute() error = %v, want an unknown slot fill error", err)
	}
}

func TestComputeRejectsAnUnknownProduct(t *testing.T) {
	if _, err := Compute(swapInput(), Request{SlotFillID: 1, NewProductID: 99}); err == nil {
		t.Fatal("Compute() = nil, want an error for a product outside the catalogue")
	}
}

func swapDate() time.Time {
	return time.Date(2026, time.August, 10, 0, 0, 0, 0, time.UTC)
}

// swapInput holds one batch eaten at half a portion, so a product change of
// x per 100 g moves the day by x grams.
func swapInput() Input {
	return Input{
		Plan: planningdom.Plan{
			ID:        7,
			StartDate: swapDate(),
			Days:      1,
			CookDate:  swapDate(),
			EaterDays: []planningdom.EaterDay{{
				EaterID: 1, Date: swapDate(), MacroGoalID: 3,
				GeneratedMealTypes: []mealdom.Type{mealdom.TypeLunch},
			}},
			Recipes: []planningdom.PlanRecipe{{
				ID: 11, TemplateID: 5, ScaleFactor: 2,
				Fills: []planningdom.SlotFill{{ID: 1, SlotID: 9, ProductID: 1, PlannedInputQtyG: 200}},
			}},
			Meals: []planningdom.Meal{{
				ID: 21, EaterID: 1, Date: swapDate(), Type: mealdom.TypeLunch,
				Planned:    nutrition.Macros{ProteinG: 46, FatG: 4, CarbG: 0, Kcal: 220},
				Components: []planningdom.ComponentAllocation{{PlanRecipeIndex: 0, PortionFactor: 0.5}},
			}},
		},
		Catalogue:         swapCatalogue(),
		Ruleset:           quality.DefaultRuleset(),
		RollingWindowDays: 7,
	}
}

func swapCatalogue() quality.Catalogue {
	preparedFactor := 1.0
	highFiber := 30.0
	lowFiber := 1.0
	return quality.Catalogue{
		Products: map[catalogdom.ProductID]catalogdom.Product{
			1: fishProduct(1, "Salmon", 23, 12, &highFiber),
			2: fishProduct(2, "Cod", 23, 12, &highFiber),
			3: fishProduct(3, "Tuna", 33, 12, nil),
			4: fishProduct(4, "White bread", 23, 12, &lowFiber),
		},
		Templates: map[recipedom.ID]recipedom.Template{
			5: {ID: 5, Slots: []recipedom.Slot{{
				ID: 9, RoleTagCode: "protein:lean", QtyG: 100, PreparedQtyFactor: &preparedFactor,
			}}},
		},
		Eaters:     map[eaterdom.ID]eaterdom.Eater{1: {ID: 1, Sex: eaterdom.SexFemale}},
		FixedMeals: map[eaterdom.FixedMealID]eaterdom.FixedMeal{},
	}
}

func fishProduct(id catalogdom.ProductID, name string, protein, fat float64, fiber *float64) catalogdom.Product {
	return catalogdom.Product{
		ID: id, Name: name,
		ProteinPer100G: protein, FatPer100G: fat, CarbPer100G: 0, KcalPer100G: 200,
		Nutrition: catalogdom.NutritionFacts{FiberPer100G: fiber},
		Roles:     []catalogdom.RoleTag{{Code: "protein:lean", Kind: catalogdom.RoleKindProtein}},
	}
}
