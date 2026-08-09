package catalog

import (
	"context"
	"errors"
	"math"
	"slices"
	"testing"

	"go.uber.org/mock/gomock"

	catalogdom "github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
	apperr "github.com/ZetoOfficial/meal-planner/internal/errors"
	catalogmocks "github.com/ZetoOfficial/meal-planner/internal/usecase/catalog/mocks"
)

func TestCreateProductRejectsNegativeNutritionValue(t *testing.T) {
	service, _ := newCatalogService(t)

	_, err := service.CreateProduct(t.Context(), CreateProductCommand{
		Brand: "  Brand  ",
		Name:  " Product ",
		Roles: []string{"protein:lean"},
		Nutrition: catalogdom.NutritionFacts{
			FiberPer100G: float64ptr(-1),
		},
	})

	assertErrorCode(t, err, apperr.CodeValidation)
}

func TestCreateProductCommandProductNormalizesDefaults(t *testing.T) {
	command := CreateProductCommand{
		Brand:     " Brand ",
		Name:      " Product ",
		Allergens: []catalogdom.Allergen{" nuts ", "nuts"},
		Nutrition: catalogdom.NutritionFacts{
			FiberPer100G:                      float64ptr(1.5),
			ContainsPartiallyHydrogenatedOils: boolptr(false),
		},
	}

	product, err := command.product()
	if err != nil {
		t.Fatalf("CreateProductCommand.product() error = %v", err)
	}
	if product.Brand != "Brand" || product.Name != "Product" || product.DefaultUnit != "g" {
		t.Fatalf("CreateProductCommand.product() = %#v, want normalized defaults", product)
	}
	if product.DataQuality.Quality != catalogdom.NutritionDataQualityUnknown ||
		product.Classification.FoodGroup != nil ||
		product.Classification.ProcessingLevel != catalogdom.ProcessingLevelUnknown {
		t.Fatalf("CreateProductCommand.product() classification = %#v/%#v", product.DataQuality, product.Classification)
	}
	if !slices.Equal(product.Allergens, []catalogdom.Allergen{"nuts"}) || product.DataQuality.Coverage != 0.2 {
		t.Fatalf("CreateProductCommand.product() allergens/coverage = %#v/%v", product.Allergens, product.DataQuality.Coverage)
	}
}

func TestCreateProductRejectsNegativeRequiredMacro(t *testing.T) {
	service, _ := newCatalogService(t)

	_, err := service.CreateProduct(t.Context(), CreateProductCommand{
		Brand:          "Brand",
		Name:           "Product",
		ProteinPer100G: -1,
		Roles:          []string{"protein:lean"},
	})

	assertErrorCode(t, err, apperr.CodeValidation)
}

func TestCreateProductRejectsInvalidEnum(t *testing.T) {
	service, _ := newCatalogService(t)

	_, err := service.CreateProduct(t.Context(), CreateProductCommand{
		Brand: "Brand",
		Name:  "Product",
		Roles: []string{"protein:lean"},
		DataQuality: catalogdom.NutritionData{
			Quality: "not_a_quality",
		},
	})

	assertErrorCode(t, err, apperr.CodeValidation)
}

func TestCreateProductRejectsNonFiniteAndOutOfRangeNumericValues(t *testing.T) {
	tests := []struct {
		name    string
		command CreateProductCommand
	}{
		{
			name:    "non-finite required macro",
			command: CreateProductCommand{ProteinPer100G: math.NaN()},
		},
		{
			name:    "required macro above numeric precision",
			command: CreateProductCommand{ProteinPer100G: 10_000},
		},
		{
			name:    "non-finite optional price",
			command: CreateProductCommand{PricePer100G: float64ptr(math.Inf(1))},
		},
		{
			name:    "quality nutrient above numeric precision",
			command: CreateProductCommand{Nutrition: catalogdom.NutritionFacts{SodiumMgPer100G: float64ptr(1_000_000)}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service, _ := newCatalogService(t)
			command := test.command
			command.Brand = "Brand"
			command.Name = "Product"
			command.Roles = []string{"protein:lean"}

			_, err := service.CreateProduct(t.Context(), command)

			assertErrorCode(t, err, apperr.CodeValidation)
		})
	}
}

func TestCreateProductAcceptsNumericUpperBounds(t *testing.T) {
	service, repository := newCatalogService(t)
	repository.EXPECT().
		GetRoleTag(gomock.Any(), "protein:lean").
		Return(catalogdom.RoleTag{ID: 1, Code: "protein:lean", Kind: catalogdom.RoleKindProtein}, nil)
	repository.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, product catalogdom.Product) (catalogdom.Product, error) {
			product.ID = 1
			return product, nil
		})
	price := 999_999.99
	sodium := 999_999.99

	_, err := service.CreateProduct(t.Context(), CreateProductCommand{
		Brand:          "Brand",
		Name:           "Product",
		ProteinPer100G: 9_999.99,
		FatPer100G:     9_999.99,
		CarbPer100G:    9_999.99,
		KcalPer100G:    99_999.99,
		PricePer100G:   &price,
		Nutrition: catalogdom.NutritionFacts{
			SodiumMgPer100G: &sodium,
		},
		Classification: catalogdom.FoodClassification{
			ProcessingConfidence: float64ptr(1),
			WholeGrainRatio:      float64ptr(1),
		},
		Roles: []string{"protein:lean"},
	})
	if err != nil {
		t.Fatalf("CreateProduct() error = %v, want nil", err)
	}
}

func TestCreateProductRejectsRoleWithMismatchedKindPrefix(t *testing.T) {
	service, repository := newCatalogService(t)
	repository.EXPECT().
		GetRoleTag(gomock.Any(), "carb:grain").
		Return(catalogdom.RoleTag{ID: 1, Code: "carb:grain", Kind: catalogdom.RoleKindProtein}, nil)

	_, err := service.CreateProduct(t.Context(), CreateProductCommand{
		Brand: "Brand",
		Name:  "Product",
		Roles: []string{"carb:grain"},
	})

	assertErrorCode(t, err, apperr.CodeValidation)
}

func TestCreateProductDerivesNutritionCoverage(t *testing.T) {
	service, repository := newCatalogService(t)
	repository.EXPECT().
		GetRoleTag(gomock.Any(), "protein:lean").
		Return(catalogdom.RoleTag{ID: 1, Code: "protein:lean", Kind: catalogdom.RoleKindProtein}, nil)
	repository.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, product catalogdom.Product) (catalogdom.Product, error) {
			product.ID = 1
			return product, nil
		})

	product, err := service.CreateProduct(t.Context(), CreateProductCommand{
		Brand: " Brand ",
		Name:  " Product ",
		Roles: []string{"protein:lean"},
		Nutrition: catalogdom.NutritionFacts{
			FiberPer100G:                      float64ptr(1.5),
			ContainsPartiallyHydrogenatedOils: boolptr(false),
		},
		Allergens: []catalogdom.Allergen{" nuts ", "nuts", "casein"},
	})
	if err != nil {
		t.Fatalf("CreateProduct() error = %v", err)
	}

	if product.Brand != "Brand" || product.Name != "Product" {
		t.Fatalf("CreateProduct() names = %q, %q, want trimmed values", product.Brand, product.Name)
	}
	if product.DataQuality.Coverage != 0.2 {
		t.Fatalf("CreateProduct() coverage = %v, want 0.2", product.DataQuality.Coverage)
	}
	if got, want := product.Allergens, []catalogdom.Allergen{"nuts", "casein"}; !slices.Equal(got, want) {
		t.Fatalf("CreateProduct() allergens = %#v, want %#v", got, want)
	}
}

func TestCreateProductReturnsConflictWhenRepositoryCreateFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	repository := catalogmocks.NewMockProductRepository(ctrl)
	transactions := catalogmocks.NewMockTxRunner(ctrl)
	transactions.EXPECT().
		WithTx(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		})
	repository.EXPECT().
		GetRoleTag(gomock.Any(), "protein:lean").
		Return(catalogdom.RoleTag{ID: 1, Code: "protein:lean", Kind: catalogdom.RoleKindProtein}, nil)
	repository.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(catalogdom.Product{}, apperr.Conflict("role assignment failed"))
	service := NewService(repository, transactions)

	_, err := service.CreateProduct(t.Context(), CreateProductCommand{
		Brand: "Brand",
		Name:  "Product",
		Roles: []string{"protein:lean"},
	})

	assertErrorCode(t, err, apperr.CodeConflict)
}

func newCatalogService(t *testing.T) (*Service, *catalogmocks.MockProductRepository) {
	t.Helper()
	ctrl := gomock.NewController(t)
	repository := catalogmocks.NewMockProductRepository(ctrl)
	transactions := catalogmocks.NewMockTxRunner(ctrl)
	transactions.EXPECT().
		WithTx(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		})
	return NewService(repository, transactions), repository
}

func assertErrorCode(t *testing.T, err error, want apperr.Code) {
	t.Helper()
	var appError *apperr.Error
	if !errors.As(err, &appError) || appError.Code != want {
		t.Fatalf("error = %v, want code %q", err, want)
	}
}

func float64ptr(value float64) *float64 { return &value }
func boolptr(value bool) *bool          { return &value }
