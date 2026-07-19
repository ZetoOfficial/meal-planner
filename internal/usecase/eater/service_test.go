package eater

import (
	"context"
	"errors"
	"math"
	"slices"
	"testing"
	"time"

	"go.uber.org/mock/gomock"

	eaterdom "github.com/ZetoOfficial/meal-planner/internal/domain/eater"
	apperr "github.com/ZetoOfficial/meal-planner/internal/errors"
	"github.com/ZetoOfficial/meal-planner/internal/usecase/eater/mocks"
)

func TestUpdateEaterReplacesAllergensAndAppendsGoal(t *testing.T) {
	service, repository, transactions, clock := newMockService(t)
	now := time.Date(2026, 7, 12, 18, 30, 0, 0, time.UTC)
	effectiveFrom := time.Date(2026, 7, 12, 0, 0, 0, 0, time.UTC)
	clock.EXPECT().Now().Return(now)
	expectPassthroughTx(transactions)

	var updatedProfile eaterdom.Eater
	var appendedGoal eaterdom.MacroGoal
	repository.EXPECT().
		Update(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, profile eaterdom.Eater, goal eaterdom.MacroGoal) (eaterdom.Eater, error) {
			updatedProfile = profile
			appendedGoal = goal
			profile.CurrentGoal = &goal
			return profile, nil
		})

	updated, err := service.UpdateEater(t.Context(), 7, UpdateEaterCommand{
		Name:      " Sasha ",
		Sex:       eaterdom.SexFemale,
		Allergens: []eaterdom.Allergen{" casein ", "casein", "nuts"},
		Goal: MacroGoalCommand{EffectiveFrom: effectiveFrom, Range: eaterdom.MacroRange{
			ProteinMinG: 100, ProteinMaxG: 120,
			FatMinG: 50, FatMaxG: 60,
			CarbMinG: 160, CarbMaxG: 180,
			KcalMin: 1600, KcalMax: 1800,
		}},
	})
	if err != nil {
		t.Fatalf("UpdateEater() error = %v", err)
	}
	if updated.Name != "Sasha" || !slices.Equal(updated.Allergens, []eaterdom.Allergen{"casein", "nuts"}) {
		t.Fatalf("UpdateEater() = %#v, want normalized profile", updated)
	}
	if updatedProfile.ID != 7 || appendedGoal.EaterID != 7 || !appendedGoal.EffectiveFrom.Equal(effectiveFrom) {
		t.Fatalf("Update() profile = %#v, goal = %#v, want eater ID and requested effective date", updatedProfile, appendedGoal)
	}
}

func TestCreateEaterCommandProfileNormalizesValues(t *testing.T) {
	command := CreateEaterCommand{
		Name:      " Sasha ",
		Allergens: []eaterdom.Allergen{" casein ", "casein", "nuts"},
	}

	profile, err := command.profile()
	if err != nil {
		t.Fatalf("CreateEaterCommand.profile() error = %v", err)
	}
	if profile.Name != "Sasha" || profile.Sex != eaterdom.SexUnspecified {
		t.Fatalf("CreateEaterCommand.profile() = %#v, want normalized defaults", profile)
	}
	if !slices.Equal(profile.Allergens, []eaterdom.Allergen{"casein", "nuts"}) {
		t.Fatalf("CreateEaterCommand.profile() allergens = %#v", profile.Allergens)
	}
}

func TestMacroGoalCommandGoalDefaultsEffectiveDate(t *testing.T) {
	now := time.Date(2026, 7, 19, 12, 30, 0, 0, time.FixedZone("YEKT", 5*60*60))
	command := MacroGoalCommand{
		Range: eaterdom.MacroRange{
			ProteinMaxG: 100,
			FatMaxG:     50,
			CarbMaxG:    150,
			KcalMax:     1800,
		},
	}

	goal, err := command.goal(7, now)
	if err != nil {
		t.Fatalf("MacroGoalCommand.goal() error = %v", err)
	}
	if goal.EaterID != 7 || !goal.EffectiveFrom.Equal(now.UTC().Truncate(dayDuration)) {
		t.Fatalf("MacroGoalCommand.goal() = %#v, want eater/default date", goal)
	}
}

func TestCreateFixedMealCommandMealDefaultsFlags(t *testing.T) {
	command := CreateFixedMealCommand{
		MealType: eaterdom.MealTypeBreakfast,
		Label:    " breakfast ",
		Macros:   eaterdom.Macros{ProteinG: 20, FatG: 10, CarbG: 30, Kcal: 300},
	}

	meal := command.meal(3)

	if meal.EaterID != 3 || meal.Label != "breakfast" || !meal.Active {
		t.Fatalf("CreateFixedMealCommand.meal() = %#v, want normalized meal", meal)
	}
	if !meal.Quality.IncludeInAnalytics || !meal.Quality.IncludeInWarnings {
		t.Fatalf("CreateFixedMealCommand.meal() quality = %#v, want default flags", meal.Quality)
	}
}

func TestCreateExclusionCommandExclusionDefaultsSeverity(t *testing.T) {
	command := CreateExclusionCommand{
		TargetKind:  eaterdom.ExclusionTargetRoleTag,
		TargetValue: " protein:lean ",
	}

	exclusion := command.exclusion(5)

	if exclusion.EaterID != 5 || exclusion.TargetValue != "protein:lean" || exclusion.Severity != eaterdom.ExclusionSeverityHard {
		t.Fatalf("CreateExclusionCommand.exclusion() = %#v, want normalized defaults", exclusion)
	}
}

func TestUpdateEaterSchedulesSameDayGoalForNextAvailableDate(t *testing.T) {
	service, repository, transactions, clock := newMockService(t)
	now := time.Date(2026, 7, 12, 18, 30, 0, 0, time.UTC)
	todayGoal := eaterdom.MacroGoal{
		ID:            1,
		EaterID:       1,
		EffectiveFrom: now.UTC().Truncate(dayDuration),
		Range:         eaterdom.MacroRange{ProteinMaxG: 100, FatMaxG: 50, CarbMaxG: 150, KcalMax: 1800},
	}
	clock.EXPECT().Now().Return(now)
	expectPassthroughTx(transactions)
	repository.EXPECT().ListGoals(gomock.Any(), eaterdom.ID(1)).Return([]eaterdom.MacroGoal{todayGoal}, nil)

	var scheduledGoal eaterdom.MacroGoal
	repository.EXPECT().
		Update(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, profile eaterdom.Eater, goal eaterdom.MacroGoal) (eaterdom.Eater, error) {
			scheduledGoal = goal
			profile.CurrentGoal = &todayGoal
			return profile, nil
		})

	updated, err := service.UpdateEater(t.Context(), 1, UpdateEaterCommand{
		Name: "Pasha Updated", Sex: eaterdom.SexMale, Allergens: []eaterdom.Allergen{"nuts"},
		Goal: MacroGoalCommand{Range: eaterdom.MacroRange{ProteinMaxG: 120, FatMaxG: 60, CarbMaxG: 170, KcalMax: 2000}},
	})
	if err != nil {
		t.Fatalf("UpdateEater() error = %v", err)
	}
	wantEffectiveFrom := todayGoal.EffectiveFrom.AddDate(0, 0, 1)
	if !scheduledGoal.EffectiveFrom.Equal(wantEffectiveFrom) {
		t.Fatalf("scheduled goal effective_from = %v, want %v", scheduledGoal.EffectiveFrom, wantEffectiveFrom)
	}
	if updated.CurrentGoal == nil || updated.CurrentGoal.ID != todayGoal.ID {
		t.Fatalf("current goal = %#v, want today's original goal", updated.CurrentGoal)
	}
	if updated.Name != "Pasha Updated" || !slices.Equal(updated.Allergens, []eaterdom.Allergen{"nuts"}) {
		t.Fatalf("updated profile = %#v, want profile/allergen replacement", updated)
	}
}

func TestCreateExclusionRejectsInvalidProductTarget(t *testing.T) {
	service, _, _, _ := newMockService(t)

	_, err := service.CreateExclusion(t.Context(), 1, CreateExclusionCommand{
		TargetKind:  eaterdom.ExclusionTargetProduct,
		TargetValue: "not-an-id",
	})
	assertErrorCode(t, err, apperr.CodeValidation)
}

func TestCreateFixedMealPreservesNilQualityAndDefaultsFlags(t *testing.T) {
	service, repository, _, _ := newMockService(t)
	repository.EXPECT().
		CreateFixedMeal(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, meal eaterdom.FixedMeal) (eaterdom.FixedMeal, error) {
			return meal, nil
		})

	meal, err := service.CreateFixedMeal(t.Context(), 3, CreateFixedMealCommand{
		MealType: eaterdom.MealTypeBreakfast,
		Label:    " breakfast ",
		Macros:   eaterdom.Macros{ProteinG: 20, FatG: 10, CarbG: 30, Kcal: 300},
	})
	if err != nil {
		t.Fatalf("CreateFixedMeal() error = %v", err)
	}
	if meal.Quality.FiberG != nil || !meal.Quality.IncludeInAnalytics || !meal.Quality.IncludeInWarnings {
		t.Fatalf("CreateFixedMeal() quality = %#v, want nil nutrient fields and default flags", meal.Quality)
	}
}

func TestCreateFixedMealAllowsExplicitQualityFlags(t *testing.T) {
	service, repository, _, _ := newMockService(t)
	repository.EXPECT().
		CreateFixedMeal(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, meal eaterdom.FixedMeal) (eaterdom.FixedMeal, error) {
			return meal, nil
		})
	no := false

	meal, err := service.CreateFixedMeal(t.Context(), 3, CreateFixedMealCommand{
		MealType:           eaterdom.MealTypeBreakfast,
		Label:              "breakfast",
		Macros:             eaterdom.Macros{ProteinG: 20, FatG: 10, CarbG: 30, Kcal: 300},
		IncludeInAnalytics: &no,
		IncludeInWarnings:  &no,
	})
	if err != nil {
		t.Fatalf("CreateFixedMeal() error = %v", err)
	}
	if meal.Quality.IncludeInAnalytics || meal.Quality.IncludeInWarnings {
		t.Fatalf("CreateFixedMeal() quality flags = %#v, want explicit false", meal.Quality)
	}
}

func TestCreateGoalRejectsInvalidRange(t *testing.T) {
	service, _, _, clock := newMockService(t)
	clock.EXPECT().Now().Return(time.Now())

	_, err := service.CreateGoal(t.Context(), 1, MacroGoalCommand{Range: eaterdom.MacroRange{ProteinMinG: 10, ProteinMaxG: 5}})
	assertErrorCode(t, err, apperr.CodeValidation)
}

func TestCreateGoalDefaultsEffectiveFromFromClock(t *testing.T) {
	service, repository, _, clock := newMockService(t)
	now := time.Date(2026, 7, 12, 18, 30, 0, 0, time.FixedZone("YEKT", 5*60*60))
	clock.EXPECT().Now().Return(now)
	repository.EXPECT().
		CreateGoal(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, goal eaterdom.MacroGoal) (eaterdom.MacroGoal, error) {
			return goal, nil
		})

	goal, err := service.CreateGoal(t.Context(), 1, MacroGoalCommand{
		Range: eaterdom.MacroRange{ProteinMaxG: 1, FatMaxG: 1, CarbMaxG: 1, KcalMax: 1},
	})
	if err != nil {
		t.Fatalf("CreateGoal() error = %v", err)
	}
	want := now.UTC().Truncate(dayDuration)
	if !goal.EffectiveFrom.Equal(want) {
		t.Fatalf("CreateGoal() effective_from = %v, want %v", goal.EffectiveFrom, want)
	}
}

func TestCreateGoalRejectsNonFiniteAndOutOfRangeMacros(t *testing.T) {
	for name, goal := range map[string]MacroGoalCommand{
		"non-finite numeric":         {Range: eaterdom.MacroRange{ProteinMaxG: math.Inf(1), FatMaxG: 1, CarbMaxG: 1, KcalMax: 1}},
		"numeric precision overflow": {Range: eaterdom.MacroRange{ProteinMaxG: 100000, FatMaxG: 1, CarbMaxG: 1, KcalMax: 1}},
		"integer overflow":           {Range: eaterdom.MacroRange{ProteinMaxG: 1, FatMaxG: 1, CarbMaxG: 1, KcalMax: int(math.MaxInt32) + 1}},
	} {
		t.Run(name, func(t *testing.T) {
			service, _, _, clock := newMockService(t)
			clock.EXPECT().Now().Return(time.Now())

			_, err := service.CreateGoal(t.Context(), 1, goal)
			assertErrorCode(t, err, apperr.CodeValidation)
		})
	}
}

func TestCreateFixedMealRejectsNonFiniteAndOutOfRangeQuality(t *testing.T) {
	sodium := 10_000_000.0
	for name, command := range map[string]CreateFixedMealCommand{
		"non-finite macro": {
			MealType: eaterdom.MealTypeBreakfast,
			Label:    "breakfast",
			Macros:   eaterdom.Macros{ProteinG: math.NaN(), FatG: 1, CarbG: 1, Kcal: 1},
		},
		"quality numeric overflow": {
			MealType: eaterdom.MealTypeBreakfast,
			Label:    "breakfast",
			Macros:   eaterdom.Macros{ProteinG: 1, FatG: 1, CarbG: 1, Kcal: 1},
			Quality:  eaterdom.FixedMealQuality{SodiumMg: &sodium},
		},
	} {
		t.Run(name, func(t *testing.T) {
			service, _, _, _ := newMockService(t)

			_, err := service.CreateFixedMeal(t.Context(), 1, command)
			assertErrorCode(t, err, apperr.CodeValidation)
		})
	}
}

func TestUpdateEaterReturnsConflictWhenRepositoryUpdateFails(t *testing.T) {
	service, repository, transactions, clock := newMockService(t)
	clock.EXPECT().Now().Return(time.Now())
	expectPassthroughTx(transactions)
	repository.EXPECT().
		Update(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(eaterdom.Eater{}, apperr.Conflict("macro goal already exists"))

	_, err := service.UpdateEater(t.Context(), 1, UpdateEaterCommand{
		Name: "Renamed", Sex: eaterdom.SexFemale, Allergens: []eaterdom.Allergen{"nuts"},
		Goal: MacroGoalCommand{EffectiveFrom: time.Date(2026, 7, 12, 0, 0, 0, 0, time.UTC), Range: eaterdom.MacroRange{ProteinMaxG: 1, FatMaxG: 1, CarbMaxG: 1, KcalMax: 1}},
	})
	assertErrorCode(t, err, apperr.CodeConflict)
}

func newMockService(t *testing.T) (*Service, *mocks.MockEaterRepository, *mocks.MockTxRunner, *mocks.MockClock) {
	t.Helper()
	controller := gomock.NewController(t)
	repository := mocks.NewMockEaterRepository(controller)
	transactions := mocks.NewMockTxRunner(controller)
	clock := mocks.NewMockClock(controller)
	return NewService(repository, transactions, clock), repository, transactions, clock
}

func expectPassthroughTx(transactions *mocks.MockTxRunner) {
	transactions.EXPECT().
		WithTx(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		})
}

func assertErrorCode(t *testing.T, err error, want apperr.Code) {
	t.Helper()
	var appError *apperr.Error
	if !errors.As(err, &appError) || appError.Code != want {
		t.Fatalf("error = %v, want code %q", err, want)
	}
}
