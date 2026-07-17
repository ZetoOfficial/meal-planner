//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/ZetoOfficial/meal-planner/internal/clock"
	catalogdom "github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
	eaterdom "github.com/ZetoOfficial/meal-planner/internal/domain/eater"
	apperr "github.com/ZetoOfficial/meal-planner/internal/errors"
	postgresdb "github.com/ZetoOfficial/meal-planner/internal/postgres"
	"github.com/ZetoOfficial/meal-planner/internal/postgres/repositories"
	testpostgres "github.com/ZetoOfficial/meal-planner/internal/testkit/postgres"
	eaterusecase "github.com/ZetoOfficial/meal-planner/internal/usecase/eater"
)

func TestEaterRoundTripProfileGoalFixedMealAndExclusion(t *testing.T) {
	database := testpostgres.RequireDatabase(t)
	repository := repositories.NewEater(repositories.New(database.Pool))
	service := eaterusecase.NewService(repository, postgresdb.NewTxManager(database.Pool), clock.System{})
	ctx := context.Background()

	created, err := service.CreateEater(ctx, eaterusecase.CreateEaterCommand{
		Name: "Pasha", Sex: eaterdom.SexMale, Allergens: []eaterdom.Allergen{"casein", "nuts"},
		Goal: eaterusecase.MacroGoalCommand{EffectiveFrom: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC), Range: eaterdom.MacroRange{
			ProteinMinG: 130, ProteinMaxG: 140, FatMinG: 50, FatMaxG: 60, CarbMinG: 180, CarbMaxG: 200, KcalMin: 1800, KcalMax: 2000,
		}},
	})
	if err != nil {
		t.Fatalf("create eater: %v", err)
	}

	if _, err := service.UpdateEater(ctx, created.ID, eaterusecase.UpdateEaterCommand{
		Name: "Pasha", Sex: eaterdom.SexMale, Allergens: []eaterdom.Allergen{"peanuts"},
		Goal: eaterusecase.MacroGoalCommand{EffectiveFrom: time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC), Range: eaterdom.MacroRange{
			ProteinMinG: 140, ProteinMaxG: 150, FatMinG: 55, FatMaxG: 65, CarbMinG: 180, CarbMaxG: 210, KcalMin: 1900, KcalMax: 2100,
		}},
	}); err != nil {
		t.Fatalf("update eater: %v", err)
	}

	goals, err := service.ListGoals(ctx, created.ID)
	if err != nil || len(goals) != 2 || goals[0].Range.ProteinMinG != 140 {
		t.Fatalf("goal history = %#v, %v", goals, err)
	}
	profile, err := service.GetEater(ctx, created.ID)
	if err != nil || len(profile.Allergens) != 1 || profile.Allergens[0] != "peanuts" {
		t.Fatalf("profile allergens = %#v, %v", profile.Allergens, err)
	}

	meal, err := service.CreateFixedMeal(ctx, created.ID, eaterusecase.CreateFixedMealCommand{
		MealType: eaterdom.MealTypeBreakfast, Label: "sandwiches", Macros: eaterdom.Macros{ProteinG: 20, FatG: 10, CarbG: 30, Kcal: 300},
	})
	if err != nil || meal.Quality.FiberG != nil || !meal.Quality.IncludeInAnalytics {
		t.Fatalf("fixed meal = %#v, %v", meal, err)
	}

	product, err := repositories.NewCatalog(repositories.New(database.Pool)).Create(ctx, catalogdom.Product{
		Brand: "Brand", Name: "Product", ProteinPer100G: 1, FatPer100G: 1, CarbPer100G: 1, KcalPer100G: 10, DefaultUnit: "g", Purchasable: true,
		DataQuality:    catalogdom.NutritionData{Quality: catalogdom.NutritionDataQualityUnknown},
		Classification: catalogdom.FoodClassification{FoodGroup: catalogdom.FoodGroupOther, ProcessingLevel: catalogdom.ProcessingLevelUnknown},
	})
	if err != nil {
		t.Fatalf("create product target: %v", err)
	}
	exclusion, err := service.CreateExclusion(ctx, created.ID, eaterusecase.CreateExclusionCommand{TargetKind: eaterdom.ExclusionTargetProduct, TargetValue: stringID(int64(product.ID))})
	if err != nil {
		t.Fatalf("create exclusion: %v", err)
	}
	profile, err = service.GetEater(ctx, created.ID)
	if err != nil || len(profile.Exclusions) != 1 || profile.Exclusions[0].ID != exclusion.ID {
		t.Fatalf("profile exclusions = %#v, %v", profile.Exclusions, err)
	}
	if err := service.DeleteExclusion(ctx, exclusion.ID); err != nil {
		t.Fatalf("delete exclusion: %v", err)
	}
}

func TestEaterUpdateRollsBackProfileAndAllergensWhenGoalDateConflicts(t *testing.T) {
	database := testpostgres.RequireDatabase(t)
	service := eaterusecase.NewService(repositories.NewEater(repositories.New(database.Pool)), postgresdb.NewTxManager(database.Pool), clock.System{})
	ctx := context.Background()
	date := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	created, err := service.CreateEater(ctx, eaterusecase.CreateEaterCommand{
		Name: "Pasha", Sex: eaterdom.SexMale, Allergens: []eaterdom.Allergen{"casein"},
		Goal: eaterusecase.MacroGoalCommand{EffectiveFrom: date, Range: eaterdom.MacroRange{ProteinMaxG: 1, FatMaxG: 1, CarbMaxG: 1, KcalMax: 1}},
	})
	if err != nil {
		t.Fatalf("create eater: %v", err)
	}

	_, err = service.UpdateEater(ctx, created.ID, eaterusecase.UpdateEaterCommand{
		Name: "Renamed", Sex: eaterdom.SexFemale, Allergens: []eaterdom.Allergen{"nuts"},
		Goal: eaterusecase.MacroGoalCommand{EffectiveFrom: date, Range: eaterdom.MacroRange{ProteinMaxG: 1, FatMaxG: 1, CarbMaxG: 1, KcalMax: 1}},
	})
	if !errors.Is(err, apperr.ErrConflict) {
		t.Fatalf("update eater: %v, want conflict", err)
	}
	profile, err := service.GetEater(ctx, created.ID)
	if err != nil {
		t.Fatalf("get eater: %v", err)
	}
	if profile.Name != "Pasha" || profile.Sex != eaterdom.SexMale || len(profile.Allergens) != 1 || profile.Allergens[0] != "casein" {
		t.Fatalf("profile after failed update = %#v, want original profile and allergens", profile)
	}
}

func stringID(id int64) string { return fmt.Sprintf("%d", id) }
