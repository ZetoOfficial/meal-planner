package solver

import (
	"testing"
	"time"

	catalogdom "github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
	eaterdom "github.com/ZetoOfficial/meal-planner/internal/domain/eater"
	mealdom "github.com/ZetoOfficial/meal-planner/internal/domain/meal"
	"github.com/ZetoOfficial/meal-planner/internal/domain/nutrition"
	recipedom "github.com/ZetoOfficial/meal-planner/internal/domain/recipe"
	"github.com/ZetoOfficial/meal-planner/internal/quality"
)

func TestBuildPlanPrefersStrongerDailyQualityRules(t *testing.T) {
	input := rankingInput(
		rankingProduct(1, withFiber(40)),
		rankingProduct(2, withFiber(1)),
	)

	if got := selectedProduct(t, input); got != 1 {
		t.Fatalf("selected product = %d, want 1 because it passes the fiber rule", got)
	}
}

func TestBuildPlanPrefersLowerRollingRedMeatProjection(t *testing.T) {
	input := rankingInput(
		rankingProduct(1, withFiber(40), withRedMeat(false)),
		rankingProduct(2, withFiber(40), withRedMeat(true)),
	)
	// Rolling coverage scales with known days, so the window needs enough
	// history for the weekly red meat rule to be evaluated at all.
	input.Eaters[0].History = redMeatHistory(t, 3, 100)
	ruleset := quality.DefaultRuleset()
	ruleset.RedMeatMaxWeeklyG = 300
	input.Ruleset = &ruleset

	if got := selectedProduct(t, input); got != 1 {
		t.Fatalf("selected product = %d, want 1 because rolling red meat stays under the weekly cap", got)
	}
}

func TestBuildPlanKeepsSoftQualityFailuresFeasible(t *testing.T) {
	input := rankingInput(rankingProduct(1, withFiber(1)))

	result, err := New().BuildPlan(t.Context(), input)
	if err != nil {
		t.Fatalf("BuildPlan() error = %v", err)
	}
	if result.Failure != nil {
		t.Fatalf("BuildPlan() failure = %#v, want a ranked plan despite the soft quality failure", result.Failure)
	}
	if len(result.Quality) != 1 {
		t.Fatalf("quality evaluations = %d, want one per eater-day", len(result.Quality))
	}
	if status := outcomeStatus(result.Quality[0].Evaluation, quality.RuleFiber); status != nutrition.StatusSoftFail {
		t.Fatalf("fiber outcome = %q, want soft_fail reported alongside the plan", status)
	}
}

func TestBuildPlanKeepsHardQualityViolationsInfeasible(t *testing.T) {
	hydrogenated := true
	product := rankingProduct(1, withFiber(40))
	product.Nutrition.ContainsPartiallyHydrogenatedOils = &hydrogenated

	result, err := New().BuildPlan(t.Context(), rankingInput(product))
	if err != nil {
		t.Fatalf("BuildPlan() error = %v", err)
	}
	if result.Failure == nil {
		t.Fatalf("BuildPlan() = %#v, want infeasibility for a hard quality violation", result.Plan)
	}
}

func TestBuildPlanComparesCoverageAfterQualityAndPreferences(t *testing.T) {
	input := twoSlotRankingInput(
		rankingProduct(20, withFiber(40)),
		rankingProduct(21, withFiberUnknown()),
	)

	if got := selectedSideProduct(t, input); got != 20 {
		t.Fatalf("selected side product = %d, want 20 because known facts raise coverage", got)
	}
}

func TestBuildPlanDoesNotCollapseQualityAndCoverageIntoOneScore(t *testing.T) {
	input := rankingInput(
		rankingProduct(1, withFiber(40)),
		rankingProduct(2, withFiber(1), withSodium(10)),
	)

	if got := selectedProduct(t, input); got != 1 {
		t.Fatalf("selected product = %d, want 1 because rule strength outranks coverage", got)
	}
}

func selectedProduct(t *testing.T, input Input) catalogdom.ProductID {
	t.Helper()
	result, err := New().BuildPlan(t.Context(), input)
	if err != nil {
		t.Fatalf("BuildPlan() error = %v", err)
	}
	if result.Failure != nil {
		t.Fatalf("BuildPlan() failure = %#v, want a ranked plan", result.Failure)
	}
	if len(result.Plan.Recipes) != 1 || len(result.Plan.Recipes[0].Fills) != 1 {
		t.Fatalf("plan recipes = %#v, want a single filled recipe", result.Plan.Recipes)
	}
	return result.Plan.Recipes[0].Fills[0].ProductID
}

func selectedSideProduct(t *testing.T, input Input) catalogdom.ProductID {
	t.Helper()
	result, err := New().BuildPlan(t.Context(), input)
	if err != nil {
		t.Fatalf("BuildPlan() error = %v", err)
	}
	if result.Failure != nil {
		t.Fatalf("BuildPlan() failure = %#v, want a ranked plan", result.Failure)
	}
	if len(result.Plan.Recipes) != 1 || len(result.Plan.Recipes[0].Fills) != 2 {
		t.Fatalf("plan recipes = %#v, want a single recipe with two fills", result.Plan.Recipes)
	}
	return result.Plan.Recipes[0].Fills[1].ProductID
}

func outcomeStatus(evaluation quality.Evaluation, code string) nutrition.RuleEvalStatus {
	for _, outcome := range evaluation.Outcomes {
		if outcome.Code == code {
			return outcome.Status
		}
	}
	return ""
}

type productOption func(*catalogdom.Product)

func withFiber(per100G float64) productOption {
	return func(product *catalogdom.Product) { product.Nutrition.FiberPer100G = &per100G }
}

func withFiberUnknown() productOption {
	return func(product *catalogdom.Product) { product.Nutrition.FiberPer100G = nil }
}

func withSodium(per100G float64) productOption {
	return func(product *catalogdom.Product) { product.Nutrition.SodiumMgPer100G = &per100G }
}

func withRedMeat(isRedMeat bool) productOption {
	return func(product *catalogdom.Product) { product.Classification.IsRedMeat = &isRedMeat }
}

func withRole(code string) productOption {
	return func(product *catalogdom.Product) {
		product.Roles = []catalogdom.RoleTag{{Code: code}}
	}
}

func rankingProduct(id catalogdom.ProductID, options ...productOption) catalogdom.Product {
	product := catalogdom.Product{
		ID:             id,
		ProteinPer100G: 30,
		FatPer100G:     10,
		CarbPer100G:    40,
		KcalPer100G:    400,
		Roles:          []catalogdom.RoleTag{{Code: "main:mixed"}},
	}
	for _, option := range options {
		option(&product)
	}
	return product
}

func redMeatHistory(t *testing.T, days int, redMeatGPerDay float64) []nutrition.DaySummary {
	t.Helper()
	coverage, err := nutrition.CoverageForInput(true, redMeatGPerDay)
	if err != nil {
		t.Fatalf("CoverageForInput() error = %v", err)
	}
	history := make([]nutrition.DaySummary, 0, days)
	for range days {
		history = append(history, nutrition.DaySummary{
			Basis:        nutrition.SummaryBasisPlanned,
			Totals:       nutrition.DailyTotals{RedMeatG: redMeatGPerDay},
			RuleCoverage: map[string]nutrition.Coverage{quality.RuleRedMeat: coverage},
		})
	}
	return history
}

// rankingInput keeps exactly one macro-feasible serving so ranking, not macro
// search, decides which candidate product ends up in the plan.
func rankingInput(products ...catalogdom.Product) Input {
	start := time.Date(2026, time.August, 10, 0, 0, 0, 0, time.UTC)
	preparedFactor := 1.0
	outputG := 100.0
	return Input{
		StartDate: start,
		Days:      1,
		CookDate:  start,
		Eaters: []EaterInput{{
			Eater: eaterdom.Eater{ID: 1, Sex: eaterdom.SexUnspecified},
			Days: []EaterDayInput{{
				Date:               start,
				Goal:               exactGoal(start, nutrition.Macros{ProteinG: 30, FatG: 10, CarbG: 40, Kcal: 400}),
				GeneratedMealTypes: []mealdom.Type{mealdom.TypeLunch},
			}},
		}},
		Products: products,
		Recipes: []recipedom.Template{{
			ID:               1,
			Status:           recipedom.StatusPermanent,
			ComponentKind:    recipedom.ComponentKindMixedMain,
			AllowedMealTypes: []mealdom.Type{mealdom.TypeLunch},
			BaseServings:     1,
			ExpectedOutputG:  &outputG,
			Storage:          recipedom.StorageKeeps3d,
			Slots: []recipedom.Slot{
				{ID: 1, RoleTagCode: "main:mixed", QtyG: 100, PreparedQtyFactor: &preparedFactor},
			},
		}},
	}
}

// twoSlotRankingInput fixes the protein slot and varies only the side slot so
// the compared candidates differ in per-rule coverage alone.
func twoSlotRankingInput(sides ...catalogdom.Product) Input {
	start := time.Date(2026, time.August, 10, 0, 0, 0, 0, time.UTC)
	preparedFactor := 1.0
	outputG := 200.0

	products := make([]catalogdom.Product, 0, 1+len(sides))
	products = append(products, rankingProduct(10, withFiber(40), withRole("main:protein")))
	for i := range sides {
		withRole("main:carb")(&sides[i])
		products = append(products, sides[i])
	}

	return Input{
		StartDate: start,
		Days:      1,
		CookDate:  start,
		Eaters: []EaterInput{{
			Eater: eaterdom.Eater{ID: 1, Sex: eaterdom.SexUnspecified},
			Days: []EaterDayInput{{
				Date:               start,
				Goal:               exactGoal(start, nutrition.Macros{ProteinG: 60, FatG: 20, CarbG: 80, Kcal: 800}),
				GeneratedMealTypes: []mealdom.Type{mealdom.TypeLunch},
			}},
		}},
		Products: products,
		Recipes: []recipedom.Template{{
			ID:               1,
			Status:           recipedom.StatusPermanent,
			ComponentKind:    recipedom.ComponentKindMixedMain,
			AllowedMealTypes: []mealdom.Type{mealdom.TypeLunch},
			BaseServings:     1,
			ExpectedOutputG:  &outputG,
			Storage:          recipedom.StorageKeeps3d,
			Slots: []recipedom.Slot{
				{ID: 1, RoleTagCode: "main:protein", QtyG: 100, PreparedQtyFactor: &preparedFactor},
				{ID: 2, RoleTagCode: "main:carb", QtyG: 100, PreparedQtyFactor: &preparedFactor, Position: 1},
			},
		}},
	}
}

func exactGoal(date time.Time, macros nutrition.Macros) eaterdom.MacroGoal {
	return eaterdom.MacroGoal{
		ID:            1,
		EaterID:       1,
		EffectiveFrom: date,
		Range: eaterdom.MacroRange{
			ProteinMinG: macros.ProteinG, ProteinMaxG: macros.ProteinG,
			FatMinG: macros.FatG, FatMaxG: macros.FatG,
			CarbMinG: macros.CarbG, CarbMaxG: macros.CarbG,
			KcalMin: macros.Kcal, KcalMax: macros.Kcal,
		},
	}
}
