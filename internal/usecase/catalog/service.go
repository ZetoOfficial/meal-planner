package catalog

import (
	"context"
	"fmt"
	"math"
	"strings"

	catalogdom "github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
	apperr "github.com/ZetoOfficial/meal-planner/internal/errors"
)

const (
	maxGramsPer100G  = 9_999.99
	maxKcalPer100G   = 99_999.99
	maxPricePer100G  = 999_999.99
	maxSodiumPer100G = 999_999.99
)

type Service struct {
	products     ProductRepository
	transactions TxRunner
}

func NewService(products ProductRepository, transactions TxRunner) *Service {
	if transactions == nil {
		panic("catalog transaction runner is required")
	}
	return &Service{products: products, transactions: transactions}
}

type CreateProductCommand struct {
	Brand          string
	Name           string
	ProteinPer100G float64
	FatPer100G     float64
	CarbPer100G    float64
	KcalPer100G    float64
	PricePer100G   *float64
	DefaultUnit    string
	Purchasable    bool
	Notes          *string
	Nutrition      catalogdom.NutritionFacts
	DataQuality    catalogdom.NutritionData
	Classification catalogdom.FoodClassification
	Allergens      []catalogdom.Allergen
	Roles          []string
}

type UpdateProductCommand = CreateProductCommand

func (s *Service) CreateProduct(ctx context.Context, command CreateProductCommand) (catalogdom.Product, error) {
	var created catalogdom.Product
	err := s.transactions.WithTx(ctx, func(ctx context.Context) error {
		product, err := s.productFromCommand(ctx, command)
		if err != nil {
			return err
		}
		created, err = s.products.Create(ctx, product)
		return err
	})
	if err != nil {
		return catalogdom.Product{}, err
	}
	return created, nil
}

func (s *Service) ListProducts(ctx context.Context, roleCode string) ([]catalogdom.Product, error) {
	roleCode = strings.TrimSpace(roleCode)
	if roleCode != "" {
		if _, err := s.products.GetRoleTag(ctx, roleCode); err != nil {
			return nil, err
		}
	}
	return s.products.List(ctx, roleCode)
}

func (s *Service) GetProduct(ctx context.Context, id catalogdom.ProductID) (catalogdom.Product, error) {
	if !id.Valid() {
		return catalogdom.Product{}, apperr.Validation("product id must be positive")
	}
	return s.products.Get(ctx, id)
}

func (s *Service) UpdateProduct(ctx context.Context, id catalogdom.ProductID, command UpdateProductCommand) (catalogdom.Product, error) {
	if !id.Valid() {
		return catalogdom.Product{}, apperr.Validation("product id must be positive")
	}
	var updated catalogdom.Product
	err := s.transactions.WithTx(ctx, func(ctx context.Context) error {
		product, err := s.productFromCommand(ctx, command)
		if err != nil {
			return err
		}
		product.ID = id
		updated, err = s.products.Update(ctx, product)
		return err
	})
	if err != nil {
		return catalogdom.Product{}, err
	}
	return updated, nil
}

func (s *Service) DeleteProduct(ctx context.Context, id catalogdom.ProductID) error {
	if !id.Valid() {
		return apperr.Validation("product id must be positive")
	}
	return s.products.Delete(ctx, id)
}

func (s *Service) ListRoleTags(ctx context.Context) ([]catalogdom.RoleTag, error) {
	return s.products.ListRoleTags(ctx)
}

func (s *Service) productFromCommand(ctx context.Context, command CreateProductCommand) (catalogdom.Product, error) {
	product, err := command.product()
	if err != nil {
		return catalogdom.Product{}, err
	}
	if len(command.Roles) == 0 {
		return catalogdom.Product{}, apperr.Validation("product must have at least one role")
	}
	roles := make([]catalogdom.RoleTag, 0, len(command.Roles))
	seen := make(map[string]struct{}, len(command.Roles))
	for _, code := range command.Roles {
		code = strings.TrimSpace(code)
		if code == "" {
			return catalogdom.Product{}, apperr.Validation("role code must not be blank")
		}
		if _, ok := seen[code]; ok {
			continue
		}
		role, err := s.products.GetRoleTag(ctx, code)
		if err != nil {
			return catalogdom.Product{}, err
		}
		if !role.MatchesKind() {
			return catalogdom.Product{}, apperr.Validation("role code does not match role kind", code)
		}
		seen[code] = struct{}{}
		roles = append(roles, role)
	}
	product.Roles = roles
	return product, nil
}

func (command CreateProductCommand) product() (catalogdom.Product, error) {
	product := catalogdom.Product{
		Brand:          strings.TrimSpace(command.Brand),
		Name:           strings.TrimSpace(command.Name),
		ProteinPer100G: command.ProteinPer100G,
		FatPer100G:     command.FatPer100G,
		CarbPer100G:    command.CarbPer100G,
		KcalPer100G:    command.KcalPer100G,
		PricePer100G:   command.PricePer100G,
		DefaultUnit:    strings.TrimSpace(command.DefaultUnit),
		Purchasable:    command.Purchasable,
		Notes:          command.Notes,
		Nutrition:      command.Nutrition,
		DataQuality:    command.DataQuality,
		Classification: command.Classification,
	}
	if product.DefaultUnit == "" {
		product.DefaultUnit = "g"
	}
	if product.DataQuality.Quality == "" {
		product.DataQuality.Quality = catalogdom.NutritionDataQualityUnknown
	}
	if product.Classification.ProcessingLevel == "" {
		product.Classification.ProcessingLevel = catalogdom.ProcessingLevelUnknown
	}
	allergens, err := normalizeAllergens(command.Allergens)
	if err != nil {
		return catalogdom.Product{}, err
	}
	product.Allergens = allergens
	if err := validateProduct(product); err != nil {
		return catalogdom.Product{}, err
	}
	product.DataQuality.Coverage = product.Nutrition.Coverage()
	return product, nil
}

func validateProduct(product catalogdom.Product) error {
	if product.Brand == "" || product.Name == "" {
		return apperr.Validation("product brand and name must not be blank")
	}
	for name, value := range map[string]numericInput{
		"protein_per_100g": {value: product.ProteinPer100G, upperBound: maxGramsPer100G},
		"fat_per_100g":     {value: product.FatPer100G, upperBound: maxGramsPer100G},
		"carb_per_100g":    {value: product.CarbPer100G, upperBound: maxGramsPer100G},
		"kcal_per_100g":    {value: product.KcalPer100G, upperBound: maxKcalPer100G},
	} {
		if err := validateNumeric(name, value.value, value.upperBound); err != nil {
			return err
		}
	}
	if err := validateOptionalNumeric("price_per_100g", product.PricePer100G, maxPricePer100G); err != nil {
		return err
	}
	if !product.DataQuality.Quality.Valid() {
		return apperr.Validation("nutrition_data_quality is invalid")
	}
	if product.DataQuality.SourceKind != nil && !product.DataQuality.SourceKind.Valid() {
		return apperr.Validation("source_kind is invalid")
	}
	if product.Classification.FoodGroup != nil && !product.Classification.FoodGroup.Valid() {
		return apperr.Validation("food_group is invalid")
	}
	if !product.Classification.ProcessingLevel.Valid() {
		return apperr.Validation("processing_level is invalid")
	}
	for name, value := range map[string]optionalNumericInput{
		"saturated_fat_per_100g":        {value: product.Nutrition.SaturatedFatPer100G, upperBound: maxGramsPer100G},
		"trans_fat_per_100g":            {value: product.Nutrition.TransFatPer100G, upperBound: maxGramsPer100G},
		"industrial_trans_fat_per_100g": {value: product.Nutrition.IndustrialTransFatPer100G, upperBound: maxGramsPer100G},
		"fiber_per_100g":                {value: product.Nutrition.FiberPer100G, upperBound: maxGramsPer100G},
		"sodium_mg_per_100g":            {value: product.Nutrition.SodiumMgPer100G, upperBound: maxSodiumPer100G},
		"salt_g_per_100g":               {value: product.Nutrition.SaltGPer100G, upperBound: maxGramsPer100G},
		"total_sugars_per_100g":         {value: product.Nutrition.TotalSugarsPer100G, upperBound: maxGramsPer100G},
		"added_sugars_per_100g":         {value: product.Nutrition.AddedSugarsPer100G, upperBound: maxGramsPer100G},
		"free_sugars_per_100g":          {value: product.Nutrition.FreeSugarsPer100G, upperBound: maxGramsPer100G},
	} {
		if err := validateOptionalNumeric(name, value.value, value.upperBound); err != nil {
			return err
		}
	}
	if err := validateRatio("processing_confidence", product.Classification.ProcessingConfidence); err != nil {
		return err
	}
	return validateRatio("whole_grain_ratio", product.Classification.WholeGrainRatio)
}

type numericInput struct {
	value      float64
	upperBound float64
}

type optionalNumericInput struct {
	value      *float64
	upperBound float64
}

func validateOptionalNumeric(name string, value *float64, upperBound float64) error {
	if value != nil {
		return validateNumeric(name, *value, upperBound)
	}
	return nil
}

func validateNumeric(name string, value, upperBound float64) error {
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > upperBound {
		return apperr.Validation(fmt.Sprintf("%s must be between 0 and %g", name, upperBound))
	}
	return nil
}

func validateRatio(name string, value *float64) error {
	if value != nil && (math.IsNaN(*value) || math.IsInf(*value, 0) || *value < 0 || *value > 1) {
		return apperr.Validation(name + " must be between 0 and 1")
	}
	return nil
}

func normalizeAllergens(allergens []catalogdom.Allergen) ([]catalogdom.Allergen, error) {
	normalized := make([]catalogdom.Allergen, 0, len(allergens))
	seen := make(map[catalogdom.Allergen]struct{}, len(allergens))
	for _, allergen := range allergens {
		allergen = catalogdom.Allergen(strings.TrimSpace(string(allergen)))
		if allergen == "" {
			return nil, apperr.Validation("allergen must not be blank")
		}
		if _, ok := seen[allergen]; ok {
			continue
		}
		seen[allergen] = struct{}{}
		normalized = append(normalized, allergen)
	}
	return normalized, nil
}
