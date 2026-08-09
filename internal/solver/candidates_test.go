package solver

import (
	"reflect"
	"testing"
	"time"

	catalogdom "github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
	eaterdom "github.com/ZetoOfficial/meal-planner/internal/domain/eater"
	mealdom "github.com/ZetoOfficial/meal-planner/internal/domain/meal"
	recipedom "github.com/ZetoOfficial/meal-planner/internal/domain/recipe"
)

func TestSolverInputKeepsFixedMealsOutsideCandidates(t *testing.T) {
	date := time.Date(2026, time.August, 10, 0, 0, 0, 0, time.UTC)
	input := Input{
		StartDate: date,
		Days:      1,
		CookDate:  date,
		Novelty:   2,
		Eaters: []EaterInput{{
			Eater: eaterdom.Eater{ID: 1},
			Days: []EaterDayInput{{
				Date:               date,
				Goal:               eaterdom.MacroGoal{ID: 7, EaterID: 1},
				GeneratedMealTypes: []mealdom.Type{mealdom.TypeLunch},
			}},
			FixedMeals: []eaterdom.FixedMeal{{ID: 9, EaterID: 1, MealType: mealdom.TypeBreakfast}},
		}},
	}

	if input.Eaters[0].FixedMeals[0].ID != 9 || input.Eaters[0].Days[0].Goal.ID != 7 {
		t.Fatalf("solver input lost fixed meal or dated macro goal: %#v", input.Eaters[0])
	}
	if _, exists := reflect.TypeOf(EaterCandidates{}).FieldByName("FixedMeals"); exists {
		t.Fatal("fixed meals must not be exposed as swappable candidates")
	}
}

func TestFilterCandidatesAppliesAllergensPerEater(t *testing.T) {
	products := []catalogdom.Product{
		{ID: 1, Allergens: []catalogdom.Allergen{"casein"}},
		{ID: 2},
	}
	input := baseInput(products, nil)
	input.Eaters = []EaterInput{
		{Eater: eaterdom.Eater{ID: 1, Allergens: []eaterdom.Allergen{"casein"}}},
		{Eater: eaterdom.Eater{ID: 2}},
	}

	got := FilterCandidates(input)

	assertProductIDs(t, got.Eaters[0].Products, 2)
	assertProductIDs(t, got.Eaters[1].Products, 1, 2)
	assertRejection(t, got.Rejections, RejectionAllergen, 1, 1, 0, "casein")
}

func TestFilterCandidatesSeparatesHardExclusionsFromSoftPreferences(t *testing.T) {
	fish := catalogdom.FoodGroupFishSeafood
	products := []catalogdom.Product{
		{ID: 1, Classification: catalogdom.FoodClassification{FoodGroup: &fish}},
		{ID: 2, Roles: []catalogdom.RoleTag{{Code: "protein:lean"}}},
		{ID: 3},
		{ID: 4},
	}
	tests := map[string]eaterdom.Exclusion{
		"food group": {TargetKind: eaterdom.ExclusionTargetFoodGroup, TargetValue: string(fish), Severity: eaterdom.ExclusionSeverityHard},
		"role":       {TargetKind: eaterdom.ExclusionTargetRoleTag, TargetValue: "protein:lean", Severity: eaterdom.ExclusionSeverityHard},
		"product":    {TargetKind: eaterdom.ExclusionTargetProduct, TargetValue: "3", Severity: eaterdom.ExclusionSeverityHard},
	}

	for name, exclusion := range tests {
		t.Run(name, func(t *testing.T) {
			input := baseInput(products, nil)
			input.Eaters[0].Eater.Exclusions = []eaterdom.Exclusion{
				exclusion,
				{TargetKind: eaterdom.ExclusionTargetProduct, TargetValue: "4", Severity: eaterdom.ExclusionSeveritySoft},
			}

			got := FilterCandidates(input)

			if len(got.Eaters[0].Products) != 3 {
				t.Fatalf("accepted products = %#v, want one hard exclusion and soft product retained", got.Eaters[0].Products)
			}
			if got.Eaters[0].Products[2].ID != 4 {
				t.Fatal("soft exclusion was incorrectly applied as a hard constraint")
			}
			if len(got.Rejections) != 1 || got.Rejections[0].Code != RejectionHardExclusion {
				t.Fatalf("rejections = %#v, want one hard exclusion", got.Rejections)
			}
		})
	}
}

func TestFilterCandidatesRejectsMultiDayIncompatibleStorage(t *testing.T) {
	recipes := []recipedom.Template{
		{ID: 1, Storage: recipedom.StorageEatSameDay},
		{ID: 2, Storage: recipedom.StorageKeeps3d},
	}
	input := baseInput(nil, recipes)
	input.Days = 3

	got := FilterCandidates(input)

	assertRecipeIDs(t, got.Eaters[0].Recipes, 2)
	assertRejection(t, got.Rejections, RejectionStorage, 1, 0, 1, string(recipedom.StorageEatSameDay))

	input.Days = 1
	got = FilterCandidates(input)
	assertRecipeIDs(t, got.Eaters[0].Recipes, 1, 2)
}

func TestFilterCandidatesRequiresAvailableEquipment(t *testing.T) {
	input := baseInput(nil, []recipedom.Template{{ID: 1, Storage: recipedom.StorageKeeps3d, Equipment: []string{"oven"}}})
	input.AvailableEquipment = []string{"stove"}

	got := FilterCandidates(input)

	assertRecipeIDs(t, got.Eaters[0].Recipes)
	assertRejection(t, got.Rejections, RejectionEquipment, 1, 0, 1, "oven")

	input.AvailableEquipment = append(input.AvailableEquipment, "oven")
	got = FilterCandidates(input)
	assertRecipeIDs(t, got.Eaters[0].Recipes, 1)
}

func TestFilterCandidatesBuildsRoleCompatibleSlotProducts(t *testing.T) {
	products := []catalogdom.Product{
		{ID: 1, Roles: []catalogdom.RoleTag{{Code: "protein:lean"}}},
		{ID: 2, Roles: []catalogdom.RoleTag{{Code: "veg:green"}}},
	}
	recipes := []recipedom.Template{{
		ID:      1,
		Storage: recipedom.StorageKeeps3d,
		Slots: []recipedom.Slot{
			{ID: 1, RoleTagCode: "protein:lean"},
			{ID: 2, RoleTagCode: "sauce:light", Optional: true},
		},
	}}
	input := baseInput(products, recipes)

	got := FilterCandidates(input)

	assertRecipeIDs(t, got.Eaters[0].Recipes, 1)
	assertProductIDs(t, got.Eaters[0].Recipes[0].Slots[0].Products, 1)
	if len(got.Eaters[0].Recipes[0].Slots[1].Products) != 0 {
		t.Fatal("optional slot unexpectedly received incompatible products")
	}

	input.Products = products[1:]
	got = FilterCandidates(input)
	assertRecipeIDs(t, got.Eaters[0].Recipes)
	assertRejection(t, got.Rejections, RejectionRoleCompatibility, 1, 0, 1, "protein:lean")
}

func TestFilterCandidatesHardRejectsOnlyExplicitIndustrialTransFat(t *testing.T) {
	positive := 0.1
	zero := 0.0
	trueValue := true
	products := []catalogdom.Product{
		{ID: 1, Nutrition: catalogdom.NutritionFacts{ContainsPartiallyHydrogenatedOils: &trueValue}},
		{ID: 2, Nutrition: catalogdom.NutritionFacts{IndustrialTransFatPer100G: &positive}},
		{ID: 3, Nutrition: catalogdom.NutritionFacts{TransFatPer100G: &positive}},
		{ID: 4, Nutrition: catalogdom.NutritionFacts{IndustrialTransFatPer100G: &zero}},
	}

	got := FilterCandidates(baseInput(products, nil))

	assertProductIDs(t, got.Eaters[0].Products, 3, 4)
	assertRejection(t, got.Rejections, RejectionIndustrialTransFat, 1, 1, 0, "partially_hydrogenated_oils")
	assertRejection(t, got.Rejections, RejectionIndustrialTransFat, 1, 2, 0, "industrial_trans_fat_per_100g")
}

func baseInput(products []catalogdom.Product, recipes []recipedom.Template) Input {
	return Input{
		Days:     1,
		Eaters:   []EaterInput{{Eater: eaterdom.Eater{ID: 1}}},
		Products: products,
		Recipes:  recipes,
	}
}

func assertProductIDs(t *testing.T, products []catalogdom.Product, want ...catalogdom.ProductID) {
	t.Helper()
	if len(products) != len(want) {
		t.Fatalf("product count = %d, want %d", len(products), len(want))
	}
	for i := range products {
		if products[i].ID != want[i] {
			t.Fatalf("product ID at %d = %d, want %d", i, products[i].ID, want[i])
		}
	}
}

func assertRecipeIDs(t *testing.T, recipes []RecipeCandidate, want ...recipedom.ID) {
	t.Helper()
	if len(recipes) != len(want) {
		t.Fatalf("recipe count = %d, want %d", len(recipes), len(want))
	}
	for i := range recipes {
		if recipes[i].Recipe.ID != want[i] {
			t.Fatalf("recipe ID at %d = %d, want %d", i, recipes[i].Recipe.ID, want[i])
		}
	}
}

func assertRejection(
	t *testing.T,
	rejections []HardRejection,
	code RejectionCode,
	eaterID eaterdom.ID,
	productID catalogdom.ProductID,
	recipeID recipedom.ID,
	detail string,
) {
	t.Helper()
	for _, rejection := range rejections {
		if rejection.Code == code && rejection.EaterID == eaterID && rejection.ProductID == productID &&
			rejection.RecipeID == recipeID && rejection.Detail == detail {
			return
		}
	}
	t.Fatalf("missing rejection code=%q eater=%d product=%d recipe=%d detail=%q in %#v", code, eaterID, productID, recipeID, detail, rejections)
}
