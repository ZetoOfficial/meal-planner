package solver

import (
	"reflect"
	"testing"
	"time"

	catalogdom "github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
	eaterdom "github.com/ZetoOfficial/meal-planner/internal/domain/eater"
	mealdom "github.com/ZetoOfficial/meal-planner/internal/domain/meal"
	"github.com/ZetoOfficial/meal-planner/internal/domain/nutrition"
	recipedom "github.com/ZetoOfficial/meal-planner/internal/domain/recipe"
)

func TestBuildPlanFindsMacroFeasibleThreeDayPlan(t *testing.T) {
	input := feasibleSearchInput()

	result, err := New().BuildPlan(t.Context(), input)
	if err != nil {
		t.Fatalf("BuildPlan() error = %v", err)
	}
	if result.Failure != nil {
		t.Fatalf("BuildPlan() failure = %#v", result.Failure)
	}
	if len(result.Plan.EaterDays) != 3 || len(result.Plan.Meals) != 6 || len(result.Plan.Recipes) != 1 {
		t.Fatalf("plan shape = %#v, want three eater-days, six meals, and one shared recipe", result.Plan)
	}

	planRecipe := result.Plan.Recipes[0]
	if planRecipe.ScaleFactor != 3 || len(planRecipe.Fills) != 1 || planRecipe.Fills[0].PlannedInputQtyG != 300 {
		t.Fatalf("aggregated plan recipe = %#v, want scale 3 and 300 g fill", planRecipe)
	}
	if planRecipe.ExpectedOutputG == nil || *planRecipe.ExpectedOutputG != 300 {
		t.Fatalf("expected output = %v, want 300 g", planRecipe.ExpectedOutputG)
	}

	for dayIndex := range input.Eaters[0].Days {
		date := input.Eaters[0].Days[dayIndex].Date
		total := macrosForDate(t, result, date)
		if !input.Eaters[0].Days[dayIndex].Goal.Range.Contains(total) {
			t.Fatalf("macros on %s = %#v, outside target %#v", date.Format(time.DateOnly), total, input.Eaters[0].Days[dayIndex].Goal.Range)
		}

		generated := result.Plan.Meals[dayIndex*2+1]
		if len(generated.Components) != 1 || generated.Components[0].PortionFactor != 1.0/3.0 {
			t.Fatalf("generated meal components = %#v, want one third of shared batch", generated.Components)
		}
		if generated.Components[0].PlannedServingG == nil || *generated.Components[0].PlannedServingG != 100 {
			t.Fatalf("planned serving = %v, want 100 g", generated.Components[0].PlannedServingG)
		}
	}
}

func TestBuildPlanIsDeterministicForOrderedInput(t *testing.T) {
	input := feasibleSearchInput()
	planner := New()

	first, err := planner.BuildPlan(t.Context(), input)
	if err != nil {
		t.Fatalf("first BuildPlan() error = %v", err)
	}
	second, err := planner.BuildPlan(t.Context(), input)
	if err != nil {
		t.Fatalf("second BuildPlan() error = %v", err)
	}

	if !reflect.DeepEqual(first, second) {
		t.Fatalf("repeated BuildPlan() differs:\nfirst:  %#v\nsecond: %#v", first, second)
	}
}

func TestBuildPlanReturnsTypedExhaustiveInfeasibility(t *testing.T) {
	input := feasibleSearchInput()
	for i := range input.Eaters[0].Days {
		input.Eaters[0].Days[i].Goal.Range = eaterdom.MacroRange{
			ProteinMinG: 100,
			ProteinMaxG: 120,
			FatMinG:     5,
			FatMaxG:     50,
			CarbMinG:    20,
			CarbMaxG:    150,
			KcalMin:     500,
			KcalMax:     1500,
		}
	}

	result, err := New().BuildPlan(t.Context(), input)
	if err != nil {
		t.Fatalf("BuildPlan() error = %v", err)
	}
	if result.Failure == nil || result.Failure.Code != FailureMacroInfeasible || !result.Failure.Exhaustive {
		t.Fatalf("BuildPlan() failure = %#v, want exhaustive macro infeasibility", result.Failure)
	}
	if result.Failure.EaterID != 1 || !result.Failure.Date.Equal(input.StartDate) {
		t.Fatalf("failure target = eater %d on %s, want eater 1 on first day", result.Failure.EaterID, result.Failure.Date)
	}
}

func TestBuildPlanDistinguishesSearchLimit(t *testing.T) {
	planner := NewWithConfig(Config{ServingMultipliers: []float64{0.5, 1, 1.5, 2}, SearchBudget: 1})

	result, err := planner.BuildPlan(t.Context(), feasibleSearchInput())
	if err != nil {
		t.Fatalf("BuildPlan() error = %v", err)
	}
	if result.Failure == nil || result.Failure.Code != FailureSearchLimitReached || result.Failure.Exhaustive {
		t.Fatalf("BuildPlan() failure = %#v, want non-exhaustive search limit", result.Failure)
	}
}

func feasibleSearchInput() Input {
	start := time.Date(2026, time.August, 10, 0, 0, 0, 0, time.UTC)
	goalRange := eaterdom.MacroRange{
		ProteinMinG: 40,
		ProteinMaxG: 40,
		FatMinG:     15,
		FatMaxG:     15,
		CarbMinG:    60,
		CarbMaxG:    60,
		KcalMin:     550,
		KcalMax:     550,
	}
	days := make([]EaterDayInput, 3)
	for i := range days {
		days[i] = EaterDayInput{
			Date: start.AddDate(0, 0, i),
			Goal: eaterdom.MacroGoal{
				ID:            eaterdom.MacroGoalID(i + 1),
				EaterID:       1,
				EffectiveFrom: start.AddDate(0, 0, i),
				Range:         goalRange,
			},
			GeneratedMealTypes: []mealdom.Type{mealdom.TypeLunch},
		}
	}
	outputG := 100.0
	return Input{
		StartDate:          start,
		Days:               3,
		CookDate:           start,
		AvailableEquipment: []string{"oven"},
		Eaters: []EaterInput{{
			Eater: eaterdom.Eater{ID: 1},
			Days:  days,
			FixedMeals: []eaterdom.FixedMeal{{
				ID:       1,
				EaterID:  1,
				MealType: mealdom.TypeBreakfast,
				Macros:   nutrition.Macros{ProteinG: 10, FatG: 5, CarbG: 20, Kcal: 150},
				Active:   true,
			}},
		}},
		Products: []catalogdom.Product{{
			ID:             1,
			ProteinPer100G: 30,
			FatPer100G:     10,
			CarbPer100G:    40,
			KcalPer100G:    400,
			Roles:          []catalogdom.RoleTag{{Code: "main:mixed"}},
		}},
		Recipes: []recipedom.Template{{
			ID:               1,
			Status:           recipedom.StatusPermanent,
			ComponentKind:    recipedom.ComponentKindMixedMain,
			AllowedMealTypes: []mealdom.Type{mealdom.TypeLunch},
			BaseServings:     1,
			ExpectedOutputG:  &outputG,
			Storage:          recipedom.StorageKeeps3d,
			Equipment:        []string{"oven"},
			Slots:            []recipedom.Slot{{ID: 1, RoleTagCode: "main:mixed", QtyG: 100}},
		}},
	}
}

func macrosForDate(t *testing.T, result Result, date time.Time) nutrition.Macros {
	t.Helper()
	total := nutrition.Macros{}
	for i := range result.Plan.Meals {
		if !result.Plan.Meals[i].Date.Equal(date) {
			continue
		}
		var err error
		total, err = total.Add(result.Plan.Meals[i].Planned)
		if err != nil {
			t.Fatalf("add macros on %s: %v", date.Format(time.DateOnly), err)
		}
	}
	return total
}
