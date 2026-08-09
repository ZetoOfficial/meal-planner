package httpapi

import (
	"github.com/ZetoOfficial/meal-planner/internal/api/openapi"
	catalogdom "github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
	usecasecatalog "github.com/ZetoOfficial/meal-planner/internal/usecase/catalog"
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
		ProcessingConfidence: float64Pointer(input.ProcessingConfidence), CountsToFruitVeg: input.CountsToFruitVeg,
		IsLegume: input.IsLegume, IsWholeGrain: input.IsWholeGrain,
		WholeGrainRatio: float64Pointer(input.WholeGrainRatio), IsRedMeat: input.IsRedMeat,
		IsProcessedMeat: input.IsProcessedMeat, Source: input.ClassificationSource, Note: input.ClassificationNote,
	}
	if input.FoodGroup != nil {
		value := catalogdom.FoodGroup(*input.FoodGroup)
		result.FoodGroup = &value
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
	processingLevel := openapi.FoodClassificationProcessingLevel(value.ProcessingLevel)
	result := &openapi.FoodClassification{
		ProcessingLevel: &processingLevel, ProcessingConfidence: float32Pointer(value.ProcessingConfidence),
		CountsToFruitVeg: value.CountsToFruitVeg, IsLegume: value.IsLegume, IsWholeGrain: value.IsWholeGrain,
		WholeGrainRatio: float32Pointer(value.WholeGrainRatio), IsRedMeat: value.IsRedMeat, IsProcessedMeat: value.IsProcessedMeat,
		ClassificationSource: value.Source, ClassificationNote: value.Note,
	}
	if value.FoodGroup != nil {
		foodGroup := openapi.FoodClassificationFoodGroup(*value.FoodGroup)
		result.FoodGroup = &foodGroup
	}
	return result
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
