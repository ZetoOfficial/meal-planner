//go:build integration

package postgres_test

import (
	"context"
	"testing"

	catalogdom "github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
	"github.com/ZetoOfficial/meal-planner/internal/postgres/postgrestest"
	"github.com/ZetoOfficial/meal-planner/internal/postgres/repositories"
	cataloguc "github.com/ZetoOfficial/meal-planner/internal/usecase/catalog"
)

func TestProductRepositoryIntegrationCRUDRoundTrip(t *testing.T) {
	db := postgrestest.RequireDatabase(t)
	repo := repositories.New(db.Pool).Products
	ctx := context.Background()

	price := 55.5
	fiber := 2.5
	coverage := 0.5
	quality := catalogdom.NutritionDataQualityLabelExact
	sourceKind := catalogdom.SourceKindPackLabel
	sourceNote := "pack label"
	foodGroup := catalogdom.FoodGroupWhiteMeat
	processingLevel := catalogdom.ProcessingLevelMinimallyProcessed
	processingConfidence := 0.9
	isRedMeat := false

	created, err := repo.Create(ctx, catalogdom.Product{
		Brand:          "Test Brand",
		Name:           "Chicken Breast",
		ProteinPer100g: 23.5,
		FatPer100g:     1.2,
		CarbPer100g:    0,
		KcalPer100g:    110,
		PricePer100g:   &price,
		DefaultUnit:    "g",
		Purchasable:    true,
		Roles:          []string{"protein:lean:breast", "protein:lean"},
		Allergens:      []string{"casein", "nuts"},
		Nutrition: &catalogdom.NutritionFacts{
			FiberPer100g: fiberPtr(fiber),
			// SaturatedFatPer100g intentionally absent: unknown must stay NULL, not zero.
		},
		DataQuality: &catalogdom.NutritionDataQualityMetadata{
			NutritionDataQuality:   &quality,
			NutritionCoverageScore: &coverage,
			SourceKind:             &sourceKind,
			SourceNote:             &sourceNote,
		},
		Classification: &catalogdom.FoodClassification{
			FoodGroup:            &foodGroup,
			ProcessingLevel:      &processingLevel,
			ProcessingConfidence: &processingConfidence,
			IsRedMeat:            &isRedMeat,
		},
	})
	if err != nil {
		t.Fatalf("create product: %v", err)
	}
	assertProductRoundTrip(t, created)
	if created.Nutrition == nil || created.Nutrition.SaturatedFatPer100g != nil {
		t.Fatalf("unknown saturated fat should round-trip as nil, got %#v", created.Nutrition)
	}

	listed, err := repo.List(ctx, cataloguc.ProductFilter{Role: stringPtr("protein:lean:breast")})
	if err != nil {
		t.Fatalf("list products by role: %v", err)
	}
	if len(listed) != 1 || listed[0].ID != created.ID {
		t.Fatalf("listed = %#v, want created product", listed)
	}

	got, err := repo.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("get product: %v", err)
	}
	assertProductRoundTrip(t, got)

	created.Name = "Chicken Thigh"
	created.Roles = []string{"protein:lean:thigh"}
	created.Allergens = []string{"casein"}
	created.Nutrition.FiberPer100g = nil
	updated, err := repo.Update(ctx, created)
	if err != nil {
		t.Fatalf("update product: %v", err)
	}
	if updated.Name != "Chicken Thigh" {
		t.Fatalf("updated name = %q", updated.Name)
	}
	if len(updated.Roles) != 1 || updated.Roles[0] != "protein:lean:thigh" {
		t.Fatalf("updated roles = %#v", updated.Roles)
	}
	if updated.Nutrition != nil && updated.Nutrition.FiberPer100g != nil {
		t.Fatalf("fiber should be nil after update, got %#v", updated.Nutrition.FiberPer100g)
	}

	if err := repo.Delete(ctx, created.ID); err != nil {
		t.Fatalf("delete product: %v", err)
	}
	if _, err := repo.Get(ctx, created.ID); err == nil {
		t.Fatal("expected get after delete to fail")
	}
}

func assertProductRoundTrip(t *testing.T, product catalogdom.Product) {
	t.Helper()
	if product.ID == 0 {
		t.Fatal("product id is zero")
	}
	if product.Brand != "Test Brand" {
		t.Fatalf("brand = %q", product.Brand)
	}
	if len(product.Roles) == 0 {
		t.Fatal("roles were not returned")
	}
	if len(product.Allergens) == 0 {
		t.Fatal("allergens were not returned")
	}
	if product.Classification == nil || product.Classification.FoodGroup == nil || *product.Classification.FoodGroup != catalogdom.FoodGroupWhiteMeat {
		t.Fatalf("classification not round-tripped: %#v", product.Classification)
	}
	if product.DataQuality == nil || product.DataQuality.NutritionDataQuality == nil || *product.DataQuality.NutritionDataQuality != catalogdom.NutritionDataQualityLabelExact {
		t.Fatalf("data quality not round-tripped: %#v", product.DataQuality)
	}
}

func fiberPtr(value float64) *float64 { return &value }

func stringPtr(value string) *string { return &value }
