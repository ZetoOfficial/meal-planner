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
	// RecipeLastUsed holds the last date each template was planned. A template
	// missing from the map has never been planned.
	RecipeLastUsed map[recipedom.ID]time.Time
	// FreshRecipes names drafts generated for this very request; at novelty 3
	// the plan is built around one of them.
	FreshRecipes []recipedom.ID
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
	// The novelty dial first looks for a day that hosts a novel component at no
	// quality cost. Only when the whole plan has no such day is it rebuilt with
	// the component forced into the first day that can carry it, so levels 2–3
	// still always deliver one when any day allows it.
	result, novel, err := p.buildPlan(ctx, input, candidates, false)
	if err != nil || result.Failure != nil || novel || len(unseenRecipes(input)) == 0 {
		return result, err
	}
	forced, novel, err := p.buildPlan(ctx, input, candidates, true)
	if err != nil {
		return Result{}, err
	}
	if forced.Failure != nil || !novel {
		return result, nil
	}
	return forced, nil
}

// buildPlan runs one pass over every eater-day and reports whether the plan
// ended up with a novel component.
func (p *Planner) buildPlan(
	ctx context.Context,
	input Input,
	candidates Candidates,
	forceUnseen bool,
) (Result, bool, error) {
	builder := newPlanBuilder(input)
	evaluations := make([]EaterDayQuality, 0, len(input.Eaters)*input.Days)
	priorUses := make(map[int64]int)
	unseen := unseenRecipes(input)
	novel := false
	keys := shuffleKeys(input)
	for eaterIndex := range input.Eaters {
		eaterInput := &input.Eaters[eaterIndex]
		eaterCandidates := &candidates.Eaters[eaterIndex]
		softExcluded := softExcludedProducts(eaterInput.Eater, eaterCandidates.Products)
		history := append([]nutrition.DaySummary(nil), eaterInput.History...)
		for dayIndex := range eaterInput.Days {
			day := &eaterInput.Days[dayIndex]
			fixedMeals, fixedMacros, err := fixedMealsForDay(*eaterInput, day.Date)
			if err != nil {
				return Result{}, false, err
			}

			evaluator := newDayEvaluator(input, *eaterInput, history)
			preferences := dayPreferences{
				priorUses:    priorUses,
				softExcluded: softExcluded,
				shuffleKeys:  keys,
				unseen:       unseen,
				forceUnseen:  forceUnseen,
				planned:      builder.hasBatch,
			}
			ranked, state, unreachable, err := searchDay(ctx, p.config, evaluator, *day, fixedMacros, eaterCandidates.Recipes, preferences)
			if err != nil {
				return Result{}, false, err
			}
			if state != searchFound {
				return Result{Failure: &Failure{
					Code:             failureCode(state),
					EaterID:          eaterInput.Eater.ID,
					Date:             day.Date,
					Exhaustive:       state == searchExhausted,
					Rejections:       rejectionsForEater(candidates.Rejections, eaterInput.Eater.ID),
					MacroConstraints: unreachable,
				}}, false, nil
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
			if preferences.hasUnseen(ranked.solution) {
				// One novel component satisfies the dial for the whole plan.
				unseen = nil
				novel = true
			}
		}
	}

	plan, err := builder.build()
	if err != nil {
		return Result{}, false, err
	}
	return Result{Plan: plan, Quality: evaluations}, novel, nil
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
