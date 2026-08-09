package solver

import (
	"strconv"

	catalogdom "github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
	eaterdom "github.com/ZetoOfficial/meal-planner/internal/domain/eater"
	recipedom "github.com/ZetoOfficial/meal-planner/internal/domain/recipe"
)

type RejectionCode string

const (
	RejectionAllergen           RejectionCode = "allergen"
	RejectionHardExclusion      RejectionCode = "hard_exclusion"
	RejectionIndustrialTransFat RejectionCode = "industrial_trans_fat"
	RejectionStorage            RejectionCode = "storage"
	RejectionEquipment          RejectionCode = "equipment"
	RejectionRoleCompatibility  RejectionCode = "role_compatibility"
)

type HardRejection struct {
	Code      RejectionCode
	EaterID   eaterdom.ID
	ProductID catalogdom.ProductID
	RecipeID  recipedom.ID
	Detail    string
}

func rejectProduct(eater eaterdom.Eater, product catalogdom.Product) *HardRejection {
	if allergen := matchingAllergen(eater.Allergens, product.Allergens); allergen != "" {
		return &HardRejection{
			Code:      RejectionAllergen,
			EaterID:   eater.ID,
			ProductID: product.ID,
			Detail:    allergen,
		}
	}
	if exclusion := matchingHardExclusion(eater.Exclusions, product); exclusion != "" {
		return &HardRejection{
			Code:      RejectionHardExclusion,
			EaterID:   eater.ID,
			ProductID: product.ID,
			Detail:    exclusion,
		}
	}
	if product.Nutrition.ContainsPartiallyHydrogenatedOils != nil && *product.Nutrition.ContainsPartiallyHydrogenatedOils {
		return &HardRejection{
			Code:      RejectionIndustrialTransFat,
			EaterID:   eater.ID,
			ProductID: product.ID,
			Detail:    "partially_hydrogenated_oils",
		}
	}
	if product.Nutrition.IndustrialTransFatPer100G != nil && *product.Nutrition.IndustrialTransFatPer100G > 0 {
		return &HardRejection{
			Code:      RejectionIndustrialTransFat,
			EaterID:   eater.ID,
			ProductID: product.ID,
			Detail:    "industrial_trans_fat_per_100g",
		}
	}
	return nil
}

func matchingAllergen(eaterAllergens []eaterdom.Allergen, productAllergens []catalogdom.Allergen) string {
	for _, eaterAllergen := range eaterAllergens {
		for _, productAllergen := range productAllergens {
			if string(eaterAllergen) == string(productAllergen) {
				return string(eaterAllergen)
			}
		}
	}
	return ""
}

func matchingHardExclusion(exclusions []eaterdom.Exclusion, product catalogdom.Product) string {
	for _, exclusion := range exclusions {
		if exclusion.Severity != eaterdom.ExclusionSeverityHard {
			continue
		}
		switch exclusion.TargetKind {
		case eaterdom.ExclusionTargetFoodGroup:
			if product.Classification.FoodGroup != nil && string(*product.Classification.FoodGroup) == exclusion.TargetValue {
				return exclusion.TargetValue
			}
		case eaterdom.ExclusionTargetRoleTag:
			if productHasRole(product, exclusion.TargetValue) {
				return exclusion.TargetValue
			}
		case eaterdom.ExclusionTargetProduct:
			if strconv.FormatInt(int64(product.ID), 10) == exclusion.TargetValue {
				return exclusion.TargetValue
			}
		}
	}
	return ""
}

func missingEquipment(required, available []string) string {
	availableSet := make(map[string]struct{}, len(available))
	for _, equipment := range available {
		availableSet[equipment] = struct{}{}
	}
	for _, equipment := range required {
		if _, exists := availableSet[equipment]; !exists {
			return equipment
		}
	}
	return ""
}

func recipeRejection(code RejectionCode, eaterID eaterdom.ID, recipeID recipedom.ID, detail string) HardRejection {
	return HardRejection{Code: code, EaterID: eaterID, RecipeID: recipeID, Detail: detail}
}
