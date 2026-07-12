package eater

import (
	"context"
	"errors"
	"math"
	"slices"
	"testing"
	"time"

	eaterdom "github.com/ZetoOfficial/meal-planner/internal/domain/eater"
	apperr "github.com/ZetoOfficial/meal-planner/internal/errors"
)

func TestUpdateEaterReplacesAllergensAndAppendsGoal(t *testing.T) {
	repository := &eaterRepositoryStub{}
	service := NewService(repository, passthroughTxRunner{})

	updated, err := service.UpdateEater(t.Context(), 7, UpdateEaterCommand{
		Name:      " Sasha ",
		Sex:       eaterdom.SexFemale,
		Allergens: []eaterdom.Allergen{" casein ", "casein", "nuts"},
		Goal: MacroGoalCommand{EffectiveFrom: time.Date(2026, 7, 12, 0, 0, 0, 0, time.UTC), Range: eaterdom.MacroRange{
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
	if len(repository.updates) != 1 || repository.updates[0].Goal.EffectiveFrom != time.Date(2026, 7, 12, 0, 0, 0, 0, time.UTC) {
		t.Fatalf("UpdateEater() updates = %#v, want one appended goal", repository.updates)
	}
}

func TestUpdateEaterSchedulesSameDayGoalForNextAvailableDate(t *testing.T) {
	repository := &sameDayEaterRepository{}
	service := NewService(repository, passthroughTxRunner{})
	created, err := service.CreateEater(t.Context(), CreateEaterCommand{
		Name: "Pasha", Sex: eaterdom.SexMale, Allergens: []eaterdom.Allergen{"casein"},
		Goal: MacroGoalCommand{Range: eaterdom.MacroRange{ProteinMaxG: 100, FatMaxG: 50, CarbMaxG: 150, KcalMax: 1800}},
	})
	if err != nil {
		t.Fatalf("CreateEater() error = %v", err)
	}

	updated, err := service.UpdateEater(t.Context(), created.ID, UpdateEaterCommand{
		Name: "Pasha Updated", Sex: eaterdom.SexMale, Allergens: []eaterdom.Allergen{"nuts"},
		Goal: MacroGoalCommand{Range: eaterdom.MacroRange{ProteinMaxG: 120, FatMaxG: 60, CarbMaxG: 170, KcalMax: 2000}},
	})
	if err != nil {
		t.Fatalf("UpdateEater() error = %v", err)
	}
	goals, err := service.ListGoals(t.Context(), created.ID)
	if err != nil {
		t.Fatalf("ListGoals() error = %v", err)
	}
	if len(goals) != 2 || !goals[1].EffectiveFrom.Equal(goals[0].EffectiveFrom.AddDate(0, 0, 1)) {
		t.Fatalf("goal history = %#v, want immutable today goal and update scheduled tomorrow", goals)
	}
	if updated.CurrentGoal == nil || updated.CurrentGoal.ID != goals[0].ID {
		t.Fatalf("current goal = %#v, want today's original goal", updated.CurrentGoal)
	}
	if updated.Name != "Pasha Updated" || !slices.Equal(updated.Allergens, []eaterdom.Allergen{"nuts"}) {
		t.Fatalf("updated profile = %#v, want profile/allergen replacement in same transaction", updated)
	}
}

func TestCreateExclusionRejectsInvalidProductTarget(t *testing.T) {
	service := NewService(&eaterRepositoryStub{}, passthroughTxRunner{})

	_, err := service.CreateExclusion(t.Context(), 1, CreateExclusionCommand{
		TargetKind:  eaterdom.ExclusionTargetProduct,
		TargetValue: "not-an-id",
	})
	if !errors.Is(err, apperr.ErrValidation) {
		t.Fatalf("CreateExclusion() error = %v, want validation", err)
	}
}

func TestCreateFixedMealPreservesNilQualityAndDefaultsFlags(t *testing.T) {
	repository := &eaterRepositoryStub{}
	service := NewService(repository, passthroughTxRunner{})

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
	repository := &eaterRepositoryStub{}
	service := NewService(repository, passthroughTxRunner{})
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
	service := NewService(&eaterRepositoryStub{}, passthroughTxRunner{})
	_, err := service.CreateGoal(t.Context(), 1, MacroGoalCommand{Range: eaterdom.MacroRange{ProteinMinG: 10, ProteinMaxG: 5}})
	if !errors.Is(err, apperr.ErrValidation) {
		t.Fatalf("CreateGoal() error = %v, want validation", err)
	}
}

func TestCreateGoalRejectsNonFiniteAndOutOfRangeMacros(t *testing.T) {
	service := NewService(&eaterRepositoryStub{}, passthroughTxRunner{})
	for name, goal := range map[string]MacroGoalCommand{
		"non-finite numeric":         {Range: eaterdom.MacroRange{ProteinMaxG: math.Inf(1), FatMaxG: 1, CarbMaxG: 1, KcalMax: 1}},
		"numeric precision overflow": {Range: eaterdom.MacroRange{ProteinMaxG: 100000, FatMaxG: 1, CarbMaxG: 1, KcalMax: 1}},
		"integer overflow":           {Range: eaterdom.MacroRange{ProteinMaxG: 1, FatMaxG: 1, CarbMaxG: 1, KcalMax: int(math.MaxInt32) + 1}},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := service.CreateGoal(t.Context(), 1, goal)
			if !errors.Is(err, apperr.ErrValidation) {
				t.Fatalf("CreateGoal() error = %v, want validation", err)
			}
		})
	}
}

func TestCreateFixedMealRejectsNonFiniteAndOutOfRangeQuality(t *testing.T) {
	repository := &eaterRepositoryStub{}
	service := NewService(repository, passthroughTxRunner{})
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
			_, err := service.CreateFixedMeal(t.Context(), 1, command)
			if !errors.Is(err, apperr.ErrValidation) {
				t.Fatalf("CreateFixedMeal() error = %v, want validation", err)
			}
		})
	}
}

func TestUpdateEaterRollsBackProfileAndAllergensWhenGoalAppendFails(t *testing.T) {
	repository := &rollbackEaterRepository{profile: eaterdom.Eater{
		ID: 1, Name: "Pasha", Sex: eaterdom.SexMale, Allergens: []eaterdom.Allergen{"casein"},
	}}
	transactions := &eaterRollbackTxRunner{repository: repository}
	service := NewService(repository, transactions)

	_, err := service.UpdateEater(t.Context(), 1, UpdateEaterCommand{
		Name: "Renamed", Sex: eaterdom.SexFemale, Allergens: []eaterdom.Allergen{"nuts"},
		Goal: MacroGoalCommand{EffectiveFrom: time.Date(2026, 7, 12, 0, 0, 0, 0, time.UTC), Range: eaterdom.MacroRange{ProteinMaxG: 1, FatMaxG: 1, CarbMaxG: 1, KcalMax: 1}},
	})
	if !errors.Is(err, apperr.ErrConflict) {
		t.Fatalf("UpdateEater() error = %v, want conflict", err)
	}
	if repository.profile.Name != "Pasha" || repository.profile.Sex != eaterdom.SexMale || !slices.Equal(repository.profile.Allergens, []eaterdom.Allergen{"casein"}) {
		t.Fatalf("profile after failed update = %#v, want original profile and allergens", repository.profile)
	}
}

type passthroughTxRunner struct{}

func (passthroughTxRunner) WithTx(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

type eaterUpdate struct {
	Profile eaterdom.Eater
	Goal    eaterdom.MacroGoal
}

type eaterRepositoryStub struct {
	updates []eaterUpdate
}

type sameDayEaterRepository struct {
	eaterRepositoryStub
	profile eaterdom.Eater
	goals   []eaterdom.MacroGoal
}

func (r *sameDayEaterRepository) Create(_ context.Context, profile eaterdom.Eater, goal eaterdom.MacroGoal) (eaterdom.Eater, error) {
	profile.ID, goal.ID, goal.EaterID = 1, 1, 1
	r.profile, r.goals = profile, []eaterdom.MacroGoal{goal}
	r.profile.CurrentGoal = &r.goals[0]
	return r.profile, nil
}

func (r *sameDayEaterRepository) ListGoals(_ context.Context, eaterID eaterdom.ID) ([]eaterdom.MacroGoal, error) {
	if eaterID != r.profile.ID {
		return nil, apperr.NotFound("eater")
	}
	return slices.Clone(r.goals), nil
}

func (r *sameDayEaterRepository) Update(_ context.Context, profile eaterdom.Eater, goal eaterdom.MacroGoal) (eaterdom.Eater, error) {
	for _, existing := range r.goals {
		if existing.EffectiveFrom.Equal(goal.EffectiveFrom) {
			return eaterdom.Eater{}, apperr.Conflict("macro goal already exists")
		}
	}
	goal.ID = eaterdom.MacroGoalID(len(r.goals) + 1)
	r.goals = append(r.goals, goal)
	r.profile = profile
	r.profile.CurrentGoal = eaterdom.CurrentGoalAt(r.goals, time.Now().UTC())
	return r.profile, nil
}

type rollbackEaterRepository struct {
	eaterRepositoryStub
	profile eaterdom.Eater
}

func (r *rollbackEaterRepository) Update(_ context.Context, profile eaterdom.Eater, _ eaterdom.MacroGoal) (eaterdom.Eater, error) {
	r.profile = profile // Mirrors the profile/allergen writes occurring before the unique goal-date failure.
	return eaterdom.Eater{}, apperr.Conflict("macro goal already exists")
}

type eaterRollbackTxRunner struct {
	repository *rollbackEaterRepository
}

func (r *eaterRollbackTxRunner) WithTx(ctx context.Context, fn func(context.Context) error) error {
	before := r.repository.profile
	before.Allergens = slices.Clone(before.Allergens)
	if err := fn(ctx); err != nil {
		r.repository.profile = before
		return err
	}
	return nil
}

func (r *eaterRepositoryStub) Create(context.Context, eaterdom.Eater, eaterdom.MacroGoal) (eaterdom.Eater, error) {
	return eaterdom.Eater{}, nil
}

func (r *eaterRepositoryStub) DeleteExclusion(context.Context, eaterdom.ExclusionID) error {
	return nil
}

func (r *eaterRepositoryStub) Get(context.Context, eaterdom.ID) (eaterdom.Eater, error) {
	return eaterdom.Eater{}, nil
}
func (r *eaterRepositoryStub) List(context.Context) ([]eaterdom.Eater, error) { return nil, nil }
func (r *eaterRepositoryStub) ListExclusions(context.Context, eaterdom.ID) ([]eaterdom.Exclusion, error) {
	return nil, nil
}

func (r *eaterRepositoryStub) ListFixedMeals(context.Context, eaterdom.ID) ([]eaterdom.FixedMeal, error) {
	return nil, nil
}

func (r *eaterRepositoryStub) ListGoals(context.Context, eaterdom.ID) ([]eaterdom.MacroGoal, error) {
	return nil, nil
}

func (r *eaterRepositoryStub) CreateExclusion(_ context.Context, exclusion eaterdom.Exclusion) (eaterdom.Exclusion, error) {
	return exclusion, nil
}

func (r *eaterRepositoryStub) CreateFixedMeal(_ context.Context, meal eaterdom.FixedMeal) (eaterdom.FixedMeal, error) {
	return meal, nil
}

func (r *eaterRepositoryStub) CreateGoal(_ context.Context, goal eaterdom.MacroGoal) (eaterdom.MacroGoal, error) {
	return goal, nil
}
func (r *eaterRepositoryStub) ProductExists(context.Context, int64) (bool, error)  { return false, nil }
func (r *eaterRepositoryStub) RoleTagExists(context.Context, string) (bool, error) { return false, nil }
func (r *eaterRepositoryStub) Update(_ context.Context, profile eaterdom.Eater, goal eaterdom.MacroGoal) (eaterdom.Eater, error) {
	r.updates = append(r.updates, eaterUpdate{Profile: profile, Goal: goal})
	profile.CurrentGoal = &goal
	return profile, nil
}
