package solver

import (
	"reflect"
	"testing"
	"time"

	catalogdom "github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
	eaterdom "github.com/ZetoOfficial/meal-planner/internal/domain/eater"
	"github.com/ZetoOfficial/meal-planner/internal/domain/nutrition"
	recipedom "github.com/ZetoOfficial/meal-planner/internal/domain/recipe"
)

func TestBuildPlanRanksSoftExcludedProductsLast(t *testing.T) {
	for _, excluded := range []catalogdom.ProductID{1, 2} {
		input := rankingInput(rankingProduct(1, withFiber(40)), rankingProduct(2, withFiber(40)))
		input.Eaters[0].Eater.Exclusions = []eaterdom.Exclusion{{
			TargetKind:  eaterdom.ExclusionTargetProduct,
			TargetValue: map[catalogdom.ProductID]string{1: "1", 2: "2"}[excluded],
			Severity:    eaterdom.ExclusionSeveritySoft,
		}}

		if got := selectedProduct(t, input); got == excluded {
			t.Fatalf("selected product = %d, want the one the eater did not softly exclude", got)
		}
	}
}

func TestBuildPlanKeepsASoftExcludedProductWhenItIsTheOnlyOption(t *testing.T) {
	input := rankingInput(rankingProduct(1, withFiber(40)))
	input.Eaters[0].Eater.Exclusions = []eaterdom.Exclusion{{
		TargetKind: eaterdom.ExclusionTargetProduct, TargetValue: "1", Severity: eaterdom.ExclusionSeveritySoft,
	}}

	if got := selectedProduct(t, input); got != 1 {
		t.Fatalf("selected product = %d, want 1: a soft exclusion lowers rank, it does not ban", got)
	}
}

func TestBuildPlanDoesNotLetASoftExclusionBuyWorseQuality(t *testing.T) {
	input := rankingInput(rankingProduct(1, withFiber(40)), rankingProduct(2, withFiber(1)))
	input.Eaters[0].Eater.Exclusions = []eaterdom.Exclusion{{
		TargetKind: eaterdom.ExclusionTargetProduct, TargetValue: "1", Severity: eaterdom.ExclusionSeveritySoft,
	}}

	if got := selectedProduct(t, input); got != 1 {
		t.Fatalf("selected product = %d, want 1: quality rules outrank level 5 preferences", got)
	}
}

func TestBuildPlanPrefersTheCheaperOfTwoEqualCandidates(t *testing.T) {
	for _, cheap := range []catalogdom.ProductID{1, 2} {
		prices := map[catalogdom.ProductID]float64{1: 9, 2: 9}
		prices[cheap] = 2
		input := rankingInput(
			rankingProduct(1, withFiber(40), withPrice(prices[1])),
			rankingProduct(2, withFiber(40), withPrice(prices[2])),
		)

		if got := selectedProduct(t, input); got != cheap {
			t.Fatalf("selected product = %d, want the cheaper product %d", got, cheap)
		}
	}
}

func TestBuildPlanPrefersABatchThatNeedsNoReheating(t *testing.T) {
	for _, reheated := range []recipedom.ID{1, 2} {
		input := twoRecipeInput()
		for i := range input.Recipes {
			if input.Recipes[i].ID == reheated {
				input.Recipes[i].Storage = recipedom.StorageKeeps3dReheat
			}
		}

		if got := selectedTemplate(t, input); got == reheated {
			t.Fatalf("selected template = %d, want the batch that keeps without reheating", got)
		}
	}
}

func TestBuildPlanSharesAnExistingBatchBetweenEaters(t *testing.T) {
	input := rankingInput(rankingProduct(2, withFiber(40)), rankingProduct(1, withFiber(40)))
	input.Eaters = append(input.Eaters, secondEater(input))
	// The first eater is steered to product 1; the second has no preference and
	// should take a portion of that batch instead of opening a new one.
	input.Eaters[0].Eater.Exclusions = []eaterdom.Exclusion{{
		TargetKind: eaterdom.ExclusionTargetProduct, TargetValue: "2", Severity: eaterdom.ExclusionSeveritySoft,
	}}

	if got := selectedProduct(t, input); got != 1 {
		t.Fatalf("selected product = %d, want both eaters on the single batch of product 1", got)
	}
}

func TestBuildPlanNoveltyZeroFollowsTheRotationWeight(t *testing.T) {
	input := twoRecipeInput()
	input.Recipes[1].Weight = 2

	if got := selectedTemplate(t, input); got != 2 {
		t.Fatalf("selected template = %d, want the heavier template 2", got)
	}
}

func TestBuildPlanNoveltyTwoBringsAComponentUnseenForTwoWeeks(t *testing.T) {
	input := twoRecipeInput()
	input.Recipes[0].Weight = 2 // the familiar favourite
	input.RecipeLastUsed = map[recipedom.ID]time.Time{
		1: input.StartDate.AddDate(0, 0, -2),
		2: input.StartDate.AddDate(0, 0, -20),
	}

	if got := selectedTemplate(t, input); got != 1 {
		t.Fatalf("novelty 0 selected template = %d, want the favourite 1", got)
	}
	input.Novelty = 2
	if got := selectedTemplate(t, input); got != 2 {
		t.Fatalf("novelty 2 selected template = %d, want template 2 unseen for 14+ days", got)
	}
}

func TestBuildPlanNoveltyTwoStillDeliversAnUnseenComponentThatCostsQuality(t *testing.T) {
	// Template 2 is the only stale one and its product has almost no fibre.
	input := twoRecipeInput()
	input.Products = append(input.Products, rankingProduct(2, withFiber(1), withRole("main:other")))
	input.Recipes[1].Slots[0].RoleTagCode = "main:other"
	input.RecipeLastUsed = map[recipedom.ID]time.Time{1: input.StartDate.AddDate(0, 0, -2)}

	if got := selectedTemplate(t, input); got != 1 {
		t.Fatalf("novelty 0 selected template = %d, want the better-quality template 1", got)
	}
	input.Novelty = 2
	if got := selectedTemplate(t, input); got != 2 {
		t.Fatalf("novelty 2 selected template = %d, want the unseen template 2 forced when no day hosts it for free", got)
	}
}

func TestBuildPlanNoveltyTwoFallsBackWhenNothingIsUnseen(t *testing.T) {
	input := twoRecipeInput()
	input.Novelty = 2
	input.RecipeLastUsed = map[recipedom.ID]time.Time{
		1: input.StartDate.AddDate(0, 0, -1),
		2: input.StartDate.AddDate(0, 0, -1),
	}

	result, err := New().BuildPlan(t.Context(), input)
	if err != nil || result.Failure != nil {
		t.Fatalf("BuildPlan() = %#v, %v, want a plan from the familiar library", result.Failure, err)
	}
}

func TestBuildPlanNoveltyThreeBuildsAroundTheFreshDraft(t *testing.T) {
	input := twoRecipeInput()
	input.Recipes[0].Weight = 2
	input.Novelty = 3
	input.FreshRecipes = []recipedom.ID{2}

	if got := selectedTemplate(t, input); got != 2 {
		t.Fatalf("selected template = %d, want the freshly generated draft 2", got)
	}
}

func TestBuildPlanNoveltyOneIsDeterministicAndNeverDrawsAZeroWeight(t *testing.T) {
	input := twoRecipeInput()
	input.Novelty = 1
	input.Recipes[0].Weight = 0

	first, err := New().BuildPlan(t.Context(), input)
	if err != nil {
		t.Fatalf("BuildPlan() error = %v", err)
	}
	second, err := New().BuildPlan(t.Context(), input)
	if err != nil {
		t.Fatalf("BuildPlan() error = %v", err)
	}
	if !reflect.DeepEqual(first.Plan, second.Plan) {
		t.Fatal("novelty 1 produced two different plans for one request")
	}
	if got := first.Plan.Recipes[0].TemplateID; got != 2 {
		t.Fatalf("selected template = %d, want 2: weight 0 must not win the draw", got)
	}
}

func TestShuffleKeysRotateBetweenRequests(t *testing.T) {
	input := twoRecipeInput()
	input.Novelty = 1

	winners := make(map[recipedom.ID]struct{})
	for week := range 12 {
		input.StartDate = input.StartDate.AddDate(0, 0, 7*week)
		keys := shuffleKeys(input)
		if keys[1] > keys[2] {
			winners[1] = struct{}{}
		} else {
			winners[2] = struct{}{}
		}
	}
	if len(winners) != 2 {
		t.Fatalf("draw winners over 12 weeks = %v, want both templates to win at least once", winners)
	}
}

func TestBuildPlanGivesEveryEaterTheirOwnBestDay(t *testing.T) {
	// Eater 2 is allergic to the high-fiber product, so their best day is
	// worse. Whatever the order, eater 1 must still get the better product:
	// one eater's ceiling is never lowered to match another's.
	for _, allergicFirst := range []bool{false, true} {
		input := rankingInput(
			rankingProduct(1, withFiber(40), withAllergen("casein")),
			rankingProduct(2, withFiber(1)),
		)
		second := secondEater(input)
		second.Eater.Allergens = []eaterdom.Allergen{"casein"}
		input.Eaters = append(input.Eaters, second)
		if allergicFirst {
			input.Eaters[0], input.Eaters[1] = input.Eaters[1], input.Eaters[0]
		}

		result, err := New().BuildPlan(t.Context(), input)
		if err != nil || result.Failure != nil {
			t.Fatalf("BuildPlan() = %#v, %v", result.Failure, err)
		}
		got := map[eaterdom.ID]catalogdom.ProductID{}
		for i := range result.Plan.Meals {
			meal := &result.Plan.Meals[i]
			got[meal.EaterID] = result.Plan.Recipes[meal.Components[0].PlanRecipeIndex].Fills[0].ProductID
		}
		if got[1] != 1 || got[2] != 2 {
			t.Fatalf("products by eater = %v, want eater 1 on product 1 and eater 2 on product 2", got)
		}
	}
}

// secondEater copies the first eater's schedule under a new identity.
func secondEater(input Input) EaterInput {
	second := input.Eaters[0]
	second.Eater.ID = 2
	second.Days = append([]EaterDayInput(nil), second.Days...)
	second.Days[0].Goal.EaterID = 2
	return second
}

func withPrice(pricePer100G float64) productOption {
	return func(product *catalogdom.Product) { product.PricePer100G = &pricePer100G }
}

func withAllergen(code string) productOption {
	return func(product *catalogdom.Product) {
		product.Allergens = []catalogdom.Allergen{catalogdom.Allergen(code)}
	}
}

// twoRecipeInput offers two interchangeable templates filled by one product,
// so only level 5 preferences and the novelty dial separate them.
func twoRecipeInput() Input {
	input := rankingInput(rankingProduct(1, withFiber(40)))
	input.Recipes[0].Weight = 1
	second := input.Recipes[0]
	second.ID = 2
	second.Slots = []recipedom.Slot{second.Slots[0]}
	second.Slots[0].ID = 2
	input.Recipes = append(input.Recipes, second)
	return input
}

func selectedTemplate(t *testing.T, input Input) recipedom.ID {
	t.Helper()
	result, err := New().BuildPlan(t.Context(), input)
	if err != nil {
		t.Fatalf("BuildPlan() error = %v", err)
	}
	if result.Failure != nil {
		t.Fatalf("BuildPlan() failure = %#v, want a ranked plan", result.Failure)
	}
	if len(result.Plan.Recipes) != 1 {
		t.Fatalf("plan recipes = %#v, want a single batch", result.Plan.Recipes)
	}
	return result.Plan.Recipes[0].TemplateID
}

func TestBuildPlanRecordsWhatAnAssembledDishIsBuiltFrom(t *testing.T) {
	// Only the cooked protein together with the fresh-assembled side hits the
	// exact goal, so both batches end up in the same lunch.
	input := rankingInput(
		catalogdom.Product{
			ID: 1, ProteinPer100G: 30, FatPer100G: 10, KcalPer100G: 210,
			Roles: []catalogdom.RoleTag{{Code: "protein:lean"}},
		},
		catalogdom.Product{
			ID: 2, CarbPer100G: 40, KcalPer100G: 160,
			Roles: []catalogdom.RoleTag{{Code: "carb:grain"}},
		},
	)
	input.Eaters[0].Days[0].Goal = exactGoal(input.StartDate, nutrition.Macros{ProteinG: 30, FatG: 10, CarbG: 40, Kcal: 370})
	cooked := input.Recipes[0]
	cooked.Slots = []recipedom.Slot{{ID: 1, RoleTagCode: "protein:lean", QtyG: 100}}
	assembled := input.Recipes[0]
	assembled.ID = 2
	assembled.Storage = recipedom.StorageAssembleFresh
	assembled.Slots = []recipedom.Slot{{ID: 2, RoleTagCode: "carb:grain", QtyG: 100}}
	input.Recipes = []recipedom.Template{cooked, assembled}

	result, err := New().BuildPlan(t.Context(), input)
	if err != nil || result.Failure != nil {
		t.Fatalf("BuildPlan() = %#v, %v", result.Failure, err)
	}

	if len(result.Plan.Recipes) != 2 || len(result.Plan.PrepDependencies) != 1 {
		t.Fatalf("plan = %d batches, dependencies %#v; want one edge between two batches",
			len(result.Plan.Recipes), result.Plan.PrepDependencies)
	}
	edge := result.Plan.PrepDependencies[0]
	dependent := result.Plan.Recipes[edge.DependentRecipeIndex].TemplateID
	source := result.Plan.Recipes[edge.SourceRecipeIndex].TemplateID
	if dependent != 2 || source != 1 {
		t.Fatalf("edge = template %d depends on %d, want the assembled dish 2 to depend on the cooked batch 1", dependent, source)
	}
}
