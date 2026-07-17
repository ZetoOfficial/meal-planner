//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"testing"

	catalogdom "github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
	apperr "github.com/ZetoOfficial/meal-planner/internal/errors"
	postgresdb "github.com/ZetoOfficial/meal-planner/internal/postgres"
	"github.com/ZetoOfficial/meal-planner/internal/postgres/repositories"
	"github.com/ZetoOfficial/meal-planner/internal/postgres/sqlc"
	testpostgres "github.com/ZetoOfficial/meal-planner/internal/testkit/postgres"
	catalogusecase "github.com/ZetoOfficial/meal-planner/internal/usecase/catalog"
)

func TestCatalogRoundTripReplacesAggregateAndPreservesNullNutrition(t *testing.T) {
	database := testpostgres.RequireDatabase(t)
	repository := repositories.NewCatalog(repositories.New(database.Pool))
	ctx := context.Background()

	proteinRole, err := repository.GetRoleTag(ctx, "protein:lean:breast")
	if err != nil {
		t.Fatalf("load protein role: %v", err)
	}
	carbRole, err := repository.GetRoleTag(ctx, "carb:grain")
	if err != nil {
		t.Fatalf("load carb role: %v", err)
	}
	fiber := 2.1
	created, err := repository.Create(ctx, catalogdom.Product{
		Brand:          "Brand",
		Name:           "Original",
		ProteinPer100G: 23,
		FatPer100G:     2,
		CarbPer100G:    1,
		KcalPer100G:    120,
		DefaultUnit:    "g",
		Purchasable:    true,
		Nutrition: catalogdom.NutritionFacts{
			FiberPer100G: &fiber,
		},
		DataQuality: catalogdom.NutritionData{
			Quality:  catalogdom.NutritionDataQualityLabelExact,
			Coverage: 0.1,
		},
		Classification: catalogdom.FoodClassification{
			FoodGroup:       catalogdom.FoodGroupWhiteMeat,
			ProcessingLevel: catalogdom.ProcessingLevelMinimallyProcessed,
		},
		Roles:     []catalogdom.RoleTag{proteinRole},
		Allergens: []catalogdom.Allergen{"casein", "nuts"},
	})
	if err != nil {
		t.Fatalf("create product: %v", err)
	}

	created.Name = "Replacement"
	created.Roles = []catalogdom.RoleTag{carbRole}
	created.Allergens = []catalogdom.Allergen{"gluten"}
	created.Nutrition.FiberPer100G = nil
	created.Classification = catalogdom.FoodClassification{
		FoodGroup:       catalogdom.FoodGroupGrain,
		ProcessingLevel: catalogdom.ProcessingLevelProcessedFood,
	}
	updated, err := repository.Update(ctx, created)
	if err != nil {
		t.Fatalf("update product: %v", err)
	}

	if len(updated.Roles) != 1 || updated.Roles[0].Code != "carb:grain" {
		t.Fatalf("updated roles = %#v, want only carb:grain", updated.Roles)
	}
	if len(updated.Allergens) != 1 || updated.Allergens[0] != "gluten" {
		t.Fatalf("updated allergens = %#v, want only gluten", updated.Allergens)
	}
	if updated.Nutrition.FiberPer100G != nil {
		t.Fatalf("updated fiber = %v, want nil", *updated.Nutrition.FiberPer100G)
	}
	if updated.Classification.FoodGroup != catalogdom.FoodGroupGrain {
		t.Fatalf("updated classification = %q, want grain", updated.Classification.FoodGroup)
	}

	row, err := sqlc.New(database.Pool).GetProduct(ctx, int64(updated.ID))
	if err != nil {
		t.Fatalf("read stored product: %v", err)
	}
	if row.FiberPer100g.Valid {
		t.Fatal("stored fiber is valid, want SQL NULL")
	}
}

func TestCatalogCreateRollsBackWhenRelatedWriteFails(t *testing.T) {
	database := testpostgres.RequireDatabase(t)
	repository := repositories.NewCatalog(repositories.New(database.Pool))
	service := catalogusecase.NewService(invalidRoleIDRepository{Catalog: repository}, postgresdb.NewTxManager(database.Pool))

	_, err := service.CreateProduct(context.Background(), catalogusecase.CreateProductCommand{
		Brand:          "Brand",
		Name:           "Not persisted",
		ProteinPer100G: 20,
		FatPer100G:     2,
		CarbPer100G:    1,
		KcalPer100G:    100,
		Roles:          []string{"protein:lean:breast"},
		Allergens:      []catalogdom.Allergen{"nuts", "casein"},
	})
	var appError *apperr.Error
	if !errors.As(err, &appError) || appError.Code != apperr.CodeNotFound {
		t.Fatalf("CreateProduct() error = %v, want not found from related role write", err)
	}

	products, err := sqlc.New(database.Pool).ListProducts(context.Background())
	if err != nil {
		t.Fatalf("list products after failed create: %v", err)
	}
	if len(products) != 0 {
		t.Fatalf("products after failed create = %d, want 0", len(products))
	}
}

type invalidRoleIDRepository struct {
	*repositories.Catalog
}

func (r invalidRoleIDRepository) GetRoleTag(ctx context.Context, code string) (catalogdom.RoleTag, error) {
	role, err := r.Catalog.GetRoleTag(ctx, code)
	if err != nil {
		return catalogdom.RoleTag{}, err
	}
	role.ID = -1
	return role, nil
}
