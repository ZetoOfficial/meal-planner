package repositories

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	catalogdom "github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
	apperr "github.com/ZetoOfficial/meal-planner/internal/errors"
	"github.com/ZetoOfficial/meal-planner/internal/postgres/sqlc"
	cataloguc "github.com/ZetoOfficial/meal-planner/internal/usecase/catalog"
)

type txStarter interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

type ProductRepository struct {
	db sqlc.DBTX
	q  *sqlc.Queries
}

func NewProductRepository(db sqlc.DBTX) *ProductRepository {
	return &ProductRepository{db: db, q: sqlc.New(db)}
}

func (r *ProductRepository) List(ctx context.Context, filter cataloguc.ProductFilter) ([]catalogdom.Product, error) {
	var rows []sqlc.Product
	var err error
	if filter.Role != nil && *filter.Role != "" {
		rows, err = r.q.ListProductsByRole(ctx, *filter.Role)
	} else {
		rows, err = r.q.ListProducts(ctx)
	}
	if err != nil {
		return nil, mapProductError(err)
	}
	return r.hydrateProducts(ctx, rows)
}

func (r *ProductRepository) ListUsable(ctx context.Context) ([]catalogdom.Product, error) {
	return r.List(ctx, cataloguc.ProductFilter{})
}

func (r *ProductRepository) Get(ctx context.Context, id catalogdom.ProductID) (catalogdom.Product, error) {
	row, err := r.q.GetProduct(ctx, int64(id))
	if err != nil {
		return catalogdom.Product{}, mapProductError(err)
	}
	return r.hydrateProduct(ctx, row)
}

func (r *ProductRepository) Create(ctx context.Context, p catalogdom.Product) (catalogdom.Product, error) {
	var created catalogdom.Product
	err := r.withTx(ctx, func(q *sqlc.Queries) error {
		row, err := q.CreateProduct(ctx, createProductParams(p))
		if err != nil {
			return err
		}
		if err := replaceProductRelations(ctx, q, row.ID, p); err != nil {
			return err
		}
		created, err = hydrateProduct(ctx, q, row)
		return err
	})
	if err != nil {
		return catalogdom.Product{}, mapProductError(err)
	}
	return created, nil
}

func (r *ProductRepository) Update(ctx context.Context, p catalogdom.Product) (catalogdom.Product, error) {
	var updated catalogdom.Product
	err := r.withTx(ctx, func(q *sqlc.Queries) error {
		row, err := q.UpdateProduct(ctx, updateProductParams(p))
		if err != nil {
			return err
		}
		if err := replaceProductRelations(ctx, q, row.ID, p); err != nil {
			return err
		}
		updated, err = hydrateProduct(ctx, q, row)
		return err
	})
	if err != nil {
		return catalogdom.Product{}, mapProductError(err)
	}
	return updated, nil
}

func (r *ProductRepository) Delete(ctx context.Context, id catalogdom.ProductID) error {
	rows, err := r.q.DeleteProduct(ctx, int64(id))
	if err != nil {
		return mapProductError(err)
	}
	if rows == 0 {
		return apperr.NotFound("product")
	}
	return nil
}

func (r *ProductRepository) ListRoleTags(ctx context.Context) ([]catalogdom.RoleTag, error) {
	rows, err := r.q.ListRoleTags(ctx)
	if err != nil {
		return nil, mapProductError(err)
	}
	out := make([]catalogdom.RoleTag, 0, len(rows))
	for _, row := range rows {
		out = append(out, roleTagFromRow(row))
	}
	return out, nil
}

func (r *ProductRepository) hydrateProducts(ctx context.Context, rows []sqlc.Product) ([]catalogdom.Product, error) {
	products := make([]catalogdom.Product, 0, len(rows))
	for i := range rows {
		product, err := r.hydrateProduct(ctx, rows[i])
		if err != nil {
			return nil, err
		}
		products = append(products, product)
	}
	return products, nil
}

func (r *ProductRepository) hydrateProduct(ctx context.Context, row sqlc.Product) (catalogdom.Product, error) {
	return hydrateProduct(ctx, r.q, row)
}

func (r *ProductRepository) withTx(ctx context.Context, fn func(q *sqlc.Queries) error) error {
	starter, ok := r.db.(txStarter)
	if !ok {
		return fn(r.q)
	}
	tx, err := starter.Begin(ctx)
	if err != nil {
		return err
	}
	if err := fn(sqlc.New(tx)); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	return tx.Commit(ctx)
}

func hydrateProduct(ctx context.Context, q *sqlc.Queries, row sqlc.Product) (catalogdom.Product, error) {
	product, err := productFromRow(row)
	if err != nil {
		return catalogdom.Product{}, err
	}
	roles, err := q.ListProductRoles(ctx, row.ID)
	if err != nil {
		return catalogdom.Product{}, err
	}
	allergens, err := q.ListProductAllergens(ctx, row.ID)
	if err != nil {
		return catalogdom.Product{}, err
	}
	product.Roles = roles
	product.Allergens = allergens

	classification, err := q.GetProductFoodClassification(ctx, row.ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return product, nil
		}
		return catalogdom.Product{}, err
	}
	product.Classification, err = classificationFromRow(classification)
	if err != nil {
		return catalogdom.Product{}, err
	}
	return product, nil
}

func replaceProductRelations(ctx context.Context, q *sqlc.Queries, productID int64, p catalogdom.Product) error {
	if err := q.DeleteProductRoles(ctx, productID); err != nil {
		return err
	}
	for _, code := range p.Roles {
		role, err := q.UpsertRoleTag(ctx, sqlc.UpsertRoleTagParams{
			Code:  code,
			Kind:  sqlc.RoleKind(roleKindFromCode(code)),
			Label: code,
		})
		if err != nil {
			return err
		}
		if err := q.AddProductRole(ctx, sqlc.AddProductRoleParams{ProductID: productID, RoleTagID: role.ID}); err != nil {
			return err
		}
	}

	if err := q.DeleteProductAllergens(ctx, productID); err != nil {
		return err
	}
	for _, allergen := range p.Allergens {
		if err := q.AddProductAllergen(ctx, sqlc.AddProductAllergenParams{ProductID: productID, Allergen: allergen}); err != nil {
			return err
		}
	}

	if p.Classification == nil {
		return q.DeleteProductFoodClassification(ctx, productID)
	}
	_, err := q.UpsertProductFoodClassification(ctx, classificationParams(productID, *p.Classification))
	return err
}

func createProductParams(p catalogdom.Product) sqlc.CreateProductParams {
	base := productWriteParams(p)
	return sqlc.CreateProductParams{
		Brand:                             base.Brand,
		Name:                              base.Name,
		ProteinPer100g:                    base.ProteinPer100g,
		FatPer100g:                        base.FatPer100g,
		CarbPer100g:                       base.CarbPer100g,
		KcalPer100g:                       base.KcalPer100g,
		PricePer100g:                      base.PricePer100g,
		DefaultUnit:                       base.DefaultUnit,
		Purchasable:                       base.Purchasable,
		SaturatedFatPer100g:               base.SaturatedFatPer100g,
		TransFatPer100g:                   base.TransFatPer100g,
		IndustrialTransFatPer100g:         base.IndustrialTransFatPer100g,
		FiberPer100g:                      base.FiberPer100g,
		SodiumMgPer100g:                   base.SodiumMgPer100g,
		SaltGPer100g:                      base.SaltGPer100g,
		TotalSugarsPer100g:                base.TotalSugarsPer100g,
		AddedSugarsPer100g:                base.AddedSugarsPer100g,
		FreeSugarsPer100g:                 base.FreeSugarsPer100g,
		ContainsPartiallyHydrogenatedOils: base.ContainsPartiallyHydrogenatedOils,
		NutritionDataQuality:              base.NutritionDataQuality,
		NutritionCoverageScore:            base.NutritionCoverageScore,
		SourceKind:                        base.SourceKind,
		SourceNote:                        base.SourceNote,
		LastVerifiedAt:                    base.LastVerifiedAt,
	}
}

func updateProductParams(p catalogdom.Product) sqlc.UpdateProductParams {
	base := productWriteParams(p)
	return sqlc.UpdateProductParams{
		ID:                                int64(p.ID),
		Brand:                             base.Brand,
		Name:                              base.Name,
		ProteinPer100g:                    base.ProteinPer100g,
		FatPer100g:                        base.FatPer100g,
		CarbPer100g:                       base.CarbPer100g,
		KcalPer100g:                       base.KcalPer100g,
		PricePer100g:                      base.PricePer100g,
		DefaultUnit:                       base.DefaultUnit,
		Purchasable:                       base.Purchasable,
		SaturatedFatPer100g:               base.SaturatedFatPer100g,
		TransFatPer100g:                   base.TransFatPer100g,
		IndustrialTransFatPer100g:         base.IndustrialTransFatPer100g,
		FiberPer100g:                      base.FiberPer100g,
		SodiumMgPer100g:                   base.SodiumMgPer100g,
		SaltGPer100g:                      base.SaltGPer100g,
		TotalSugarsPer100g:                base.TotalSugarsPer100g,
		AddedSugarsPer100g:                base.AddedSugarsPer100g,
		FreeSugarsPer100g:                 base.FreeSugarsPer100g,
		ContainsPartiallyHydrogenatedOils: base.ContainsPartiallyHydrogenatedOils,
		NutritionDataQuality:              base.NutritionDataQuality,
		NutritionCoverageScore:            base.NutritionCoverageScore,
		SourceKind:                        base.SourceKind,
		SourceNote:                        base.SourceNote,
		LastVerifiedAt:                    base.LastVerifiedAt,
	}
}

type productWrite struct {
	Brand                             string
	Name                              string
	ProteinPer100g                    pgtype.Numeric
	FatPer100g                        pgtype.Numeric
	CarbPer100g                       pgtype.Numeric
	KcalPer100g                       pgtype.Numeric
	PricePer100g                      pgtype.Numeric
	DefaultUnit                       string
	Purchasable                       bool
	SaturatedFatPer100g               pgtype.Numeric
	TransFatPer100g                   pgtype.Numeric
	IndustrialTransFatPer100g         pgtype.Numeric
	FiberPer100g                      pgtype.Numeric
	SodiumMgPer100g                   pgtype.Numeric
	SaltGPer100g                      pgtype.Numeric
	TotalSugarsPer100g                pgtype.Numeric
	AddedSugarsPer100g                pgtype.Numeric
	FreeSugarsPer100g                 pgtype.Numeric
	ContainsPartiallyHydrogenatedOils *bool
	NutritionDataQuality              sqlc.NutritionDataQuality
	NutritionCoverageScore            pgtype.Numeric
	SourceKind                        *sqlc.SourceKind
	SourceNote                        *string
	LastVerifiedAt                    pgtype.Timestamptz
}

func productWriteParams(p catalogdom.Product) productWrite {
	nutrition := p.Nutrition
	dataQuality := p.DataQuality
	quality := sqlc.NutritionDataQualityUnknown
	var coverage *float64
	var sourceKind *sqlc.SourceKind
	var sourceNote *string
	var lastVerifiedAt *time.Time
	if dataQuality != nil {
		if dataQuality.NutritionDataQuality != nil {
			quality = sqlc.NutritionDataQuality(*dataQuality.NutritionDataQuality)
		}
		coverage = dataQuality.NutritionCoverageScore
		if dataQuality.SourceKind != nil {
			value := sqlc.SourceKind(*dataQuality.SourceKind)
			sourceKind = &value
		}
		sourceNote = dataQuality.SourceNote
		lastVerifiedAt = dataQuality.LastVerifiedAt
	}
	return productWrite{
		Brand:                             p.Brand,
		Name:                              p.Name,
		ProteinPer100g:                    numeric(p.ProteinPer100g),
		FatPer100g:                        numeric(p.FatPer100g),
		CarbPer100g:                       numeric(p.CarbPer100g),
		KcalPer100g:                       numeric(p.KcalPer100g),
		PricePer100g:                      nullableNumeric(p.PricePer100g),
		DefaultUnit:                       p.DefaultUnit,
		Purchasable:                       p.Purchasable,
		SaturatedFatPer100g:               nullableNumeric(nutritionFloat(nutrition, func(n *catalogdom.NutritionFacts) *float64 { return n.SaturatedFatPer100g })),
		TransFatPer100g:                   nullableNumeric(nutritionFloat(nutrition, func(n *catalogdom.NutritionFacts) *float64 { return n.TransFatPer100g })),
		IndustrialTransFatPer100g:         nullableNumeric(nutritionFloat(nutrition, func(n *catalogdom.NutritionFacts) *float64 { return n.IndustrialTransFatPer100g })),
		FiberPer100g:                      nullableNumeric(nutritionFloat(nutrition, func(n *catalogdom.NutritionFacts) *float64 { return n.FiberPer100g })),
		SodiumMgPer100g:                   nullableNumeric(nutritionFloat(nutrition, func(n *catalogdom.NutritionFacts) *float64 { return n.SodiumMgPer100g })),
		SaltGPer100g:                      nullableNumeric(nutritionFloat(nutrition, func(n *catalogdom.NutritionFacts) *float64 { return n.SaltGPer100g })),
		TotalSugarsPer100g:                nullableNumeric(nutritionFloat(nutrition, func(n *catalogdom.NutritionFacts) *float64 { return n.TotalSugarsPer100g })),
		AddedSugarsPer100g:                nullableNumeric(nutritionFloat(nutrition, func(n *catalogdom.NutritionFacts) *float64 { return n.AddedSugarsPer100g })),
		FreeSugarsPer100g:                 nullableNumeric(nutritionFloat(nutrition, func(n *catalogdom.NutritionFacts) *float64 { return n.FreeSugarsPer100g })),
		ContainsPartiallyHydrogenatedOils: nutritionBool(nutrition, func(n *catalogdom.NutritionFacts) *bool { return n.ContainsPartiallyHydrogenatedOils }),
		NutritionDataQuality:              quality,
		NutritionCoverageScore:            nullableNumeric(coverage),
		SourceKind:                        sourceKind,
		SourceNote:                        sourceNote,
		LastVerifiedAt:                    nullableTime(lastVerifiedAt),
	}
}

func productFromRow(row sqlc.Product) (catalogdom.Product, error) {
	protein, err := numericFloat64(row.ProteinPer100g)
	if err != nil {
		return catalogdom.Product{}, err
	}
	fat, err := numericFloat64(row.FatPer100g)
	if err != nil {
		return catalogdom.Product{}, err
	}
	carb, err := numericFloat64(row.CarbPer100g)
	if err != nil {
		return catalogdom.Product{}, err
	}
	kcal, err := numericFloat64(row.KcalPer100g)
	if err != nil {
		return catalogdom.Product{}, err
	}
	price, err := nullableFloat64(row.PricePer100g)
	if err != nil {
		return catalogdom.Product{}, err
	}
	nutrition, err := nutritionFromRow(row)
	if err != nil {
		return catalogdom.Product{}, err
	}
	dataQuality, err := dataQualityFromRow(row)
	if err != nil {
		return catalogdom.Product{}, err
	}
	return catalogdom.Product{
		ID:             catalogdom.ProductID(row.ID),
		Brand:          row.Brand,
		Name:           row.Name,
		ProteinPer100g: protein,
		FatPer100g:     fat,
		CarbPer100g:    carb,
		KcalPer100g:    kcal,
		PricePer100g:   price,
		DefaultUnit:    row.DefaultUnit,
		Purchasable:    row.Purchasable,
		Nutrition:      nutrition,
		DataQuality:    dataQuality,
	}, nil
}

func nutritionFromRow(row sqlc.Product) (*catalogdom.NutritionFacts, error) {
	values := []pgtype.Numeric{
		row.SaturatedFatPer100g,
		row.TransFatPer100g,
		row.IndustrialTransFatPer100g,
		row.FiberPer100g,
		row.SodiumMgPer100g,
		row.SaltGPer100g,
		row.TotalSugarsPer100g,
		row.AddedSugarsPer100g,
		row.FreeSugarsPer100g,
	}
	hasValue := row.ContainsPartiallyHydrogenatedOils != nil
	for _, value := range values {
		hasValue = hasValue || value.Valid
	}
	if !hasValue {
		return nil, nil
	}
	saturatedFat, err := nullableFloat64(row.SaturatedFatPer100g)
	if err != nil {
		return nil, err
	}
	transFat, err := nullableFloat64(row.TransFatPer100g)
	if err != nil {
		return nil, err
	}
	industrialTransFat, err := nullableFloat64(row.IndustrialTransFatPer100g)
	if err != nil {
		return nil, err
	}
	fiber, err := nullableFloat64(row.FiberPer100g)
	if err != nil {
		return nil, err
	}
	sodium, err := nullableFloat64(row.SodiumMgPer100g)
	if err != nil {
		return nil, err
	}
	salt, err := nullableFloat64(row.SaltGPer100g)
	if err != nil {
		return nil, err
	}
	totalSugars, err := nullableFloat64(row.TotalSugarsPer100g)
	if err != nil {
		return nil, err
	}
	addedSugars, err := nullableFloat64(row.AddedSugarsPer100g)
	if err != nil {
		return nil, err
	}
	freeSugars, err := nullableFloat64(row.FreeSugarsPer100g)
	if err != nil {
		return nil, err
	}
	return &catalogdom.NutritionFacts{
		SaturatedFatPer100g:               saturatedFat,
		TransFatPer100g:                   transFat,
		IndustrialTransFatPer100g:         industrialTransFat,
		FiberPer100g:                      fiber,
		SodiumMgPer100g:                   sodium,
		SaltGPer100g:                      salt,
		TotalSugarsPer100g:                totalSugars,
		AddedSugarsPer100g:                addedSugars,
		FreeSugarsPer100g:                 freeSugars,
		ContainsPartiallyHydrogenatedOils: row.ContainsPartiallyHydrogenatedOils,
	}, nil
}

func dataQualityFromRow(row sqlc.Product) (*catalogdom.NutritionDataQualityMetadata, error) {
	quality := catalogdom.NutritionDataQuality(row.NutritionDataQuality)
	coverage, err := nullableFloat64(row.NutritionCoverageScore)
	if err != nil {
		return nil, err
	}
	var sourceKind *catalogdom.SourceKind
	if row.SourceKind != nil {
		value := catalogdom.SourceKind(*row.SourceKind)
		sourceKind = &value
	}
	var lastVerifiedAt *time.Time
	if row.LastVerifiedAt.Valid {
		lastVerifiedAt = &row.LastVerifiedAt.Time
	}
	return &catalogdom.NutritionDataQualityMetadata{
		NutritionDataQuality:   &quality,
		NutritionCoverageScore: coverage,
		SourceKind:             sourceKind,
		SourceNote:             row.SourceNote,
		LastVerifiedAt:         lastVerifiedAt,
	}, nil
}

func classificationFromRow(row sqlc.ProductFoodClassification) (*catalogdom.FoodClassification, error) {
	foodGroup := catalogdom.FoodGroup(row.FoodGroup)
	processingLevel := catalogdom.ProcessingLevel(row.ProcessingLevel)
	processingConfidence, err := nullableFloat64(row.ProcessingConfidence)
	if err != nil {
		return nil, err
	}
	wholeGrainRatio, err := nullableFloat64(row.WholeGrainRatio)
	if err != nil {
		return nil, err
	}
	return &catalogdom.FoodClassification{
		FoodGroup:            &foodGroup,
		ProcessingLevel:      &processingLevel,
		ProcessingConfidence: processingConfidence,
		CountsToFruitVeg:     &row.CountsToFruitVeg,
		IsLegume:             &row.IsLegume,
		IsWholeGrain:         &row.IsWholeGrain,
		WholeGrainRatio:      wholeGrainRatio,
		IsRedMeat:            &row.IsRedMeat,
		IsProcessedMeat:      &row.IsProcessedMeat,
		ClassificationNote:   row.ClassificationNote,
	}, nil
}

func classificationParams(productID int64, c catalogdom.FoodClassification) sqlc.UpsertProductFoodClassificationParams {
	foodGroup := sqlc.FoodGroupOther
	if c.FoodGroup != nil {
		foodGroup = sqlc.FoodGroup(*c.FoodGroup)
	}
	processingLevel := sqlc.ProcessingLevelUnknown
	if c.ProcessingLevel != nil {
		processingLevel = sqlc.ProcessingLevel(*c.ProcessingLevel)
	}
	return sqlc.UpsertProductFoodClassificationParams{
		ProductID:            productID,
		FoodGroup:            foodGroup,
		ProcessingLevel:      processingLevel,
		ProcessingConfidence: nullableNumeric(c.ProcessingConfidence),
		CountsToFruitVeg:     boolValue(c.CountsToFruitVeg),
		IsLegume:             boolValue(c.IsLegume),
		IsWholeGrain:         boolValue(c.IsWholeGrain),
		WholeGrainRatio:      nullableNumeric(c.WholeGrainRatio),
		IsRedMeat:            boolValue(c.IsRedMeat),
		IsProcessedMeat:      boolValue(c.IsProcessedMeat),
		ClassificationNote:   c.ClassificationNote,
	}
}

func roleTagFromRow(row sqlc.RoleTag) catalogdom.RoleTag {
	return catalogdom.RoleTag{
		ID:       row.ID,
		Code:     row.Code,
		Kind:     catalogdom.RoleKind(row.Kind),
		Label:    row.Label,
		ParentID: row.ParentID,
	}
}

func roleKindFromCode(code string) catalogdom.RoleKind {
	kind, _, _ := strings.Cut(code, ":")
	return catalogdom.RoleKind(kind)
}

func nutritionFloat(n *catalogdom.NutritionFacts, pick func(*catalogdom.NutritionFacts) *float64) *float64 {
	if n == nil {
		return nil
	}
	return pick(n)
}

func nutritionBool(n *catalogdom.NutritionFacts, pick func(*catalogdom.NutritionFacts) *bool) *bool {
	if n == nil {
		return nil
	}
	return pick(n)
}

func boolValue(value *bool) bool {
	return value != nil && *value
}

func numeric(value float64) pgtype.Numeric {
	return nullableNumeric(&value)
}

func nullableNumeric(value *float64) pgtype.Numeric {
	if value == nil {
		return pgtype.Numeric{}
	}
	var n pgtype.Numeric
	if err := n.Scan(fmt.Sprintf("%.6f", *value)); err != nil {
		panic(err)
	}
	return n
}

func nullableFloat64(n pgtype.Numeric) (*float64, error) {
	if !n.Valid {
		return nil, nil
	}
	value, err := numericFloat64(n)
	if err != nil {
		return nil, err
	}
	return &value, nil
}

func nullableTime(value *time.Time) pgtype.Timestamptz {
	if value == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *value, Valid: true}
}

func mapProductError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return apperr.NotFound("product")
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			return apperr.Conflict("product already exists")
		case "23503", "23514", "22P02":
			return apperr.Validation("invalid product", pgErr.Message)
		}
	}
	return err
}
