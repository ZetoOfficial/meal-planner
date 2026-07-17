package repositories

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	catalogdom "github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
	apperr "github.com/ZetoOfficial/meal-planner/internal/errors"
	"github.com/ZetoOfficial/meal-planner/internal/postgres/sqlc"
)

type Catalog struct {
	repositories *Repositories
}

func NewCatalog(repositories *Repositories) *Catalog {
	return &Catalog{repositories: repositories}
}

func (r *Catalog) Create(ctx context.Context, product catalogdom.Product) (catalogdom.Product, error) {
	query := r.repositories.Queries(ctx)
	stored, err := query.CreateProduct(ctx, productParams(product))
	if err != nil {
		return catalogdom.Product{}, mapCatalogError(err, "product")
	}
	product.ID = catalogdom.ProductID(stored.ID)
	if err := r.writeAggregate(ctx, query, product); err != nil {
		return catalogdom.Product{}, err
	}
	return r.load(ctx, query, product.ID)
}

func (r *Catalog) List(ctx context.Context, roleCode string) ([]catalogdom.Product, error) {
	query := r.repositories.Queries(ctx)
	var (
		rows []sqlc.Product
		err  error
	)
	if roleCode == "" {
		rows, err = query.ListProducts(ctx)
	} else {
		rows, err = query.ListProductsByRole(ctx, roleCode)
	}
	if err != nil {
		return nil, mapCatalogError(err, "product")
	}
	products := make([]catalogdom.Product, 0, len(rows))
	for i := range rows {
		product, err := r.load(ctx, query, catalogdom.ProductID(rows[i].ID))
		if err != nil {
			return nil, err
		}
		products = append(products, product)
	}
	return products, nil
}

func (r *Catalog) Get(ctx context.Context, id catalogdom.ProductID) (catalogdom.Product, error) {
	return r.load(ctx, r.repositories.Queries(ctx), id)
}

func (r *Catalog) Update(ctx context.Context, product catalogdom.Product) (catalogdom.Product, error) {
	query := r.repositories.Queries(ctx)
	if _, err := query.UpdateProduct(ctx, updateProductParams(product)); err != nil {
		return catalogdom.Product{}, mapCatalogError(err, "product")
	}
	if err := r.writeAggregate(ctx, query, product); err != nil {
		return catalogdom.Product{}, err
	}
	return r.load(ctx, query, product.ID)
}

func (r *Catalog) Delete(ctx context.Context, id catalogdom.ProductID) error {
	deleted, err := r.repositories.Queries(ctx).DeleteProduct(ctx, int64(id))
	if err != nil {
		return mapCatalogError(err, "product")
	}
	if deleted == 0 {
		return apperr.NotFound("product")
	}
	return nil
}

func (r *Catalog) GetRoleTag(ctx context.Context, code string) (catalogdom.RoleTag, error) {
	row, err := r.repositories.Queries(ctx).GetRoleTagByCode(ctx, code)
	if err != nil {
		return catalogdom.RoleTag{}, mapCatalogError(err, "role tag")
	}
	return roleTagFromRow(row), nil
}

func (r *Catalog) ListRoleTags(ctx context.Context) ([]catalogdom.RoleTag, error) {
	rows, err := r.repositories.Queries(ctx).ListRoleTags(ctx)
	if err != nil {
		return nil, mapCatalogError(err, "role tag")
	}
	tags := make([]catalogdom.RoleTag, 0, len(rows))
	for _, row := range rows {
		tags = append(tags, roleTagFromRow(row))
	}
	return tags, nil
}

func (r *Catalog) writeAggregate(ctx context.Context, query *sqlc.Queries, product catalogdom.Product) error {
	id := int64(product.ID)
	if _, err := query.UpsertProductFoodClassification(ctx, classificationParams(id, product.Classification)); err != nil {
		return mapCatalogError(err, "product classification")
	}
	if err := query.DeleteProductRoles(ctx, id); err != nil {
		return mapCatalogError(err, "product roles")
	}
	for _, role := range product.Roles {
		if err := query.CreateProductRole(ctx, sqlc.CreateProductRoleParams{ProductID: id, RoleTagID: role.ID}); err != nil {
			return mapCatalogError(err, "role tag")
		}
	}
	if err := query.DeleteProductAllergens(ctx, id); err != nil {
		return mapCatalogError(err, "product allergens")
	}
	for _, allergen := range product.Allergens {
		if err := query.CreateProductAllergen(ctx, sqlc.CreateProductAllergenParams{ProductID: id, Allergen: string(allergen)}); err != nil {
			return mapCatalogError(err, "product allergen")
		}
	}
	return nil
}

func (r *Catalog) load(ctx context.Context, query *sqlc.Queries, id catalogdom.ProductID) (catalogdom.Product, error) {
	row, err := query.GetProduct(ctx, int64(id))
	if err != nil {
		return catalogdom.Product{}, mapCatalogError(err, "product")
	}
	product, err := productFromRow(row)
	if err != nil {
		return catalogdom.Product{}, err
	}
	roles, err := query.ListProductRoleTags(ctx, int64(id))
	if err != nil {
		return catalogdom.Product{}, mapCatalogError(err, "product roles")
	}
	product.Roles = make([]catalogdom.RoleTag, 0, len(roles))
	for _, role := range roles {
		product.Roles = append(product.Roles, roleTagFromRow(role))
	}
	allergens, err := query.ListProductAllergens(ctx, int64(id))
	if err != nil {
		return catalogdom.Product{}, mapCatalogError(err, "product allergens")
	}
	product.Allergens = make([]catalogdom.Allergen, len(allergens))
	for i, allergen := range allergens {
		product.Allergens[i] = catalogdom.Allergen(allergen)
	}
	classification, err := query.GetProductFoodClassification(ctx, int64(id))
	if err != nil {
		return catalogdom.Product{}, mapCatalogError(err, "product classification")
	}
	product.Classification, err = classificationFromRow(classification)
	if err != nil {
		return catalogdom.Product{}, err
	}
	return product, nil
}

func productParams(product catalogdom.Product) sqlc.CreateProductParams {
	return sqlc.CreateProductParams{
		Brand:                             product.Brand,
		Name:                              product.Name,
		ProteinPer100g:                    numeric(product.ProteinPer100G),
		FatPer100g:                        numeric(product.FatPer100G),
		CarbPer100g:                       numeric(product.CarbPer100G),
		KcalPer100g:                       numeric(product.KcalPer100G),
		PricePer100g:                      optionalNumeric(product.PricePer100G),
		DefaultUnit:                       product.DefaultUnit,
		Purchasable:                       product.Purchasable,
		Notes:                             product.Notes,
		SaturatedFatPer100g:               optionalNumeric(product.Nutrition.SaturatedFatPer100G),
		TransFatPer100g:                   optionalNumeric(product.Nutrition.TransFatPer100G),
		IndustrialTransFatPer100g:         optionalNumeric(product.Nutrition.IndustrialTransFatPer100G),
		FiberPer100g:                      optionalNumeric(product.Nutrition.FiberPer100G),
		SodiumMgPer100g:                   optionalNumeric(product.Nutrition.SodiumMgPer100G),
		SaltGPer100g:                      optionalNumeric(product.Nutrition.SaltGPer100G),
		TotalSugarsPer100g:                optionalNumeric(product.Nutrition.TotalSugarsPer100G),
		AddedSugarsPer100g:                optionalNumeric(product.Nutrition.AddedSugarsPer100G),
		FreeSugarsPer100g:                 optionalNumeric(product.Nutrition.FreeSugarsPer100G),
		ContainsPartiallyHydrogenatedOils: product.Nutrition.ContainsPartiallyHydrogenatedOils,
		NutritionDataQuality:              sqlc.NutritionDataQuality(product.DataQuality.Quality),
		NutritionCoverageScore:            numeric(product.DataQuality.Coverage),
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
		ProductID: productID, FoodGroup: sqlc.FoodGroup(classification.FoodGroup),
		ProcessingLevel:      sqlc.ProcessingLevel(classification.ProcessingLevel),
		ProcessingConfidence: optionalNumeric(classification.ProcessingConfidence),
		CountsToFruitVeg:     classification.CountsToFruitVeg, IsLegume: classification.IsLegume,
		IsWholeGrain: classification.IsWholeGrain, WholeGrainRatio: optionalNumeric(classification.WholeGrainRatio),
		IsRedMeat: classification.IsRedMeat, IsProcessedMeat: classification.IsProcessedMeat,
		ClassificationSource: classification.Source, ClassificationNote: classification.Note,
	}
}

func productFromRow(row sqlc.Product) (catalogdom.Product, error) {
	protein, err := float64FromNumeric(row.ProteinPer100g)
	if err != nil {
		return catalogdom.Product{}, err
	}
	fat, err := float64FromNumeric(row.FatPer100g)
	if err != nil {
		return catalogdom.Product{}, err
	}
	carb, err := float64FromNumeric(row.CarbPer100g)
	if err != nil {
		return catalogdom.Product{}, err
	}
	kcal, err := float64FromNumeric(row.KcalPer100g)
	if err != nil {
		return catalogdom.Product{}, err
	}
	price, err := optionalFloat64FromNumeric(row.PricePer100g)
	if err != nil {
		return catalogdom.Product{}, err
	}
	coverage, err := optionalFloat64FromNumeric(row.NutritionCoverageScore)
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
	saturatedFat, err := optionalFloat64FromNumeric(row.SaturatedFatPer100g)
	if err != nil {
		return catalogdom.NutritionFacts{}, err
	}
	transFat, err := optionalFloat64FromNumeric(row.TransFatPer100g)
	if err != nil {
		return catalogdom.NutritionFacts{}, err
	}
	industrialTransFat, err := optionalFloat64FromNumeric(row.IndustrialTransFatPer100g)
	if err != nil {
		return catalogdom.NutritionFacts{}, err
	}
	fiber, err := optionalFloat64FromNumeric(row.FiberPer100g)
	if err != nil {
		return catalogdom.NutritionFacts{}, err
	}
	sodium, err := optionalFloat64FromNumeric(row.SodiumMgPer100g)
	if err != nil {
		return catalogdom.NutritionFacts{}, err
	}
	salt, err := optionalFloat64FromNumeric(row.SaltGPer100g)
	if err != nil {
		return catalogdom.NutritionFacts{}, err
	}
	totalSugars, err := optionalFloat64FromNumeric(row.TotalSugarsPer100g)
	if err != nil {
		return catalogdom.NutritionFacts{}, err
	}
	addedSugars, err := optionalFloat64FromNumeric(row.AddedSugarsPer100g)
	if err != nil {
		return catalogdom.NutritionFacts{}, err
	}
	freeSugars, err := optionalFloat64FromNumeric(row.FreeSugarsPer100g)
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
	confidence, err := optionalFloat64FromNumeric(row.ProcessingConfidence)
	if err != nil {
		return catalogdom.FoodClassification{}, err
	}
	ratio, err := optionalFloat64FromNumeric(row.WholeGrainRatio)
	if err != nil {
		return catalogdom.FoodClassification{}, err
	}
	return catalogdom.FoodClassification{
		FoodGroup: catalogdom.FoodGroup(row.FoodGroup), ProcessingLevel: catalogdom.ProcessingLevel(row.ProcessingLevel),
		ProcessingConfidence: confidence, CountsToFruitVeg: row.CountsToFruitVeg, IsLegume: row.IsLegume,
		IsWholeGrain: row.IsWholeGrain, WholeGrainRatio: ratio, IsRedMeat: row.IsRedMeat,
		IsProcessedMeat: row.IsProcessedMeat, Source: row.ClassificationSource, Note: row.ClassificationNote,
	}, nil
}

func roleTagFromRow(row sqlc.RoleTag) catalogdom.RoleTag {
	return catalogdom.RoleTag{ID: row.ID, Code: row.Code, Kind: catalogdom.RoleKind(row.Kind), Label: row.Label, ParentID: row.ParentID}
}

func numeric(value float64) pgtype.Numeric {
	var number pgtype.Numeric
	if err := number.Scan(strconv.FormatFloat(value, 'f', -1, 64)); err != nil {
		panic(fmt.Sprintf("convert numeric: %v", err))
	}
	return number
}

func optionalNumeric(value *float64) pgtype.Numeric {
	if value == nil {
		return pgtype.Numeric{}
	}
	return numeric(*value)
}

func float64FromNumeric(value pgtype.Numeric) (float64, error) {
	converted, err := value.Float64Value()
	if err != nil {
		return 0, fmt.Errorf("convert numeric to float64: %w", err)
	}
	return converted.Float64, nil
}

func optionalFloat64FromNumeric(value pgtype.Numeric) (*float64, error) {
	if !value.Valid {
		return nil, nil
	}
	converted, err := float64FromNumeric(value)
	if err != nil {
		return nil, err
	}
	return &converted, nil
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

func mapCatalogError(err error, resource string) error {
	var appError *apperr.Error
	if err == nil || errors.As(err, &appError) {
		return err
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return apperr.NotFound(resource)
	}
	var databaseError *pgconn.PgError
	if errors.As(err, &databaseError) {
		switch databaseError.Code {
		case "23505":
			return apperr.Conflict(resource + " already exists")
		case "23503":
			return apperr.NotFound(resource)
		}
	}
	return err
}
