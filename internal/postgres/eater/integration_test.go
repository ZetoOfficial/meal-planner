//go:build integration

package eater_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/ZetoOfficial/meal-planner/internal/apperr"
	"github.com/ZetoOfficial/meal-planner/internal/clock"
	catalogdom "github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
	eaterdom "github.com/ZetoOfficial/meal-planner/internal/domain/eater"
	postgresdb "github.com/ZetoOfficial/meal-planner/internal/postgres"
	catalogrepo "github.com/ZetoOfficial/meal-planner/internal/postgres/catalog"
	eaterrepo "github.com/ZetoOfficial/meal-planner/internal/postgres/eater"
	testpostgres "github.com/ZetoOfficial/meal-planner/internal/testkit/postgres"
	eaterusecase "github.com/ZetoOfficial/meal-planner/internal/usecase/eater"
)

func TestEaterRoundTripProfileGoalFixedMealAndExclusion(t *testing.T) {
	database := testpostgres.RequireDatabase(t)
	store := postgresdb.NewStore(database.Pool)
	repository := eaterrepo.New(store)
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

	grainG := 60.0
	legumeG := 25.0
	upfKcal := 140.0
	meal, err := service.CreateFixedMeal(ctx, created.ID, eaterusecase.CreateFixedMealCommand{
		MealType: eaterdom.MealTypeBreakfast, Label: "sandwiches", Macros: eaterdom.Macros{ProteinG: 20, FatG: 10, CarbG: 30, Kcal: 300},
		Quality: eaterdom.FixedMealQuality{GrainGEquiv: &grainG, LegumeG: &legumeG, UPFKcal: &upfKcal},
	})
	if err != nil || meal.Quality.FiberG != nil || !meal.Quality.IncludeInAnalytics ||
		meal.Quality.GrainGEquiv == nil || *meal.Quality.GrainGEquiv != grainG ||
		meal.Quality.LegumeG == nil || *meal.Quality.LegumeG != legumeG ||
		meal.Quality.UPFKcal == nil || *meal.Quality.UPFKcal != upfKcal {
		t.Fatalf("fixed meal = %#v, %v", meal, err)
	}

	otherFoodGroup := catalogdom.FoodGroupOther
	product, err := catalogrepo.New(store).Create(ctx, catalogdom.Product{
		Brand: "Brand", Name: "Product", ProteinPer100G: 1, FatPer100G: 1, CarbPer100G: 1, KcalPer100G: 10, DefaultUnit: "g", Purchasable: true,
		DataQuality:    catalogdom.NutritionData{Quality: catalogdom.NutritionDataQualityUnknown},
		Classification: catalogdom.FoodClassification{FoodGroup: &otherFoodGroup, ProcessingLevel: catalogdom.ProcessingLevelUnknown},
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
	service := eaterusecase.NewService(eaterrepo.New(postgresdb.NewStore(database.Pool)), postgresdb.NewTxManager(database.Pool), clock.System{})
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
	var appError *apperr.Error
	if !errors.As(err, &appError) || appError.Code != apperr.CodeConflict {
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

func TestFixedMealUpdateDeactivateAndEaterDelete(t *testing.T) {
	database := testpostgres.RequireDatabase(t)
	repository := eaterrepo.New(postgresdb.NewStore(database.Pool))
	service := eaterusecase.NewService(repository, postgresdb.NewTxManager(database.Pool), clock.System{})
	ctx := context.Background()
	created, err := service.CreateEater(ctx, eaterusecase.CreateEaterCommand{
		Name: "Sasha", Sex: eaterdom.SexFemale,
		Goal: eaterusecase.MacroGoalCommand{
			EffectiveFrom: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
			Range:         eaterdom.MacroRange{ProteinMaxG: 1, FatMaxG: 1, CarbMaxG: 1, KcalMax: 1},
		},
	})
	if err != nil {
		t.Fatalf("create eater: %v", err)
	}
	meal, err := service.CreateFixedMeal(ctx, created.ID, eaterusecase.CreateFixedMealCommand{
		MealType: eaterdom.MealTypeBreakfast, Label: "sandwiches",
		Macros: eaterdom.Macros{ProteinG: 20, FatG: 10, CarbG: 30, Kcal: 300},
	})
	if err != nil {
		t.Fatalf("create fixed meal: %v", err)
	}

	sodium := 900.0
	updated, err := service.UpdateFixedMeal(ctx, meal.ID, eaterusecase.CreateFixedMealCommand{
		MealType: eaterdom.MealTypeBreakfast, Label: "two sandwiches",
		Macros:  eaterdom.Macros{ProteinG: 14, FatG: 7, CarbG: 20, Kcal: 200},
		Quality: eaterdom.FixedMealQuality{SodiumMg: &sodium},
	})
	if err != nil {
		t.Fatalf("update fixed meal: %v", err)
	}
	if updated.ID != meal.ID || updated.EaterID != created.ID || updated.Label != "two sandwiches" ||
		updated.Macros.Kcal != 200 || updated.Quality.SodiumMg == nil || *updated.Quality.SodiumMg != sodium || !updated.Active {
		t.Fatalf("updated fixed meal = %#v, want the same row with the new content", updated)
	}

	if err := service.DeleteFixedMeal(ctx, meal.ID); err != nil {
		t.Fatalf("delete fixed meal: %v", err)
	}
	listed, err := service.ListFixedMeals(ctx, created.ID)
	if err != nil || len(listed) != 0 {
		t.Fatalf("fixed meals after delete = %#v, %v; want none offered to new plans", listed, err)
	}
	kept, err := repository.GetFixedMeal(ctx, meal.ID)
	if err != nil || kept.Active {
		t.Fatalf("stored fixed meal = %#v, %v; want the row kept but inactive", kept, err)
	}
	var appError *apperr.Error
	if _, err := service.UpdateFixedMeal(ctx, meal.ID, eaterusecase.CreateFixedMealCommand{
		MealType: eaterdom.MealTypeBreakfast, Label: "x", Macros: eaterdom.Macros{Kcal: 1},
	}); !errors.As(err, &appError) || appError.Code != apperr.CodeNotFound {
		t.Fatalf("update of a switched-off fixed meal: %v, want not found", err)
	}
	if err := service.DeleteFixedMeal(ctx, meal.ID); !errors.As(err, &appError) || appError.Code != apperr.CodeNotFound {
		t.Fatalf("second delete: %v, want not found", err)
	}
	recreated, err := service.CreateFixedMeal(ctx, created.ID, eaterusecase.CreateFixedMealCommand{
		MealType: eaterdom.MealTypeBreakfast, Label: "two sandwiches",
		Macros: eaterdom.Macros{ProteinG: 14, FatG: 7, CarbG: 20, Kcal: 200},
	})
	if err != nil || recreated.ID == meal.ID || !recreated.Active {
		t.Fatalf("recreated fixed meal = %#v, %v; want a new active row under the label of the switched-off one", recreated, err)
	}

	if err := service.DeleteEater(ctx, created.ID); err != nil {
		t.Fatalf("delete eater without history: %v", err)
	}
	if _, err := service.GetEater(ctx, created.ID); !errors.As(err, &appError) || appError.Code != apperr.CodeNotFound {
		t.Fatalf("get deleted eater: %v, want not found", err)
	}
	if err := service.DeleteEater(ctx, created.ID); !errors.As(err, &appError) || appError.Code != apperr.CodeNotFound {
		t.Fatalf("delete missing eater: %v, want not found", err)
	}
}
