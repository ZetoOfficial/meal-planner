package solver

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	catalogdom "github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
	eaterdom "github.com/ZetoOfficial/meal-planner/internal/domain/eater"
	mealdom "github.com/ZetoOfficial/meal-planner/internal/domain/meal"
	recipedom "github.com/ZetoOfficial/meal-planner/internal/domain/recipe"
)

type EaterDayInput struct {
	Date               time.Time
	Goal               eaterdom.MacroGoal
	GeneratedMealTypes []mealdom.Type
}

type EaterInput struct {
	Eater      eaterdom.Eater
	Days       []EaterDayInput
	FixedMeals []eaterdom.FixedMeal
}

type Input struct {
	StartDate          time.Time
	Days               int
	CookDate           time.Time
	Novelty            int
	AvailableEquipment []string
	Eaters             []EaterInput
	Products           []catalogdom.Product
	Recipes            []recipedom.Template
}

type Config struct {
	ServingMultipliers []float64
	SearchBudget       int
}

type Planner struct {
	config Config
}

const defaultSearchBudget = 200_000

func New() *Planner {
	return NewWithConfig(Config{
		ServingMultipliers: []float64{0.5, 1, 1.5, 2},
		SearchBudget:       defaultSearchBudget,
	})
}

func NewWithConfig(config Config) *Planner {
	config.ServingMultipliers = append([]float64(nil), config.ServingMultipliers...)
	return &Planner{config: config}
}

func (p *Planner) BuildPlan(ctx context.Context, input Input) (Result, error) {
	if err := p.validate(input); err != nil {
		return Result{}, err
	}

	candidates := FilterCandidates(input)
	builder := newPlanBuilder(input)
	for eaterIndex := range input.Eaters {
		eaterInput := &input.Eaters[eaterIndex]
		eaterCandidates := &candidates.Eaters[eaterIndex]
		for dayIndex := range eaterInput.Days {
			day := &eaterInput.Days[dayIndex]
			fixedMeals, fixedMacros, err := fixedMealsForDay(*eaterInput, day.Date)
			if err != nil {
				return Result{}, err
			}

			solution, state, err := searchDay(ctx, p.config, *day, fixedMacros, eaterCandidates.Recipes)
			if err != nil {
				return Result{}, err
			}
			if state != searchFound {
				return Result{Failure: &Failure{
					Code:       failureCode(state),
					EaterID:    eaterInput.Eater.ID,
					Date:       day.Date,
					Exhaustive: state == searchExhausted,
					Rejections: rejectionsForEater(candidates.Rejections, eaterInput.Eater.ID),
				}}, nil
			}
			builder.addDay(*eaterInput, *day, fixedMeals, solution)
		}
	}

	plan, err := builder.build()
	if err != nil {
		return Result{}, err
	}
	return Result{Plan: plan}, nil
}

func (p *Planner) validate(input Input) error {
	if p == nil {
		return errors.New("solver planner is nil")
	}
	if err := validateConfig(p.config); err != nil {
		return err
	}
	return validateInput(input)
}

func validateConfig(config Config) error {
	if len(config.ServingMultipliers) == 0 || config.SearchBudget <= 0 {
		return errors.New("solver config requires serving multipliers and a positive search budget")
	}
	for _, multiplier := range config.ServingMultipliers {
		if math.IsNaN(multiplier) || math.IsInf(multiplier, 0) || multiplier <= 0 {
			return errors.New("solver serving multipliers must be finite and positive")
		}
	}
	return nil
}

func validateInput(input Input) error {
	if input.StartDate.IsZero() || input.CookDate.IsZero() || input.Days <= 0 || input.Novelty < 0 || input.Novelty > 3 || len(input.Eaters) == 0 {
		return errors.New("invalid solver horizon or novelty")
	}
	for eaterIndex := range input.Eaters {
		if err := validateEaterInput(input.Eaters[eaterIndex], input.Days); err != nil {
			return err
		}
	}
	return nil
}

func validateEaterInput(input EaterInput, planDays int) error {
	if !input.Eater.ID.Valid() || len(input.Days) != planDays {
		return errors.New("solver input requires one dated target per eater and plan day")
	}
	fixedTypes, err := activeFixedMealTypes(input)
	if err != nil {
		return err
	}
	for dayIndex := range input.Days {
		if err := validateEaterDay(input.Eater.ID, input.Days[dayIndex], fixedTypes); err != nil {
			return err
		}
	}
	return nil
}

func activeFixedMealTypes(input EaterInput) (map[mealdom.Type]struct{}, error) {
	result := make(map[mealdom.Type]struct{}, len(input.FixedMeals))
	for i := range input.FixedMeals {
		fixed := &input.FixedMeals[i]
		if !fixed.Active {
			continue
		}
		if fixed.EaterID != input.Eater.ID || !fixed.MealType.Valid() || !fixed.Macros.Valid() {
			return nil, fmt.Errorf("invalid fixed meal for eater %d", input.Eater.ID)
		}
		result[fixed.MealType] = struct{}{}
	}
	return result, nil
}

func validateEaterDay(eaterID eaterdom.ID, day EaterDayInput, fixedTypes map[mealdom.Type]struct{}) error {
	if day.Date.IsZero() || day.Goal.EaterID != eaterID || !day.Goal.Range.Valid() || len(day.GeneratedMealTypes) == 0 {
		return fmt.Errorf("invalid solver target for eater %d", eaterID)
	}
	seen := make(map[mealdom.Type]struct{}, len(day.GeneratedMealTypes))
	for _, mealType := range day.GeneratedMealTypes {
		_, duplicatesFixed := fixedTypes[mealType]
		_, duplicateTarget := seen[mealType]
		if !mealType.Valid() || duplicatesFixed || duplicateTarget {
			return fmt.Errorf("invalid generated meal targets for eater %d", eaterID)
		}
		seen[mealType] = struct{}{}
	}
	return nil
}

func failureCode(state searchState) FailureCode {
	if state == searchLimited {
		return FailureSearchLimitReached
	}
	return FailureMacroInfeasible
}

func rejectionsForEater(rejections []HardRejection, eaterID eaterdom.ID) []HardRejection {
	result := make([]HardRejection, 0)
	for i := range rejections {
		if rejections[i].EaterID == eaterID {
			result = append(result, rejections[i])
		}
	}
	return result
}
