package catalog

import (
	"context"
	"errors"
	"math"
	"slices"
	"testing"

	catalogdom "github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
	apperr "github.com/ZetoOfficial/meal-planner/internal/errors"
)

func TestCreateProductRejectsNegativeNutritionValue(t *testing.T) {
	repository := &catalogRepositoryStub{
		roleTags: map[string]catalogdom.RoleTag{
			"protein:lean": {ID: 1, Code: "protein:lean", Kind: catalogdom.RoleKindProtein},
		},
	}
	service := NewService(repository, passthroughTxRunner{})

	_, err := service.CreateProduct(t.Context(), CreateProductCommand{
		Brand: "  Brand  ",
		Name:  " Product ",
		Roles: []string{"protein:lean"},
		Nutrition: catalogdom.NutritionFacts{
			FiberPer100G: float64ptr(-1),
		},
	})

	if !errors.Is(err, apperr.ErrValidation) {
		t.Fatalf("CreateProduct() error = %v, want validation error", err)
	}
	if repository.created {
		t.Fatal("CreateProduct() persisted an invalid product")
	}
}

func TestCreateProductRejectsNegativeRequiredMacro(t *testing.T) {
	repository := &catalogRepositoryStub{
		roleTags: map[string]catalogdom.RoleTag{
			"protein:lean": {ID: 1, Code: "protein:lean", Kind: catalogdom.RoleKindProtein},
		},
	}
	service := NewService(repository, passthroughTxRunner{})

	_, err := service.CreateProduct(t.Context(), CreateProductCommand{
		Brand:          "Brand",
		Name:           "Product",
		ProteinPer100G: -1,
		Roles:          []string{"protein:lean"},
	})

	if !errors.Is(err, apperr.ErrValidation) {
		t.Fatalf("CreateProduct() error = %v, want validation error", err)
	}
	if repository.created {
		t.Fatal("CreateProduct() persisted an invalid product")
	}
}

func TestCreateProductRejectsInvalidEnum(t *testing.T) {
	repository := &catalogRepositoryStub{
		roleTags: map[string]catalogdom.RoleTag{
			"protein:lean": {ID: 1, Code: "protein:lean", Kind: catalogdom.RoleKindProtein},
		},
	}
	service := NewService(repository, passthroughTxRunner{})

	_, err := service.CreateProduct(t.Context(), CreateProductCommand{
		Brand: "Brand",
		Name:  "Product",
		Roles: []string{"protein:lean"},
		DataQuality: catalogdom.NutritionData{
			Quality: "not_a_quality",
		},
	})

	if !errors.Is(err, apperr.ErrValidation) {
		t.Fatalf("CreateProduct() error = %v, want validation error", err)
	}
	if repository.created {
		t.Fatal("CreateProduct() persisted an invalid enum")
	}
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
			repository := &catalogRepositoryStub{roleTags: map[string]catalogdom.RoleTag{
				"protein:lean": {ID: 1, Code: "protein:lean", Kind: catalogdom.RoleKindProtein},
			}}
			service := NewService(repository, passthroughTxRunner{})
			command := test.command
			command.Brand = "Brand"
			command.Name = "Product"
			command.Roles = []string{"protein:lean"}

			_, err := service.CreateProduct(t.Context(), command)
			if !errors.Is(err, apperr.ErrValidation) {
				t.Fatalf("CreateProduct() error = %v, want validation error", err)
			}
			if repository.created {
				t.Fatal("CreateProduct() persisted an unrepresentable numeric value")
			}
		})
	}
}

func TestCreateProductAcceptsNumericUpperBounds(t *testing.T) {
	repository := &catalogRepositoryStub{roleTags: map[string]catalogdom.RoleTag{
		"protein:lean": {ID: 1, Code: "protein:lean", Kind: catalogdom.RoleKindProtein},
	}}
	service := NewService(repository, passthroughTxRunner{})
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
	if !repository.created {
		t.Fatal("CreateProduct() did not persist numeric values at supported upper bounds")
	}
}

func TestCreateProductRejectsRoleWithMismatchedKindPrefix(t *testing.T) {
	repository := &catalogRepositoryStub{
		roleTags: map[string]catalogdom.RoleTag{
			"carb:grain": {ID: 1, Code: "carb:grain", Kind: catalogdom.RoleKindProtein},
		},
	}
	service := NewService(repository, passthroughTxRunner{})

	_, err := service.CreateProduct(t.Context(), CreateProductCommand{
		Brand: "Brand",
		Name:  "Product",
		Roles: []string{"carb:grain"},
	})

	if !errors.Is(err, apperr.ErrValidation) {
		t.Fatalf("CreateProduct() error = %v, want validation error", err)
	}
	if repository.created {
		t.Fatal("CreateProduct() persisted a product with an invalid role tag")
	}
}

func TestCreateProductDerivesNutritionCoverage(t *testing.T) {
	repository := &catalogRepositoryStub{
		roleTags: map[string]catalogdom.RoleTag{
			"protein:lean": {ID: 1, Code: "protein:lean", Kind: catalogdom.RoleKindProtein},
		},
	}
	service := NewService(repository, passthroughTxRunner{})

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

func TestCreateProductRollsBackWhenRelatedWriteFails(t *testing.T) {
	transactions := &rollbackTxRunner{}
	service := NewService(&relatedWriteFailureRepository{}, transactions)

	_, err := service.CreateProduct(t.Context(), CreateProductCommand{
		Brand: "Brand",
		Name:  "Product",
		Roles: []string{"protein:lean"},
	})
	if !errors.Is(err, apperr.ErrConflict) {
		t.Fatalf("CreateProduct() error = %v, want conflict", err)
	}
	if len(transactions.committed) != 0 {
		t.Fatalf("committed products = %#v, want no partial product after rollback", transactions.committed)
	}
}

type catalogRepositoryStub struct {
	roleTags map[string]catalogdom.RoleTag
	created  bool
}

type passthroughTxRunner struct{}

func (passthroughTxRunner) WithTx(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

type rollbackTxRunner struct {
	committed []catalogdom.Product
}

func (r *rollbackTxRunner) WithTx(ctx context.Context, fn func(context.Context) error) error {
	transaction := &catalogTransaction{}
	if err := fn(context.WithValue(ctx, catalogTransactionKey{}, transaction)); err != nil {
		return err
	}
	r.committed = append(r.committed, transaction.pending...)
	return nil
}

type catalogTransactionKey struct{}

type catalogTransaction struct {
	pending []catalogdom.Product
}

type relatedWriteFailureRepository struct{}

func (*relatedWriteFailureRepository) Create(ctx context.Context, product catalogdom.Product) (catalogdom.Product, error) {
	transaction, ok := ctx.Value(catalogTransactionKey{}).(*catalogTransaction)
	if !ok {
		return catalogdom.Product{}, errors.New("missing transaction context")
	}
	transaction.pending = append(transaction.pending, product) // Simulates the product row written before a related row fails.
	return catalogdom.Product{}, apperr.Conflict("role assignment failed")
}

func (*relatedWriteFailureRepository) Delete(context.Context, catalogdom.ProductID) error { return nil }

func (*relatedWriteFailureRepository) Get(context.Context, catalogdom.ProductID) (catalogdom.Product, error) {
	return catalogdom.Product{}, nil
}

func (*relatedWriteFailureRepository) GetRoleTag(_ context.Context, code string) (catalogdom.RoleTag, error) {
	return catalogdom.RoleTag{ID: 1, Code: code, Kind: catalogdom.RoleKindProtein}, nil
}

func (*relatedWriteFailureRepository) List(context.Context, string) ([]catalogdom.Product, error) {
	return nil, nil
}

func (*relatedWriteFailureRepository) ListRoleTags(context.Context) ([]catalogdom.RoleTag, error) {
	return nil, nil
}

func (*relatedWriteFailureRepository) Update(context.Context, catalogdom.Product) (catalogdom.Product, error) {
	return catalogdom.Product{}, nil
}

func (r *catalogRepositoryStub) Create(_ context.Context, product catalogdom.Product) (catalogdom.Product, error) {
	r.created = true
	product.ID = 1
	return product, nil
}

func (r *catalogRepositoryStub) Delete(context.Context, catalogdom.ProductID) error { return nil }

func (r *catalogRepositoryStub) Get(context.Context, catalogdom.ProductID) (catalogdom.Product, error) {
	return catalogdom.Product{}, nil
}

func (r *catalogRepositoryStub) GetRoleTag(_ context.Context, code string) (catalogdom.RoleTag, error) {
	tag, ok := r.roleTags[code]
	if !ok {
		return catalogdom.RoleTag{}, apperr.NotFound("role tag")
	}
	return tag, nil
}

func (r *catalogRepositoryStub) List(context.Context, string) ([]catalogdom.Product, error) {
	return nil, nil
}

func (r *catalogRepositoryStub) ListRoleTags(context.Context) ([]catalogdom.RoleTag, error) {
	return nil, nil
}

func (r *catalogRepositoryStub) Update(_ context.Context, product catalogdom.Product) (catalogdom.Product, error) {
	return product, nil
}

func float64ptr(value float64) *float64 { return &value }
func boolptr(value bool) *bool          { return &value }
