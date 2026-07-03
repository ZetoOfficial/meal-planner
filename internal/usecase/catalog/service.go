package catalog

import (
	"context"
	"fmt"
	"strings"

	catalogdom "github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
	apperr "github.com/ZetoOfficial/meal-planner/internal/errors"
)

type Service struct {
	products ProductRepository
}

func NewService(products ProductRepository) *Service {
	return &Service{products: products}
}

func (s *Service) ListProducts(ctx context.Context, filter ProductFilter) ([]catalogdom.Product, error) {
	return s.products.List(ctx, filter)
}

func (s *Service) GetProduct(ctx context.Context, id catalogdom.ProductID) (catalogdom.Product, error) {
	if id <= 0 {
		return catalogdom.Product{}, apperr.Validation("product id must be positive")
	}
	return s.products.Get(ctx, id)
}

func (s *Service) CreateProduct(ctx context.Context, p catalogdom.Product) (catalogdom.Product, error) {
	if err := validateProduct(p, true); err != nil {
		return catalogdom.Product{}, err
	}
	return s.products.Create(ctx, normalizeProduct(p))
}

func (s *Service) UpdateProduct(ctx context.Context, id catalogdom.ProductID, p catalogdom.Product) (catalogdom.Product, error) {
	p.ID = id
	if id <= 0 {
		return catalogdom.Product{}, apperr.Validation("product id must be positive")
	}
	if err := validateProduct(p, true); err != nil {
		return catalogdom.Product{}, err
	}
	return s.products.Update(ctx, normalizeProduct(p))
}

func (s *Service) DeleteProduct(ctx context.Context, id catalogdom.ProductID) error {
	if id <= 0 {
		return apperr.Validation("product id must be positive")
	}
	return s.products.Delete(ctx, id)
}

func (s *Service) ListRoleTags(ctx context.Context) ([]catalogdom.RoleTag, error) {
	return s.products.ListRoleTags(ctx)
}

func validateProduct(p catalogdom.Product, requireRoles bool) error {
	var details []string
	if strings.TrimSpace(p.Brand) == "" {
		details = append(details, "brand is required")
	}
	if strings.TrimSpace(p.Name) == "" {
		details = append(details, "name is required")
	}
	if p.ProteinPer100g < 0 {
		details = append(details, "protein_per_100g must be non-negative")
	}
	if p.FatPer100g < 0 {
		details = append(details, "fat_per_100g must be non-negative")
	}
	if p.CarbPer100g < 0 {
		details = append(details, "carb_per_100g must be non-negative")
	}
	if p.KcalPer100g < 0 {
		details = append(details, "kcal_per_100g must be non-negative")
	}
	if p.PricePer100g != nil && *p.PricePer100g < 0 {
		details = append(details, "price_per_100g must be non-negative")
	}
	if requireRoles && len(p.Roles) == 0 {
		details = append(details, "roles is required")
	}
	for _, role := range p.Roles {
		if err := validateRoleCode(role); err != nil {
			details = append(details, err.Error())
		}
	}
	validateOptionalNonNegative("saturated_fat_per_100g", nutritionValue(p.Nutrition, func(n *catalogdom.NutritionFacts) *float64 { return n.SaturatedFatPer100g }), &details)
	validateOptionalNonNegative("trans_fat_per_100g", nutritionValue(p.Nutrition, func(n *catalogdom.NutritionFacts) *float64 { return n.TransFatPer100g }), &details)
	validateOptionalNonNegative("industrial_trans_fat_per_100g", nutritionValue(p.Nutrition, func(n *catalogdom.NutritionFacts) *float64 { return n.IndustrialTransFatPer100g }), &details)
	validateOptionalNonNegative("fiber_per_100g", nutritionValue(p.Nutrition, func(n *catalogdom.NutritionFacts) *float64 { return n.FiberPer100g }), &details)
	validateOptionalNonNegative("sodium_mg_per_100g", nutritionValue(p.Nutrition, func(n *catalogdom.NutritionFacts) *float64 { return n.SodiumMgPer100g }), &details)
	validateOptionalNonNegative("salt_g_per_100g", nutritionValue(p.Nutrition, func(n *catalogdom.NutritionFacts) *float64 { return n.SaltGPer100g }), &details)
	validateOptionalNonNegative("total_sugars_per_100g", nutritionValue(p.Nutrition, func(n *catalogdom.NutritionFacts) *float64 { return n.TotalSugarsPer100g }), &details)
	validateOptionalNonNegative("added_sugars_per_100g", nutritionValue(p.Nutrition, func(n *catalogdom.NutritionFacts) *float64 { return n.AddedSugarsPer100g }), &details)
	validateOptionalNonNegative("free_sugars_per_100g", nutritionValue(p.Nutrition, func(n *catalogdom.NutritionFacts) *float64 { return n.FreeSugarsPer100g }), &details)
	if p.DataQuality != nil {
		validateOptionalRange("nutrition_coverage_score", p.DataQuality.NutritionCoverageScore, &details)
		if p.DataQuality.NutritionDataQuality != nil && !validNutritionDataQuality(*p.DataQuality.NutritionDataQuality) {
			details = append(details, "nutrition_data_quality is invalid")
		}
		if p.DataQuality.SourceKind != nil && !validSourceKind(*p.DataQuality.SourceKind) {
			details = append(details, "source_kind is invalid")
		}
	}
	if p.Classification != nil {
		validateOptionalRange("processing_confidence", p.Classification.ProcessingConfidence, &details)
		validateOptionalRange("whole_grain_ratio", p.Classification.WholeGrainRatio, &details)
		if p.Classification.FoodGroup != nil && !validFoodGroup(*p.Classification.FoodGroup) {
			details = append(details, "food_group is invalid")
		}
		if p.Classification.ProcessingLevel != nil && !validProcessingLevel(*p.Classification.ProcessingLevel) {
			details = append(details, "processing_level is invalid")
		}
	}
	if len(details) > 0 {
		return apperr.Validation("invalid product", details...)
	}
	return nil
}

func normalizeProduct(p catalogdom.Product) catalogdom.Product {
	p.Brand = strings.TrimSpace(p.Brand)
	p.Name = strings.TrimSpace(p.Name)
	if p.DefaultUnit == "" {
		p.DefaultUnit = "g"
	}
	p.Roles = uniqueTrimmed(p.Roles)
	p.Allergens = uniqueTrimmed(p.Allergens)
	return p
}

func uniqueTrimmed(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func validateRoleCode(code string) error {
	parts := strings.Split(code, ":")
	if len(parts) < 2 || strings.TrimSpace(code) == "" {
		return fmt.Errorf("role %q is invalid", code)
	}
	if !validRoleKind(catalogdom.RoleKind(parts[0])) {
		return fmt.Errorf("role %q has invalid kind", code)
	}
	return nil
}

func nutritionValue(n *catalogdom.NutritionFacts, pick func(*catalogdom.NutritionFacts) *float64) *float64 {
	if n == nil {
		return nil
	}
	return pick(n)
}

func validateOptionalNonNegative(name string, value *float64, details *[]string) {
	if value != nil && *value < 0 {
		*details = append(*details, name+" must be non-negative")
	}
}

func validateOptionalRange(name string, value *float64, details *[]string) {
	if value != nil && (*value < 0 || *value > 1) {
		*details = append(*details, name+" must be between 0 and 1")
	}
}

func validRoleKind(kind catalogdom.RoleKind) bool {
	switch kind {
	case catalogdom.RoleKindProtein, catalogdom.RoleKindCarb, catalogdom.RoleKindVeg, catalogdom.RoleKindSauce, catalogdom.RoleKindFat, catalogdom.RoleKindOther:
		return true
	default:
		return false
	}
}

func validFoodGroup(group catalogdom.FoodGroup) bool {
	switch group {
	case catalogdom.FoodGroupFruit, catalogdom.FoodGroupVegetable, catalogdom.FoodGroupPotato, catalogdom.FoodGroupGrain,
		catalogdom.FoodGroupLegume, catalogdom.FoodGroupNutsSeeds, catalogdom.FoodGroupDairy, catalogdom.FoodGroupDairyAlt,
		catalogdom.FoodGroupFishSeafood, catalogdom.FoodGroupEgg, catalogdom.FoodGroupRedMeat, catalogdom.FoodGroupWhiteMeat,
		catalogdom.FoodGroupProcessedMeat, catalogdom.FoodGroupMixed, catalogdom.FoodGroupSauceCondiment, catalogdom.FoodGroupConfectionery,
		catalogdom.FoodGroupBeverage, catalogdom.FoodGroupOther:
		return true
	default:
		return false
	}
}

func validProcessingLevel(level catalogdom.ProcessingLevel) bool {
	switch level {
	case catalogdom.ProcessingLevelMinimallyProcessed, catalogdom.ProcessingLevelProcessedCulinary,
		catalogdom.ProcessingLevelProcessedFood, catalogdom.ProcessingLevelUltraProcessed, catalogdom.ProcessingLevelUnknown:
		return true
	default:
		return false
	}
}

func validNutritionDataQuality(value catalogdom.NutritionDataQuality) bool {
	switch value {
	case catalogdom.NutritionDataQualityLabelExact, catalogdom.NutritionDataQualityTrustedDB,
		catalogdom.NutritionDataQualityEstimated, catalogdom.NutritionDataQualityUnknown:
		return true
	default:
		return false
	}
}

func validSourceKind(value catalogdom.SourceKind) bool {
	switch value {
	case catalogdom.SourceKindPackLabel, catalogdom.SourceKindStoreCard, catalogdom.SourceKindCompositionTable, catalogdom.SourceKindManualEstimate:
		return true
	default:
		return false
	}
}
