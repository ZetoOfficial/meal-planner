package swap

import (
	"errors"
	"strings"
	"testing"

	catalogdom "github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
	eaterdom "github.com/ZetoOfficial/meal-planner/internal/domain/eater"
	mealdom "github.com/ZetoOfficial/meal-planner/internal/domain/meal"
	planningdom "github.com/ZetoOfficial/meal-planner/internal/domain/planning"
	recipedom "github.com/ZetoOfficial/meal-planner/internal/domain/recipe"
)

// recipeSwapInput extends the fill fixture with a second template: a grain
// side for four servings that the lunch batch can be swapped to.
func recipeSwapInput() Input {
	input := swapInput()
	current := input.Catalogue.Templates[5]
	current.Name = "Baked fish"
	current.BaseServings = 2
	current.AllowedMealTypes = []mealdom.Type{mealdom.TypeLunch}
	current.Storage = recipedom.StorageKeeps3d
	input.Catalogue.Templates[5] = current

	output := 600.0
	preparedFactor := 2.5
	input.Catalogue.Templates[6] = recipedom.Template{
		ID: 6, Name: "Rice", Status: recipedom.StatusPermanent, BaseServings: 4, ExpectedOutputG: &output,
		AllowedMealTypes: []mealdom.Type{mealdom.TypeLunch, mealdom.TypeDinner},
		Storage:          recipedom.StorageKeeps3d,
		Equipment:        []string{"stove"},
		Slots: []recipedom.Slot{
			{ID: 20, RoleTagCode: "carb:grain", QtyG: 240, PreparedQtyFactor: &preparedFactor},
			{ID: 21, RoleTagCode: "fat:cooking", QtyG: 10, Optional: true},
		},
	}
	input.Catalogue.Products[10] = catalogdom.Product{
		ID: 10, Name: "Brown rice", ProteinPer100G: 8, FatPer100G: 2, CarbPer100G: 76, KcalPer100G: 350,
		Roles: []catalogdom.RoleTag{{Code: "carb:grain", Kind: catalogdom.RoleKindCarb}},
	}
	input.Catalogue.Products[11] = catalogdom.Product{
		ID: 11, Name: "Peanut rice mix", ProteinPer100G: 8, FatPer100G: 2, CarbPer100G: 76, KcalPer100G: 350,
		Allergens: []catalogdom.Allergen{"peanut"},
		Roles:     []catalogdom.RoleTag{{Code: "carb:grain", Kind: catalogdom.RoleKindCarb}},
	}
	return input
}

func TestComputeRecipeKeepsServingsAndRecomputesTheBatch(t *testing.T) {
	input := recipeSwapInput()

	preview, err := ComputeRecipe(input, RecipeRequest{PlanRecipeID: 11, NewTemplateID: 6})
	if err != nil {
		t.Fatalf("ComputeRecipe() error = %v", err)
	}

	if preview.Level != LevelRecipeChange {
		t.Fatalf("level = %q, want recipe_change", preview.Level)
	}
	recipe := preview.Plan.Recipes[0]
	// The fish batch held 2 × 2 = 4 servings; rice is a 4-serving base.
	if recipe.TemplateID != 6 || recipe.ScaleFactor != 1 {
		t.Fatalf("recipe = %#v, want template 6 at scale 1 for the same four servings", recipe)
	}
	if len(recipe.Fills) != 1 || recipe.Fills[0].SlotID != 20 || recipe.Fills[0].PlannedInputQtyG != 240 {
		t.Fatalf("fills = %#v, want only the required grain slot filled at base quantity", recipe.Fills)
	}
	if recipe.ExpectedOutputG == nil || *recipe.ExpectedOutputG != 600 {
		t.Fatalf("expected output = %v, want the template output scaled", recipe.ExpectedOutputG)
	}
	meal := preview.Plan.Meals[0]
	// Half of 240 g rice: 9.6 g protein, 91.2 g carbs.
	if meal.Planned.ProteinG != 9.6 || meal.Planned.CarbG != 91.2 {
		t.Fatalf("planned macros = %#v, want them recomputed from the new batch", meal.Planned)
	}
	if serving := meal.Components[0].PlannedServingG; serving == nil || *serving != 300 {
		t.Fatalf("planned serving = %v, want half of the new expected output", serving)
	}
	if len(preview.MacroDiff) != 1 || preview.MacroDiff[0].Delta.CarbG != 91.2 {
		t.Fatalf("macro diff = %#v, want the day's carbs to rise by the rice portion", preview.MacroDiff)
	}
	if len(preview.ShoppingDelta.Added) != 1 || len(preview.ShoppingDelta.Removed) != 1 {
		t.Fatalf("shopping delta = %#v, want rice added and the fish removed", preview.ShoppingDelta)
	}
	if got := input.Plan.Recipes[0].TemplateID; got != 5 {
		t.Fatalf("input plan template = %d, want the preview to leave the plan untouched", got)
	}
}

func TestComputeRecipeNeverPicksAProductAnEaterMustNotEat(t *testing.T) {
	input := recipeSwapInput()
	delete(input.Catalogue.Products, 10)
	eater := input.Catalogue.Eaters[1]
	eater.Allergens = []eaterdom.Allergen{"peanut"}
	input.Catalogue.Eaters[1] = eater

	_, err := ComputeRecipe(input, RecipeRequest{PlanRecipeID: 11, NewTemplateID: 6})

	var incompatible *IncompatibleError
	if !errors.As(err, &incompatible) {
		t.Fatalf("ComputeRecipe() error = %v, want the swap refused: only an allergen fills the slot", err)
	}
}

func TestComputeRecipeRejectsATemplateThePlanCannotHost(t *testing.T) {
	cases := map[string]func(*Input){
		"meal type not allowed": func(input *Input) {
			template := input.Catalogue.Templates[6]
			template.AllowedMealTypes = []mealdom.Type{mealdom.TypeDinner}
			input.Catalogue.Templates[6] = template
		},
		"does not keep for a multi-day plan": func(input *Input) {
			input.Plan.Days = 3
			template := input.Catalogue.Templates[6]
			template.Storage = recipedom.StorageEatSameDay
			input.Catalogue.Templates[6] = template
		},
		"equipment the household lacks": func(input *Input) {
			input.AvailableEquipment = []string{"oven"}
		},
		"not in the permanent library": func(input *Input) {
			template := input.Catalogue.Templates[6]
			template.Status = recipedom.StatusRejected
			input.Catalogue.Templates[6] = template
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			input := recipeSwapInput()
			mutate(&input)

			_, err := ComputeRecipe(input, RecipeRequest{PlanRecipeID: 11, NewTemplateID: 6})

			var incompatible *IncompatibleError
			if !errors.As(err, &incompatible) {
				t.Fatalf("ComputeRecipe() error = %v, want an incompatible swap", err)
			}
		})
	}
}

func TestComputeRecipeRejectsUnknownTargets(t *testing.T) {
	input := recipeSwapInput()

	if _, err := ComputeRecipe(input, RecipeRequest{PlanRecipeID: 99, NewTemplateID: 6}); !errors.Is(err, ErrUnknownRecipe) {
		t.Fatalf("ComputeRecipe() error = %v, want an unknown plan recipe", err)
	}
	if _, err := ComputeRecipe(input, RecipeRequest{PlanRecipeID: 11, NewTemplateID: 99}); !errors.Is(err, ErrUnknownTemplate) {
		t.Fatalf("ComputeRecipe() error = %v, want an unknown template", err)
	}
}

func TestComputeRecipeReportsTheDishesAssembledFromTheBatch(t *testing.T) {
	input := recipeSwapInput()
	input.Plan.Recipes = append(input.Plan.Recipes, planningdom.PlanRecipe{
		ID: 12, TemplateID: 5, ScaleFactor: 1,
		Fills: []planningdom.SlotFill{{ID: 2, SlotID: 9, ProductID: 2, PlannedInputQtyG: 100}},
	})
	input.Plan.PrepDependencies = []planningdom.PrepDependency{{DependentRecipeIndex: 1, SourceRecipeIndex: 0}}

	preview, err := ComputeRecipe(input, RecipeRequest{PlanRecipeID: 11, NewTemplateID: 6})
	if err != nil {
		t.Fatalf("ComputeRecipe() error = %v", err)
	}
	if len(preview.InvalidatedRecipes) != 1 || preview.InvalidatedRecipes[0].PlanRecipeID != 12 {
		t.Fatalf("invalidated = %#v, want the dish built from the swapped batch", preview.InvalidatedRecipes)
	}
}

func TestComputeRejectsAProductWithoutTheSlotRole(t *testing.T) {
	input := recipeSwapInput()

	_, err := Compute(input, Request{SlotFillID: 1, NewProductID: 10})

	var incompatible *IncompatibleError
	if !errors.As(err, &incompatible) {
		t.Fatalf("Compute() error = %v, want rice refused in a protein slot (HC-6)", err)
	}
}

func TestComputeRejectsAProductAnEaterIsAllergicTo(t *testing.T) {
	input := swapInput()
	product := input.Catalogue.Products[2]
	product.Allergens = []catalogdom.Allergen{"fish"}
	input.Catalogue.Products[2] = product
	eater := input.Catalogue.Eaters[1]
	eater.Allergens = []eaterdom.Allergen{"fish"}
	input.Catalogue.Eaters[1] = eater

	_, err := Compute(input, Request{SlotFillID: 1, NewProductID: 2})

	var incompatible *IncompatibleError
	if !errors.As(err, &incompatible) {
		t.Fatalf("Compute() error = %v, want the allergen refused (HC-1)", err)
	}
}

func TestComputeRecomputesPlannedMealMacros(t *testing.T) {
	preview, err := Compute(swapInput(), Request{SlotFillID: 1, NewProductID: 3})
	if err != nil {
		t.Fatalf("Compute() error = %v", err)
	}
	// Half of 200 g tuna at 33 g protein per 100 g.
	if got := preview.Plan.Meals[0].Planned.ProteinG; got != 33 {
		t.Fatalf("planned protein = %v, want the meal's stored macros to follow the new fill", got)
	}
}

func TestComputeRotationDiffsTheWholePlan(t *testing.T) {
	input := recipeSwapInput()
	replacement := input.Plan
	replacement.ID = 0
	replacement.Recipes = []planningdom.PlanRecipe{{
		TemplateID: 6, ScaleFactor: 1,
		Fills: []planningdom.SlotFill{{SlotID: 20, ProductID: 10, PlannedInputQtyG: 240}},
	}}
	replacement.Meals = []planningdom.Meal{{
		EaterID: 1, Date: swapDate(), Type: mealdom.TypeLunch,
		Components: []planningdom.ComponentAllocation{{PlanRecipeIndex: 0, PortionFactor: 0.5}},
	}}

	preview, err := ComputeRotation(input, replacement)
	if err != nil {
		t.Fatalf("ComputeRotation() error = %v", err)
	}

	if preview.Level != LevelFullRotation {
		t.Fatalf("level = %q, want full_rotation", preview.Level)
	}
	if len(preview.AffectedMeals) != 1 || preview.AffectedMeals[0] != 21 {
		t.Fatalf("affected meals = %#v, want every generated meal of the old plan", preview.AffectedMeals)
	}
	if len(preview.ShoppingDelta.Added) != 1 || preview.ShoppingDelta.Added[0] != 10 ||
		len(preview.ShoppingDelta.Removed) != 1 || preview.ShoppingDelta.Removed[0] != 1 {
		t.Fatalf("shopping delta = %#v, want rice in and salmon out", preview.ShoppingDelta)
	}
	if len(preview.MacroDiff) != 1 || preview.MacroDiff[0].Delta.CarbG != 91.2 {
		t.Fatalf("macro diff = %#v, want the eater-day compared across both plans", preview.MacroDiff)
	}
	if preview.Summary == "" {
		t.Fatal("summary is empty, want a readable explanation")
	}
}

func TestComputeSaysWhenASwapPushesADayOutOfItsMacroGoal(t *testing.T) {
	input := swapInput()
	// The day holds 23 g protein from salmon; tuna brings 33 g.
	input.Goals = []GoalTarget{{EaterID: 1, Date: swapDate(), Range: eaterdom.MacroRange{
		ProteinMinG: 20, ProteinMaxG: 30, FatMaxG: 100, CarbMaxG: 100, KcalMax: 3000,
	}}}

	within, err := Compute(input, Request{SlotFillID: 1, NewProductID: 2})
	if err != nil {
		t.Fatalf("Compute() error = %v", err)
	}
	if within.MacroGoalBreaches != 0 {
		t.Fatalf("breaches = %d, want none for a like-for-like swap", within.MacroGoalBreaches)
	}

	outside, err := Compute(input, Request{SlotFillID: 1, NewProductID: 3})
	if err != nil {
		t.Fatalf("Compute() error = %v", err)
	}
	if outside.MacroGoalBreaches != 1 {
		t.Fatalf("breaches = %d, want the day above its protein ceiling counted", outside.MacroGoalBreaches)
	}
	if !strings.Contains(outside.Summary, "leave the macro goal") {
		t.Fatalf("summary = %q, want the macro cost spelled out", outside.Summary)
	}
}

func TestComputeDoesNotChargeASwapForADayAlreadyOutsideItsGoal(t *testing.T) {
	input := swapInput()
	// The day holds 23 g protein from salmon, already under a 30 g floor.
	input.Goals = []GoalTarget{{EaterID: 1, Date: swapDate(), Range: eaterdom.MacroRange{
		ProteinMinG: 30, ProteinMaxG: 40, FatMaxG: 100, CarbMaxG: 100, KcalMax: 3000,
	}}}

	preview, err := Compute(input, Request{SlotFillID: 1, NewProductID: 2})
	if err != nil {
		t.Fatalf("Compute() error = %v", err)
	}
	if preview.MacroGoalBreaches != 0 {
		t.Fatalf("breaches = %d, want none: cod leaves the day exactly as far out as salmon did", preview.MacroGoalBreaches)
	}
	if !strings.Contains(preview.Summary, "no material change") {
		t.Fatalf("summary = %q, want a like-for-like swap reported as immaterial", preview.Summary)
	}
}
