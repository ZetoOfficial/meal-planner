//go:build integration

package catalog_test

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/mock/gomock"

	catalogdom "github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
	apperr "github.com/ZetoOfficial/meal-planner/internal/errors"
	postgresdb "github.com/ZetoOfficial/meal-planner/internal/postgres"
	catalogrepo "github.com/ZetoOfficial/meal-planner/internal/postgres/catalog"
	"github.com/ZetoOfficial/meal-planner/internal/postgres/sqlc"
	testpostgres "github.com/ZetoOfficial/meal-planner/internal/testkit/postgres"
	catalogusecase "github.com/ZetoOfficial/meal-planner/internal/usecase/catalog"
	catalogmocks "github.com/ZetoOfficial/meal-planner/internal/usecase/catalog/mocks"
)

func TestCatalogRoundTripReplacesAggregateAndPreservesNullNutrition(t *testing.T) {
	database := testpostgres.RequireDatabase(t)
	repository := catalogrepo.New(postgresdb.NewStore(database.Pool))
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
	whiteMeat := catalogdom.FoodGroupWhiteMeat
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
			FoodGroup:       &whiteMeat,
			ProcessingLevel: catalogdom.ProcessingLevelMinimallyProcessed,
		},
		Roles:     []catalogdom.RoleTag{proteinRole},
		Allergens: []catalogdom.Allergen{"casein", "nuts"},
	})
	if err != nil {
		t.Fatalf("create product: %v", err)
	}
	if created.Classification.IsLegume != nil {
		t.Fatalf("created legume classification = %v, want unknown", *created.Classification.IsLegume)
	}

	created.Name = "Replacement"
	created.Roles = []catalogdom.RoleTag{carbRole}
	created.Allergens = []catalogdom.Allergen{"gluten"}
	created.Nutrition.FiberPer100G = nil
	grain := catalogdom.FoodGroupGrain
	knownFalse := false
	created.Classification = catalogdom.FoodClassification{
		FoodGroup:       &grain,
		ProcessingLevel: catalogdom.ProcessingLevelProcessedFood,
		IsLegume:        &knownFalse,
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
	if updated.Classification.FoodGroup == nil || *updated.Classification.FoodGroup != catalogdom.FoodGroupGrain {
		t.Fatalf("updated classification = %v, want grain", updated.Classification.FoodGroup)
	}
	if updated.Classification.IsLegume == nil || *updated.Classification.IsLegume {
		t.Fatalf("updated legume classification = %v, want known false", updated.Classification.IsLegume)
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
	repository := catalogrepo.New(postgresdb.NewStore(database.Pool))
	ctrl := gomock.NewController(t)
	productRepository := catalogmocks.NewMockProductRepository(ctrl)
	productRepository.EXPECT().
		GetRoleTag(gomock.Any(), "protein:lean:breast").
		DoAndReturn(func(ctx context.Context, code string) (catalogdom.RoleTag, error) {
			role, err := repository.GetRoleTag(ctx, code)
			role.ID = -1
			return role, err
		})
	productRepository.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(repository.Create)
	service := catalogusecase.NewService(productRepository, postgresdb.NewTxManager(database.Pool))

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
