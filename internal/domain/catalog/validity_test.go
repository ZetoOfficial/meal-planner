package catalog

import "testing"

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
