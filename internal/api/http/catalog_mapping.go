package httpapi

import (
	"github.com/ZetoOfficial/meal-planner/api/sdk/go/model"
	catalogdom "github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
	usecasecatalog "github.com/ZetoOfficial/meal-planner/internal/usecase/catalog"
)

func productCommand(input model.ProductInput) usecasecatalog.CreateProductCommand {
	command := usecasecatalog.CreateProductCommand{
		Brand: input.Brand, Name: input.Name, ProteinPer100G: float64(input.ProteinPer100g), FatPer100G: float64(input.FatPer100g),
		CarbPer100G: float64(input.CarbPer100g), KcalPer100G: float64(input.KcalPer100g), PricePer100G: input.PricePer100g,
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

func nutritionFactsCommand(input *model.NutritionFacts) catalogdom.NutritionFacts {
	if input == nil {
		return catalogdom.NutritionFacts{}
	}
	return catalogdom.NutritionFacts{
		SaturatedFatPer100G: input.SaturatedFatPer100g, TransFatPer100G: input.TransFatPer100g,
		IndustrialTransFatPer100G: input.IndustrialTransFatPer100g, FiberPer100G: input.FiberPer100g,
		SodiumMgPer100G: input.SodiumMgPer100g, SaltGPer100G: input.SaltGPer100g,
		TotalSugarsPer100G: input.TotalSugarsPer100g, AddedSugarsPer100G: input.AddedSugarsPer100g,
		FreeSugarsPer100G: input.FreeSugarsPer100g, ContainsPartiallyHydrogenatedOils: input.ContainsPartiallyHydrogenatedOils,
	}
}

func nutritionDataCommand(input *model.NutritionDataQuality) catalogdom.NutritionData {
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

func classificationCommand(input *model.FoodClassification) catalogdom.FoodClassification {
	if input == nil {
		return catalogdom.FoodClassification{}
	}
	result := catalogdom.FoodClassification{
		ProcessingConfidence: input.ProcessingConfidence, CountsToFruitVeg: input.CountsToFruitVeg,
		IsLegume: input.IsLegume, IsWholeGrain: input.IsWholeGrain,
		WholeGrainRatio: input.WholeGrainRatio, IsRedMeat: input.IsRedMeat,
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

func productResponse(product catalogdom.Product) model.Product {
	roles := make([]string, len(product.Roles))
	for i := range product.Roles {
		roles[i] = product.Roles[i].Code
	}
	allergens := make([]string, len(product.Allergens))
	for i := range product.Allergens {
		allergens[i] = string(product.Allergens[i])
	}
	return model.Product{
		Id: int64(product.ID), Brand: product.Brand, Name: product.Name, ProteinPer100g: product.ProteinPer100G,
		FatPer100g: product.FatPer100G, CarbPer100g: product.CarbPer100G, KcalPer100g: product.KcalPer100G,
		PricePer100g: product.PricePer100G, DefaultUnit: stringPointer(product.DefaultUnit), Purchasable: boolPointer(product.Purchasable),
		Allergens: &allergens, Roles: &roles, Nutrition: nutritionFactsResponse(product.Nutrition),
		DataQuality: nutritionDataResponse(product.DataQuality), Classification: classificationResponse(product.Classification),
	}
}

func productsResponse(products []catalogdom.Product) []model.Product {
	result := make([]model.Product, len(products))
	for i := range products {
		result[i] = productResponse(products[i])
	}
	return result
}

func nutritionFactsResponse(value catalogdom.NutritionFacts) *model.NutritionFacts {
	return &model.NutritionFacts{
		SaturatedFatPer100g: value.SaturatedFatPer100G, TransFatPer100g: value.TransFatPer100G,
		IndustrialTransFatPer100g: value.IndustrialTransFatPer100G, FiberPer100g: value.FiberPer100G,
		SodiumMgPer100g: value.SodiumMgPer100G, SaltGPer100g: value.SaltGPer100G,
		TotalSugarsPer100g: value.TotalSugarsPer100G, AddedSugarsPer100g: value.AddedSugarsPer100G,
		FreeSugarsPer100g: value.FreeSugarsPer100G, ContainsPartiallyHydrogenatedOils: value.ContainsPartiallyHydrogenatedOils,
	}
}

func nutritionDataResponse(value catalogdom.NutritionData) *model.NutritionDataQuality {
	result := &model.NutritionDataQuality{NutritionCoverageScore: &value.Coverage, SourceNote: value.SourceNote, LastVerifiedAt: value.LastVerifiedAt}
	quality := model.NutritionDataQualityNutritionDataQuality(value.Quality)
	result.NutritionDataQuality = &quality
	if value.SourceKind != nil {
		source := model.NutritionDataQualitySourceKind(*value.SourceKind)
		result.SourceKind = &source
	}
	return result
}

func classificationResponse(value catalogdom.FoodClassification) *model.FoodClassification {
	processingLevel := model.FoodClassificationProcessingLevel(value.ProcessingLevel)
	result := &model.FoodClassification{
		ProcessingLevel: &processingLevel, ProcessingConfidence: value.ProcessingConfidence,
		CountsToFruitVeg: value.CountsToFruitVeg, IsLegume: value.IsLegume, IsWholeGrain: value.IsWholeGrain,
		WholeGrainRatio: value.WholeGrainRatio, IsRedMeat: value.IsRedMeat, IsProcessedMeat: value.IsProcessedMeat,
		ClassificationSource: value.Source, ClassificationNote: value.Note,
	}
	if value.FoodGroup != nil {
		foodGroup := model.FoodClassificationFoodGroup(*value.FoodGroup)
		result.FoodGroup = &foodGroup
	}
	return result
}

func roleTagResponse(value catalogdom.RoleTag) model.RoleTag {
	return model.RoleTag{
		Id: value.ID, Code: value.Code, Kind: model.RoleKind(value.Kind), Label: value.Label, ParentId: value.ParentID,
	}
}

func roleTagsResponse(tags []catalogdom.RoleTag) []model.RoleTag {
	result := make([]model.RoleTag, len(tags))
	for i := range tags {
		result[i] = roleTagResponse(tags[i])
	}
	return result
}
