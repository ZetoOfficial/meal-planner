// Package swap computes the blast radius of changing one plan component before
// anything is written: what the day's macros and quality become, which meals
// move, which dependent dishes break, and what the shopping list gains or loses.
package swap

import (
	"errors"
	"fmt"
	"time"

	catalogdom "github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
	eaterdom "github.com/ZetoOfficial/meal-planner/internal/domain/eater"
	"github.com/ZetoOfficial/meal-planner/internal/domain/nutrition"
	planningdom "github.com/ZetoOfficial/meal-planner/internal/domain/planning"
	"github.com/ZetoOfficial/meal-planner/internal/quality"
)

// Level names how far a swap reaches, matching the documented blast radius.
type Level string

const (
	LevelFillToFill    Level = "fill_to_fill"
	LevelProteinChange Level = "protein_change"
	LevelRecipeChange  Level = "recipe_change"
	LevelFullRotation  Level = "full_rotation"
)

// Request addresses one slot fill and the product that should replace it.
// Recipe and rotation level swaps need a full re-solve and are not part of v1.
type Request struct {
	SlotFillID   planningdom.SlotFillID
	NewProductID catalogdom.ProductID
}

type EaterDayMacroDelta struct {
	EaterID eaterdom.ID
	Date    time.Time
	Delta   MacroDelta
}

// MacroDelta is signed, so it cannot reuse the nonnegative macro value type.
type MacroDelta struct {
	ProteinG float64
	FatG     float64
	CarbG    float64
	Kcal     int
}

type InvalidatedRecipe struct {
	PlanRecipeID planningdom.PlanRecipeID
	Reason       string
}

type ShoppingDelta struct {
	Added   []catalogdom.ProductID
	Removed []catalogdom.ProductID
}

// QualityDelta reports the hot-path nutrient and coverage movement plus every
// rule whose verdict changed.
type QualityDelta struct {
	FiberG                  float64
	SodiumMg                float64
	SaturatedFatG           float64
	AddedSugarsG            float64
	FruitVegG               float64
	RedMeatGRolling7d       float64
	ProcessedMeatGRolling7d float64
	CoverageScore           float64
	ChangedRules            []RuleChange
}

type RuleChange struct {
	EaterID eaterdom.ID
	Date    time.Time
	Before  nutrition.RuleEvaluationResult
	After   nutrition.RuleEvaluationResult
}

type Preview struct {
	Level              Level
	MacroDiff          []EaterDayMacroDelta
	AffectedMeals      []planningdom.MealID
	InvalidatedRecipes []InvalidatedRecipe
	ShoppingDelta      ShoppingDelta
	QualityDiff        QualityDelta
	Summary            string
	// Plan is the swapped plan. Preview never persists it; ApplySwap does.
	Plan planningdom.Plan
	// EaterDays is the post-swap evaluation, ready to replace the stored
	// quality snapshot when the swap is applied.
	EaterDays []DaySnapshot
}

// DaySnapshot is one eater-day of the swapped plan as it would be stored.
type DaySnapshot struct {
	EaterID      eaterdom.ID
	Date         time.Time
	Basis        nutrition.SummaryBasis
	Day          nutrition.DaySummary
	Rolling      nutrition.RollingSummary
	DayRules     []nutrition.RuleEvaluationResult
	RollingRules []nutrition.RuleEvaluationResult
}

// Input carries everything the deterministic diff needs. History holds the days
// already outside the plan that the rolling window still covers.
type Input struct {
	Plan              planningdom.Plan
	Catalogue         quality.Catalogue
	Ruleset           quality.Ruleset
	RollingWindowDays int
	History           map[eaterdom.ID][]nutrition.DaySummary
}

// ErrUnknownFill and ErrUnknownProduct are the request-level rejections: the
// caller named something the plan or the catalogue does not hold. Every other
// error means the stored plan or the arithmetic is broken, and must not be
// reported back as the owner's mistake.
var (
	ErrUnknownFill    = errors.New("swap targets a slot fill outside the plan")
	ErrUnknownProduct = errors.New("swap targets a product outside the catalogue")
)

// Compute produces the blast radius without touching the supplied plan: the
// swapped plan is a copy, so the caller decides whether to persist it.
func Compute(input Input, request Request) (Preview, error) {
	recipeIndex, fillIndex, err := locateFill(input.Plan, request.SlotFillID)
	if err != nil {
		return Preview{}, err
	}
	oldProductID := input.Plan.Recipes[recipeIndex].Fills[fillIndex].ProductID
	if _, ok := input.Catalogue.Products[request.NewProductID]; !ok {
		return Preview{}, fmt.Errorf("%w: %d", ErrUnknownProduct, request.NewProductID)
	}

	before, err := evaluatePlan(input, input.Plan)
	if err != nil {
		return Preview{}, err
	}
	swapped := withSwappedFill(input.Plan, recipeIndex, fillIndex, request.NewProductID)
	after, err := evaluatePlan(input, swapped)
	if err != nil {
		return Preview{}, err
	}

	preview := Preview{
		Level:              levelOf(input, recipeIndex, fillIndex),
		MacroDiff:          macroDiff(before, after),
		AffectedMeals:      affectedMeals(swapped, recipeIndex),
		InvalidatedRecipes: invalidatedRecipes(swapped, recipeIndex),
		ShoppingDelta:      shoppingDelta(input.Plan, swapped, oldProductID, request.NewProductID),
		QualityDiff:        qualityDiff(before, after),
		Plan:               swapped,
		EaterDays:          snapshots(after),
	}
	preview.Summary = summarise(input.Catalogue, oldProductID, request.NewProductID, preview)
	return preview, nil
}

func locateFill(plan planningdom.Plan, id planningdom.SlotFillID) (recipeIndex, fillIndex int, err error) {
	for i := range plan.Recipes {
		for j := range plan.Recipes[i].Fills {
			if plan.Recipes[i].Fills[j].ID == id {
				return i, j, nil
			}
		}
	}
	return 0, 0, ErrUnknownFill
}

// withSwappedFill deep copies the parts a swap touches so the caller's plan is
// never mutated by a preview.
func withSwappedFill(
	plan planningdom.Plan,
	recipeIndex, fillIndex int,
	productID catalogdom.ProductID,
) planningdom.Plan {
	swapped := plan
	swapped.Recipes = append([]planningdom.PlanRecipe(nil), plan.Recipes...)
	fills := append([]planningdom.SlotFill(nil), plan.Recipes[recipeIndex].Fills...)
	fills[fillIndex].ProductID = productID
	swapped.Recipes[recipeIndex].Fills = fills
	return swapped
}

type dayEvaluation struct {
	eaterID eaterdom.ID
	date    time.Time
	macros  nutrition.Macros
	day     nutrition.DaySummary
	rolling nutrition.RollingSummary
	rules   []nutrition.RuleEvaluationResult
}

func evaluatePlan(input Input, plan planningdom.Plan) ([]dayEvaluation, error) {
	days, err := quality.BuildPlanDays(plan, input.Catalogue)
	if err != nil {
		return nil, err
	}

	ruleset := input.Ruleset
	if ruleset.LegumeServingSizeG <= 0 {
		ruleset = quality.DefaultRuleset()
	}
	window := input.RollingWindowDays
	if window <= 0 {
		window = defaultRollingWindowDays
	}

	summaries := make(map[eaterdom.ID][]nutrition.DaySummary, len(days))
	result := make([]dayEvaluation, 0, len(days))
	for i := range days {
		day := &days[i]
		summary, err := quality.BuildDaySummary(day.Input, quality.SummaryPurposeWarnings)
		if err != nil {
			return nil, err
		}
		history := append(append([]nutrition.DaySummary(nil), input.History[day.EaterID]...), summaries[day.EaterID]...)
		rolling, err := quality.BuildRollingSummary(ruleset, window, rollingWindow(history, &summary, window))
		if err != nil {
			return nil, err
		}
		evaluation := quality.Evaluate(ruleset, day.Input.Sex, summary, &rolling, nil)

		summaries[day.EaterID] = append(summaries[day.EaterID], summary)
		result = append(result, dayEvaluation{
			eaterID: day.EaterID,
			date:    day.Date,
			macros:  summary.Macros,
			day:     summary,
			rolling: rolling,
			rules:   ruleResults(evaluation),
		})
	}
	return result, nil
}

const defaultRollingWindowDays = 7

func rollingWindow(history []nutrition.DaySummary, candidate *nutrition.DaySummary, window int) []*nutrition.DaySummary {
	if len(history) > window-1 {
		history = history[len(history)-(window-1):]
	}
	days := make([]*nutrition.DaySummary, 0, len(history)+1)
	for i := range history {
		days = append(days, &history[i])
	}
	return append(days, candidate)
}

func snapshots(days []dayEvaluation) []DaySnapshot {
	result := make([]DaySnapshot, 0, len(days))
	for i := range days {
		day := &days[i]
		result = append(result, DaySnapshot{
			EaterID:      day.eaterID,
			Date:         day.date,
			Basis:        nutrition.SummaryBasisPlanned,
			Day:          day.day,
			Rolling:      day.rolling,
			DayRules:     partitionRules(day.rules, false),
			RollingRules: partitionRules(day.rules, true),
		})
	}
	return result
}

// partitionRules splits the flat verdict list into the daily and weekly groups
// the snapshot tables store separately.
func partitionRules(rules []nutrition.RuleEvaluationResult, rolling bool) []nutrition.RuleEvaluationResult {
	result := make([]nutrition.RuleEvaluationResult, 0, len(rules))
	for _, rule := range rules {
		if isRollingRule(rule.RuleCode) != rolling {
			continue
		}
		result = append(result, rule)
	}
	return result
}

func isRollingRule(code string) bool {
	switch code {
	case quality.RuleRedMeat, quality.RuleProcessedMeat, quality.RuleLegumes:
		return true
	default:
		return false
	}
}

func ruleResults(evaluation quality.Evaluation) []nutrition.RuleEvaluationResult {
	results := make([]nutrition.RuleEvaluationResult, 0, len(evaluation.Outcomes))
	for _, outcome := range evaluation.Outcomes {
		results = append(results, outcome.Result())
	}
	return results
}
