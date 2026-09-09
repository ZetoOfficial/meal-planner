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
	"github.com/ZetoOfficial/meal-planner/internal/domain/nutrition"
	recipedom "github.com/ZetoOfficial/meal-planner/internal/domain/recipe"
	"github.com/ZetoOfficial/meal-planner/internal/quality"
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
	// History holds planned or actual day summaries that already fall inside
	// the rolling window, oldest first, so weekly rules see the real horizon.
	History []nutrition.DaySummary
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
	// Ruleset overrides the default quality rules; nil keeps the MVP defaults.
	Ruleset           *quality.Ruleset
	RollingWindowDays int
}

type Config struct {
	ServingMultipliers []float64
	SearchBudget       int
	// CandidateBudget bounds how many macro-feasible candidates the objective
	// ranks per eater-day before it settles on the best one seen.
	CandidateBudget int
}

type Planner struct {
	config Config
}

const (
	defaultSearchBudget    = 200_000
	defaultCandidateBudget = 256
)

func New() *Planner {
	return NewWithConfig(Config{
		ServingMultipliers: []float64{0.5, 1, 1.5, 2},
		SearchBudget:       defaultSearchBudget,
		CandidateBudget:    defaultCandidateBudget,
	})
}

func NewWithConfig(config Config) *Planner {
	config.ServingMultipliers = append([]float64(nil), config.ServingMultipliers...)
	if config.CandidateBudget <= 0 {
		config.CandidateBudget = defaultCandidateBudget
	}
	return &Planner{config: config}
}

func (p *Planner) BuildPlan(ctx context.Context, input Input) (Result, error) {
	if err := p.validate(input); err != nil {
		return Result{}, err
	}

	candidates := FilterCandidates(input)
	builder := newPlanBuilder(input)
	evaluations := make([]EaterDayQuality, 0, len(input.Eaters)*input.Days)
	priorUses := make(map[int64]int)
	for eaterIndex := range input.Eaters {
		eaterInput := &input.Eaters[eaterIndex]
		eaterCandidates := &candidates.Eaters[eaterIndex]
		history := append([]nutrition.DaySummary(nil), eaterInput.History...)
		for dayIndex := range eaterInput.Days {
			day := &eaterInput.Days[dayIndex]
			fixedMeals, fixedMacros, err := fixedMealsForDay(*eaterInput, day.Date)
			if err != nil {
				return Result{}, err
			}

			evaluator := newDayEvaluator(input, *eaterInput, history)
			ranked, state, err := searchDay(ctx, p.config, evaluator, *day, fixedMacros, eaterCandidates.Recipes, priorUses)
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
			builder.addDay(*eaterInput, *day, fixedMeals, ranked.solution)
			evaluations = append(evaluations, EaterDayQuality{
				EaterID:    eaterInput.Eater.ID,
				Date:       day.Date,
				Day:        ranked.day,
				Rolling:    ranked.rolling,
				Evaluation: ranked.evaluation,
			})
			history = append(history, ranked.day)
			recordUses(priorUses, ranked.solution)
		}
	}

	plan, err := builder.build()
	if err != nil {
		return Result{}, err
	}
	return Result{Plan: plan, Quality: evaluations}, nil
}

func recordUses(priorUses map[int64]int, solution daySolution) {
	for i := range solution.uses {
		priorUses[int64(solution.uses[i].variant.recipe.ID)]++
	}
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
