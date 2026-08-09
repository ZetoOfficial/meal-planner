package catalog

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	catalogdom "github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
	"github.com/ZetoOfficial/meal-planner/internal/postgres/pgvalue"
	"github.com/ZetoOfficial/meal-planner/internal/postgres/sqlc"
)

func productParams(product catalogdom.Product) sqlc.CreateProductParams {
	return sqlc.CreateProductParams{
		Brand:                             product.Brand,
		Name:                              product.Name,
		ProteinPer100g:                    pgvalue.Numeric(product.ProteinPer100G),
		FatPer100g:                        pgvalue.Numeric(product.FatPer100G),
		CarbPer100g:                       pgvalue.Numeric(product.CarbPer100G),
		KcalPer100g:                       pgvalue.Numeric(product.KcalPer100G),
		PricePer100g:                      pgvalue.OptionalNumeric(product.PricePer100G),
		DefaultUnit:                       product.DefaultUnit,
		Purchasable:                       product.Purchasable,
		Notes:                             product.Notes,
		SaturatedFatPer100g:               pgvalue.OptionalNumeric(product.Nutrition.SaturatedFatPer100G),
		TransFatPer100g:                   pgvalue.OptionalNumeric(product.Nutrition.TransFatPer100G),
		IndustrialTransFatPer100g:         pgvalue.OptionalNumeric(product.Nutrition.IndustrialTransFatPer100G),
		FiberPer100g:                      pgvalue.OptionalNumeric(product.Nutrition.FiberPer100G),
		SodiumMgPer100g:                   pgvalue.OptionalNumeric(product.Nutrition.SodiumMgPer100G),
		SaltGPer100g:                      pgvalue.OptionalNumeric(product.Nutrition.SaltGPer100G),
		TotalSugarsPer100g:                pgvalue.OptionalNumeric(product.Nutrition.TotalSugarsPer100G),
		AddedSugarsPer100g:                pgvalue.OptionalNumeric(product.Nutrition.AddedSugarsPer100G),
		FreeSugarsPer100g:                 pgvalue.OptionalNumeric(product.Nutrition.FreeSugarsPer100G),
		ContainsPartiallyHydrogenatedOils: product.Nutrition.ContainsPartiallyHydrogenatedOils,
		NutritionDataQuality:              sqlc.NutritionDataQuality(product.DataQuality.Quality),
		NutritionCoverageScore:            pgvalue.Numeric(product.DataQuality.Coverage),
		SourceKind:                        sourceKindToRow(product.DataQuality.SourceKind),
		SourceNote:                        product.DataQuality.SourceNote,
		LastVerifiedAt:                    timestamp(product.DataQuality.LastVerifiedAt),
	}
}

func updateProductParams(product catalogdom.Product) sqlc.UpdateProductParams {
	params := productParams(product)
	return sqlc.UpdateProductParams{
		Brand: params.Brand, Name: params.Name, ProteinPer100g: params.ProteinPer100g, FatPer100g: params.FatPer100g,
		CarbPer100g: params.CarbPer100g, KcalPer100g: params.KcalPer100g, PricePer100g: params.PricePer100g,
		DefaultUnit: params.DefaultUnit, Purchasable: params.Purchasable, Notes: params.Notes,
		SaturatedFatPer100g: params.SaturatedFatPer100g, TransFatPer100g: params.TransFatPer100g,
		IndustrialTransFatPer100g: params.IndustrialTransFatPer100g, FiberPer100g: params.FiberPer100g,
		SodiumMgPer100g: params.SodiumMgPer100g, SaltGPer100g: params.SaltGPer100g,
		TotalSugarsPer100g: params.TotalSugarsPer100g, AddedSugarsPer100g: params.AddedSugarsPer100g,
		FreeSugarsPer100g:                 params.FreeSugarsPer100g,
		ContainsPartiallyHydrogenatedOils: params.ContainsPartiallyHydrogenatedOils,
		NutritionDataQuality:              params.NutritionDataQuality, NutritionCoverageScore: params.NutritionCoverageScore,
		SourceKind: params.SourceKind, SourceNote: params.SourceNote, LastVerifiedAt: params.LastVerifiedAt, ID: int64(product.ID),
	}
}

func classificationParams(productID int64, classification catalogdom.FoodClassification) sqlc.UpsertProductFoodClassificationParams {
	return sqlc.UpsertProductFoodClassificationParams{
		ProductID: productID, FoodGroup: foodGroupToRow(classification.FoodGroup),
		ProcessingLevel:      sqlc.ProcessingLevel(classification.ProcessingLevel),
		ProcessingConfidence: pgvalue.OptionalNumeric(classification.ProcessingConfidence),
		CountsToFruitVeg:     classification.CountsToFruitVeg, IsLegume: classification.IsLegume,
		IsWholeGrain: classification.IsWholeGrain, WholeGrainRatio: pgvalue.OptionalNumeric(classification.WholeGrainRatio),
		IsRedMeat: classification.IsRedMeat, IsProcessedMeat: classification.IsProcessedMeat,
		ClassificationSource: classification.Source, ClassificationNote: classification.Note,
	}
}

func productFromRow(row sqlc.Product) (catalogdom.Product, error) {
	protein, err := pgvalue.Float64(row.ProteinPer100g)
	if err != nil {
		return catalogdom.Product{}, err
	}
	fat, err := pgvalue.Float64(row.FatPer100g)
	if err != nil {
		return catalogdom.Product{}, err
	}
	carb, err := pgvalue.Float64(row.CarbPer100g)
	if err != nil {
		return catalogdom.Product{}, err
	}
	kcal, err := pgvalue.Float64(row.KcalPer100g)
	if err != nil {
		return catalogdom.Product{}, err
	}
	price, err := pgvalue.OptionalFloat64(row.PricePer100g)
	if err != nil {
		return catalogdom.Product{}, err
	}
	coverage, err := pgvalue.OptionalFloat64(row.NutritionCoverageScore)
	if err != nil {
		return catalogdom.Product{}, err
	}
	nutrition, err := nutritionFactsFromRow(row)
	if err != nil {
		return catalogdom.Product{}, err
	}
	return catalogdom.Product{
		ID: catalogdom.ProductID(row.ID), Brand: row.Brand, Name: row.Name,
		ProteinPer100G: protein, FatPer100G: fat, CarbPer100G: carb, KcalPer100G: kcal,
		PricePer100G: price, DefaultUnit: row.DefaultUnit, Purchasable: row.Purchasable, Notes: row.Notes,
		Nutrition:   nutrition,
		DataQuality: catalogdom.NutritionData{Quality: catalogdom.NutritionDataQuality(row.NutritionDataQuality), Coverage: derefFloat64(coverage), SourceKind: sourceKindFromRow(row.SourceKind), SourceNote: row.SourceNote, LastVerifiedAt: timestampFromRow(row.LastVerifiedAt)},
	}, nil
}

func nutritionFactsFromRow(row sqlc.Product) (catalogdom.NutritionFacts, error) {
	saturatedFat, err := pgvalue.OptionalFloat64(row.SaturatedFatPer100g)
	if err != nil {
		return catalogdom.NutritionFacts{}, err
	}
	transFat, err := pgvalue.OptionalFloat64(row.TransFatPer100g)
	if err != nil {
		return catalogdom.NutritionFacts{}, err
	}
	industrialTransFat, err := pgvalue.OptionalFloat64(row.IndustrialTransFatPer100g)
	if err != nil {
		return catalogdom.NutritionFacts{}, err
	}
	fiber, err := pgvalue.OptionalFloat64(row.FiberPer100g)
	if err != nil {
		return catalogdom.NutritionFacts{}, err
	}
	sodium, err := pgvalue.OptionalFloat64(row.SodiumMgPer100g)
	if err != nil {
		return catalogdom.NutritionFacts{}, err
	}
	salt, err := pgvalue.OptionalFloat64(row.SaltGPer100g)
	if err != nil {
		return catalogdom.NutritionFacts{}, err
	}
	totalSugars, err := pgvalue.OptionalFloat64(row.TotalSugarsPer100g)
	if err != nil {
		return catalogdom.NutritionFacts{}, err
	}
	addedSugars, err := pgvalue.OptionalFloat64(row.AddedSugarsPer100g)
	if err != nil {
		return catalogdom.NutritionFacts{}, err
	}
	freeSugars, err := pgvalue.OptionalFloat64(row.FreeSugarsPer100g)
	if err != nil {
		return catalogdom.NutritionFacts{}, err
	}
	return catalogdom.NutritionFacts{
		SaturatedFatPer100G:               saturatedFat,
		TransFatPer100G:                   transFat,
		IndustrialTransFatPer100G:         industrialTransFat,
		FiberPer100G:                      fiber,
		SodiumMgPer100G:                   sodium,
		SaltGPer100G:                      salt,
		TotalSugarsPer100G:                totalSugars,
		AddedSugarsPer100G:                addedSugars,
		FreeSugarsPer100G:                 freeSugars,
		ContainsPartiallyHydrogenatedOils: row.ContainsPartiallyHydrogenatedOils,
	}, nil
}

func classificationFromRow(row sqlc.ProductFoodClassification) (catalogdom.FoodClassification, error) {
	confidence, err := pgvalue.OptionalFloat64(row.ProcessingConfidence)
	if err != nil {
		return catalogdom.FoodClassification{}, err
	}
	ratio, err := pgvalue.OptionalFloat64(row.WholeGrainRatio)
	if err != nil {
		return catalogdom.FoodClassification{}, err
	}
	return catalogdom.FoodClassification{
		FoodGroup: foodGroupFromRow(row.FoodGroup), ProcessingLevel: catalogdom.ProcessingLevel(row.ProcessingLevel),
		ProcessingConfidence: confidence, CountsToFruitVeg: row.CountsToFruitVeg, IsLegume: row.IsLegume,
		IsWholeGrain: row.IsWholeGrain, WholeGrainRatio: ratio, IsRedMeat: row.IsRedMeat,
		IsProcessedMeat: row.IsProcessedMeat, Source: row.ClassificationSource, Note: row.ClassificationNote,
	}, nil
}

func roleTagFromRow(row sqlc.RoleTag) catalogdom.RoleTag {
	return catalogdom.RoleTag{ID: row.ID, Code: row.Code, Kind: catalogdom.RoleKind(row.Kind), Label: row.Label, ParentID: row.ParentID}
}

func derefFloat64(value *float64) float64 {
	if value == nil {
		return 0
	}
	return *value
}

func timestamp(value *time.Time) pgtype.Timestamptz {
	if value == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *value, Valid: true}
}

func timestampFromRow(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	result := value.Time
	return &result
}

func sourceKindToRow(value *catalogdom.SourceKind) *sqlc.SourceKind {
	if value == nil {
		return nil
	}
	result := sqlc.SourceKind(*value)
	return &result
}

func sourceKindFromRow(value *sqlc.SourceKind) *catalogdom.SourceKind {
	if value == nil {
		return nil
	}
	result := catalogdom.SourceKind(*value)
	return &result
}

func foodGroupToRow(value *catalogdom.FoodGroup) *sqlc.FoodGroup {
	if value == nil {
		return nil
	}
	result := sqlc.FoodGroup(*value)
	return &result
}

func foodGroupFromRow(value *sqlc.FoodGroup) *catalogdom.FoodGroup {
	if value == nil {
		return nil
	}
	result := catalogdom.FoodGroup(*value)
	return &result
}
