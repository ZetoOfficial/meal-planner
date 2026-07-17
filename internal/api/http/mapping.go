package httpapi

import (
	openapitypes "github.com/oapi-codegen/runtime/types"

	"github.com/ZetoOfficial/meal-planner/internal/api/openapi"
	catalogdom "github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
	eaterdom "github.com/ZetoOfficial/meal-planner/internal/domain/eater"
	usecasecatalog "github.com/ZetoOfficial/meal-planner/internal/usecase/catalog"
	usecaseeater "github.com/ZetoOfficial/meal-planner/internal/usecase/eater"
)

func productCommand(input openapi.ProductInput) usecasecatalog.CreateProductCommand {
	command := usecasecatalog.CreateProductCommand{
		Brand: input.Brand, Name: input.Name, ProteinPer100G: float64(input.ProteinPer100g), FatPer100G: float64(input.FatPer100g),
		CarbPer100G: float64(input.CarbPer100g), KcalPer100G: float64(input.KcalPer100g), PricePer100G: float64Pointer(input.PricePer100g),
		Purchasable: true, Roles: append([]string(nil), input.Roles...),
	}
	if input.DefaultUnit != nil {
		command.DefaultUnit = *input.DefaultUnit
	}
	if input.Purchasable != nil {
		command.Purchasable = *input.Purchasable
	}
	if input.Allergens != nil {
		command.Allergens = make([]catalogdom.Allergen, len(*input.Allergens))
		for i, allergen := range *input.Allergens {
			command.Allergens[i] = catalogdom.Allergen(allergen)
		}
	}
	command.Nutrition = nutritionFactsCommand(input.Nutrition)
	command.DataQuality = nutritionDataCommand(input.DataQuality)
	command.Classification = classificationCommand(input.Classification)
	return command
}

func nutritionFactsCommand(input *openapi.NutritionFacts) catalogdom.NutritionFacts {
	if input == nil {
		return catalogdom.NutritionFacts{}
	}
	return catalogdom.NutritionFacts{
		SaturatedFatPer100G: float64Pointer(input.SaturatedFatPer100g), TransFatPer100G: float64Pointer(input.TransFatPer100g),
		IndustrialTransFatPer100G: float64Pointer(input.IndustrialTransFatPer100g), FiberPer100G: float64Pointer(input.FiberPer100g),
		SodiumMgPer100G: float64Pointer(input.SodiumMgPer100g), SaltGPer100G: float64Pointer(input.SaltGPer100g),
		TotalSugarsPer100G: float64Pointer(input.TotalSugarsPer100g), AddedSugarsPer100G: float64Pointer(input.AddedSugarsPer100g),
		FreeSugarsPer100G: float64Pointer(input.FreeSugarsPer100g), ContainsPartiallyHydrogenatedOils: input.ContainsPartiallyHydrogenatedOils,
	}
}

func nutritionDataCommand(input *openapi.NutritionDataQuality) catalogdom.NutritionData {
	if input == nil {
		return catalogdom.NutritionData{}
	}
	result := catalogdom.NutritionData{SourceNote: input.SourceNote, LastVerifiedAt: input.LastVerifiedAt}
	if input.NutritionDataQuality != nil {
		result.Quality = catalogdom.NutritionDataQuality(*input.NutritionDataQuality)
	}
	if input.SourceKind != nil {
		value := catalogdom.SourceKind(*input.SourceKind)
		result.SourceKind = &value
	}
	return result
}

func classificationCommand(input *openapi.FoodClassification) catalogdom.FoodClassification {
	if input == nil {
		return catalogdom.FoodClassification{}
	}
	result := catalogdom.FoodClassification{
		ProcessingConfidence: float64Pointer(input.ProcessingConfidence), CountsToFruitVeg: dereferenceBool(input.CountsToFruitVeg),
		IsLegume: dereferenceBool(input.IsLegume), IsWholeGrain: dereferenceBool(input.IsWholeGrain),
		WholeGrainRatio: float64Pointer(input.WholeGrainRatio), IsRedMeat: dereferenceBool(input.IsRedMeat),
		IsProcessedMeat: dereferenceBool(input.IsProcessedMeat), Note: input.ClassificationNote,
	}
	if input.FoodGroup != nil {
		result.FoodGroup = catalogdom.FoodGroup(*input.FoodGroup)
	}
	if input.ProcessingLevel != nil {
		result.ProcessingLevel = catalogdom.ProcessingLevel(*input.ProcessingLevel)
	}
	return result
}

func productResponse(product catalogdom.Product) openapi.Product {
	roles := make([]string, len(product.Roles))
	for i := range product.Roles {
		roles[i] = product.Roles[i].Code
	}
	allergens := make([]string, len(product.Allergens))
	for i := range product.Allergens {
		allergens[i] = string(product.Allergens[i])
	}
	return openapi.Product{
		Id: int64(product.ID), Brand: product.Brand, Name: product.Name, ProteinPer100g: float32(product.ProteinPer100G),
		FatPer100g: float32(product.FatPer100G), CarbPer100g: float32(product.CarbPer100G), KcalPer100g: float32(product.KcalPer100G),
		PricePer100g: float32Pointer(product.PricePer100G), DefaultUnit: stringPointer(product.DefaultUnit), Purchasable: boolPointer(product.Purchasable),
		Allergens: &allergens, Roles: &roles, Nutrition: nutritionFactsResponse(product.Nutrition),
		DataQuality: nutritionDataResponse(product.DataQuality), Classification: classificationResponse(product.Classification),
	}
}

func productsResponse(products []catalogdom.Product) []openapi.Product {
	result := make([]openapi.Product, len(products))
	for i := range products {
		result[i] = productResponse(products[i])
	}
	return result
}

func nutritionFactsResponse(value catalogdom.NutritionFacts) *openapi.NutritionFacts {
	return &openapi.NutritionFacts{
		SaturatedFatPer100g: float32Pointer(value.SaturatedFatPer100G), TransFatPer100g: float32Pointer(value.TransFatPer100G),
		IndustrialTransFatPer100g: float32Pointer(value.IndustrialTransFatPer100G), FiberPer100g: float32Pointer(value.FiberPer100G),
		SodiumMgPer100g: float32Pointer(value.SodiumMgPer100G), SaltGPer100g: float32Pointer(value.SaltGPer100G),
		TotalSugarsPer100g: float32Pointer(value.TotalSugarsPer100G), AddedSugarsPer100g: float32Pointer(value.AddedSugarsPer100G),
		FreeSugarsPer100g: float32Pointer(value.FreeSugarsPer100G), ContainsPartiallyHydrogenatedOils: value.ContainsPartiallyHydrogenatedOils,
	}
}

func nutritionDataResponse(value catalogdom.NutritionData) *openapi.NutritionDataQuality {
	result := &openapi.NutritionDataQuality{NutritionCoverageScore: float32Pointer(&value.Coverage), SourceNote: value.SourceNote, LastVerifiedAt: value.LastVerifiedAt}
	quality := openapi.NutritionDataQualityNutritionDataQuality(value.Quality)
	result.NutritionDataQuality = &quality
	if value.SourceKind != nil {
		source := openapi.NutritionDataQualitySourceKind(*value.SourceKind)
		result.SourceKind = &source
	}
	return result
}

func classificationResponse(value catalogdom.FoodClassification) *openapi.FoodClassification {
	foodGroup := openapi.FoodClassificationFoodGroup(value.FoodGroup)
	processingLevel := openapi.FoodClassificationProcessingLevel(value.ProcessingLevel)
	return &openapi.FoodClassification{
		FoodGroup: &foodGroup, ProcessingLevel: &processingLevel, ProcessingConfidence: float32Pointer(value.ProcessingConfidence),
		CountsToFruitVeg: boolPointer(value.CountsToFruitVeg), IsLegume: boolPointer(value.IsLegume), IsWholeGrain: boolPointer(value.IsWholeGrain),
		WholeGrainRatio: float32Pointer(value.WholeGrainRatio), IsRedMeat: boolPointer(value.IsRedMeat), IsProcessedMeat: boolPointer(value.IsProcessedMeat),
		ClassificationNote: value.Note,
	}
}

func roleTagResponse(value catalogdom.RoleTag) openapi.RoleTag {
	return openapi.RoleTag{
		Id: value.ID, Code: value.Code, Kind: openapi.RoleKind(value.Kind), Label: value.Label, ParentId: value.ParentID,
	}
}

func roleTagsResponse(tags []catalogdom.RoleTag) []openapi.RoleTag {
	result := make([]openapi.RoleTag, len(tags))
	for i := range tags {
		result[i] = roleTagResponse(tags[i])
	}
	return result
}

func eaterCommand(input openapi.EaterInput) usecaseeater.CreateEaterCommand {
	result := usecaseeater.CreateEaterCommand{Name: input.Name, Goal: macroRangeCommand(input.Goal)}
	if input.Sex != nil {
		result.Sex = eaterdom.Sex(*input.Sex)
	}
	if input.Allergens != nil {
		result.Allergens = make([]eaterdom.Allergen, len(*input.Allergens))
		for i, allergen := range *input.Allergens {
			result.Allergens[i] = eaterdom.Allergen(allergen)
		}
	}
	return result
}

func macroRangeCommand(input openapi.MacroRange) usecaseeater.MacroGoalCommand {
	return usecaseeater.MacroGoalCommand{Range: eaterdom.MacroRange{
		ProteinMinG: float64(input.ProteinMinG), ProteinMaxG: float64(input.ProteinMaxG), FatMinG: float64(input.FatMinG), FatMaxG: float64(input.FatMaxG),
		CarbMinG: float64(input.CarbMinG), CarbMaxG: float64(input.CarbMaxG), KcalMin: input.KcalMin, KcalMax: input.KcalMax,
	}}
}

func macroGoalCommand(input openapi.MacroGoalInput) usecaseeater.MacroGoalCommand {
	result := macroRangeCommand(openapi.MacroRange{
		ProteinMinG: input.ProteinMinG, ProteinMaxG: input.ProteinMaxG, FatMinG: input.FatMinG, FatMaxG: input.FatMaxG,
		CarbMinG: input.CarbMinG, CarbMaxG: input.CarbMaxG, KcalMin: input.KcalMin, KcalMax: input.KcalMax,
	})
	result.Note = input.Note
	if input.EffectiveFrom != nil {
		result.EffectiveFrom = input.EffectiveFrom.Time
	}
	return result
}

func fixedMealCommand(input openapi.FixedMealInput) usecaseeater.CreateFixedMealCommand {
	result := usecaseeater.CreateFixedMealCommand{MealType: eaterdom.MealType(input.MealType), Label: input.Label, Macros: eaterdom.Macros{
		ProteinG: float64(input.Macros.ProteinG), FatG: float64(input.Macros.FatG), CarbG: float64(input.Macros.CarbG), Kcal: input.Macros.Kcal,
	}}
	if input.Quality != nil {
		result.Quality = fixedMealQualityCommand(*input.Quality)
		result.IncludeInAnalytics = input.Quality.IncludeInAnalytics
		result.IncludeInWarnings = input.Quality.IncludeInWarnings
	}
	return result
}

func fixedMealQualityCommand(input openapi.FixedMealQuality) eaterdom.FixedMealQuality {
	return eaterdom.FixedMealQuality{
		SaturatedFatG: float64Pointer(input.SaturatedFatG), TransFatG: float64Pointer(input.TransFatG), FiberG: float64Pointer(input.FiberG),
		SodiumMg: float64Pointer(input.SodiumMg), AddedSugarsG: float64Pointer(input.AddedSugarsG), FreeSugarsG: float64Pointer(input.FreeSugarsG),
		ProcessedMeatG: float64Pointer(input.ProcessedMeatG), RedMeatG: float64Pointer(input.RedMeatG),
		WholeGrainGEquiv: float64Pointer(input.WholeGrainGEquiv), FruitVegG: float64Pointer(input.FruitVegG),
	}
}

func exclusionCommand(input openapi.ExclusionInput) usecaseeater.CreateExclusionCommand {
	result := usecaseeater.CreateExclusionCommand{TargetKind: eaterdom.ExclusionTarget(input.TargetKind), TargetValue: input.TargetValue, Note: input.Note}
	if input.Severity != nil {
		result.Severity = eaterdom.ExclusionSeverity(*input.Severity)
	}
	return result
}

func eaterResponse(value eaterdom.Eater) openapi.Eater {
	allergens := make([]string, len(value.Allergens))
	for i := range value.Allergens {
		allergens[i] = string(value.Allergens[i])
	}
	exclusions := make([]openapi.Exclusion, 0, len(value.Exclusions))
	for i := range value.Exclusions {
		exclusions = append(exclusions, exclusionResponse(value.Exclusions[i]))
	}
	result := openapi.Eater{Id: int64(value.ID), Name: value.Name, Allergens: &allergens, Exclusions: &exclusions}
	sex := openapi.Sex(value.Sex)
	result.Sex = &sex
	if value.CurrentGoal != nil {
		goal := macroGoalResponse(*value.CurrentGoal)
		result.CurrentGoal = &goal
	}
	return result
}

func eatersResponse(eaters []eaterdom.Eater) []openapi.Eater {
	result := make([]openapi.Eater, len(eaters))
	for i := range eaters {
		result[i] = eaterResponse(eaters[i])
	}
	return result
}

func macroGoalResponse(value eaterdom.MacroGoal) openapi.MacroGoal {
	return openapi.MacroGoal{
		Id: int64(value.ID), EffectiveFrom: openapitypes.Date{Time: value.EffectiveFrom}, Note: value.Note,
		ProteinMinG: float32(value.Range.ProteinMinG), ProteinMaxG: float32(value.Range.ProteinMaxG), FatMinG: float32(value.Range.FatMinG), FatMaxG: float32(value.Range.FatMaxG),
		CarbMinG: float32(value.Range.CarbMinG), CarbMaxG: float32(value.Range.CarbMaxG), KcalMin: value.Range.KcalMin, KcalMax: value.Range.KcalMax,
	}
}

func macroGoalsResponse(goals []eaterdom.MacroGoal) []openapi.MacroGoal {
	result := make([]openapi.MacroGoal, len(goals))
	for i := range goals {
		result[i] = macroGoalResponse(goals[i])
	}
	return result
}

func fixedMealResponse(value eaterdom.FixedMeal) openapi.FixedMeal {
	active := value.Active
	return openapi.FixedMeal{
		Id: int64(value.ID), EaterId: int64(value.EaterID), MealType: openapi.MealType(value.MealType), Label: value.Label,
		Macros:  openapi.Macros{ProteinG: float32(value.Macros.ProteinG), FatG: float32(value.Macros.FatG), CarbG: float32(value.Macros.CarbG), Kcal: value.Macros.Kcal},
		Quality: fixedMealQualityResponse(value.Quality), Active: &active,
	}
}

func fixedMealsResponse(meals []eaterdom.FixedMeal) []openapi.FixedMeal {
	result := make([]openapi.FixedMeal, len(meals))
	for i := range meals {
		result[i] = fixedMealResponse(meals[i])
	}
	return result
}

func fixedMealQualityResponse(value eaterdom.FixedMealQuality) *openapi.FixedMealQuality {
	return &openapi.FixedMealQuality{
		SaturatedFatG: float32Pointer(value.SaturatedFatG), TransFatG: float32Pointer(value.TransFatG), FiberG: float32Pointer(value.FiberG), SodiumMg: float32Pointer(value.SodiumMg),
		AddedSugarsG: float32Pointer(value.AddedSugarsG), FreeSugarsG: float32Pointer(value.FreeSugarsG), ProcessedMeatG: float32Pointer(value.ProcessedMeatG), RedMeatG: float32Pointer(value.RedMeatG),
		WholeGrainGEquiv: float32Pointer(value.WholeGrainGEquiv), FruitVegG: float32Pointer(value.FruitVegG),
		IncludeInAnalytics: boolPointer(value.IncludeInAnalytics), IncludeInWarnings: boolPointer(value.IncludeInWarnings),
	}
}

func exclusionResponse(value eaterdom.Exclusion) openapi.Exclusion {
	return openapi.Exclusion{Id: int64(value.ID), TargetKind: openapi.ExclusionTargetKind(value.TargetKind), TargetValue: value.TargetValue, Severity: openapi.ExclusionSeverity(value.Severity), Note: value.Note}
}

func exclusionsResponse(exclusions []eaterdom.Exclusion) []openapi.Exclusion {
	result := make([]openapi.Exclusion, len(exclusions))
	for i := range exclusions {
		result[i] = exclusionResponse(exclusions[i])
	}
	return result
}

func float64Pointer(value *float32) *float64 {
	if value == nil {
		return nil
	}
	converted := float64(*value)
	return &converted
}

func float32Pointer(value *float64) *float32 {
	if value == nil {
		return nil
	}
	converted := float32(*value)
	return &converted
}
func boolPointer(value bool) *bool       { return &value }
func stringPointer(value string) *string { return &value }
func dereferenceBool(value *bool) bool   { return value != nil && *value }
