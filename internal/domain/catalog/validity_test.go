package catalog

import "testing"

func TestProductIDValid(t *testing.T) {
	for value, want := range map[ProductID]bool{1: true, 0: false, -1: false} {
		if got := value.Valid(); got != want {
			t.Errorf("ProductID(%d).Valid() = %t, want %t", value, got, want)
		}
	}
}

func TestNutritionDataQualityValid(t *testing.T) {
	for _, value := range []NutritionDataQuality{
		NutritionDataQualityLabelExact,
		NutritionDataQualityTrustedDB,
		NutritionDataQualityEstimated,
		NutritionDataQualityUnknown,
	} {
		if !value.Valid() {
			t.Errorf("NutritionDataQuality(%q).Valid() = false, want true", value)
		}
	}
	if NutritionDataQuality("invalid").Valid() {
		t.Error("NutritionDataQuality(\"invalid\").Valid() = true, want false")
	}
}

func TestSourceKindValid(t *testing.T) {
	for _, value := range []SourceKind{
		SourceKindPackLabel,
		SourceKindStoreCard,
		SourceKindCompositionTable,
		SourceKindManualEstimate,
	} {
		if !value.Valid() {
			t.Errorf("SourceKind(%q).Valid() = false, want true", value)
		}
	}
	if SourceKind("invalid").Valid() {
		t.Error("SourceKind(\"invalid\").Valid() = true, want false")
	}
}

func TestFoodGroupValid(t *testing.T) {
	for _, value := range []FoodGroup{
		FoodGroupFruit,
		FoodGroupVegetable,
		FoodGroupPotato,
		FoodGroupGrain,
		FoodGroupLegume,
		FoodGroupNutsSeeds,
		FoodGroupDairy,
		FoodGroupDairyAlt,
		FoodGroupFishSeafood,
		FoodGroupEgg,
		FoodGroupRedMeat,
		FoodGroupWhiteMeat,
		FoodGroupProcessedMeat,
		FoodGroupMixed,
		FoodGroupSauceCondiment,
		FoodGroupConfectionery,
		FoodGroupBeverage,
		FoodGroupOther,
	} {
		if !value.Valid() {
			t.Errorf("FoodGroup(%q).Valid() = false, want true", value)
		}
	}
	if FoodGroup("invalid").Valid() {
		t.Error("FoodGroup(\"invalid\").Valid() = true, want false")
	}
}

func TestProcessingLevelValid(t *testing.T) {
	for _, value := range []ProcessingLevel{
		ProcessingLevelMinimallyProcessed,
		ProcessingLevelProcessedCulinary,
		ProcessingLevelProcessedFood,
		ProcessingLevelUltraProcessed,
		ProcessingLevelUnknown,
	} {
		if !value.Valid() {
			t.Errorf("ProcessingLevel(%q).Valid() = false, want true", value)
		}
	}
	if ProcessingLevel("invalid").Valid() {
		t.Error("ProcessingLevel(\"invalid\").Valid() = true, want false")
	}
}
