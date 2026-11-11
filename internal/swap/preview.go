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
	"github.com/ZetoOfficial/meal-planner/internal/solver"
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
	// MacroGoalBreaches counts the eater-days the swap pushes outside their
	// macro range (HC-8); it is part of the price the owner confirms.
	MacroGoalBreaches int
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
	// Score is the 0..100 quality score of the day, so the plan headline can
	// be refreshed together with the snapshot.
	Score float64
}

// Input carries everything the deterministic diff needs. History holds the days
// already outside the plan that the rolling window still covers.
type Input struct {
	Plan              planningdom.Plan
	Catalogue         quality.Catalogue
	Ruleset           quality.Ruleset
	RollingWindowDays int
	History           map[eaterdom.ID][]nutrition.DaySummary
	// Goals holds the macro range each eater-day was planned against. A swap
	// does not rebalance portions, so the preview says how many days it would
	// push outside their range; empty skips the check.
	Goals []GoalTarget
	// AvailableEquipment limits recipe-level swaps to what the household can
	// cook; nil skips the check.
	AvailableEquipment []string
}

// ErrUnknownFill and ErrUnknownProduct are the request-level rejections: the
// caller named something the plan or the catalogue does not hold. Every other
// error means the stored plan or the arithmetic is broken, and must not be
// reported back as the owner's mistake.
var (
	ErrUnknownFill     = errors.New("swap targets a slot fill outside the plan")
	ErrUnknownProduct  = errors.New("swap targets a product outside the catalogue")
	ErrUnknownRecipe   = errors.New("swap targets a plan recipe outside the plan")
	ErrUnknownTemplate = errors.New("swap targets a recipe template outside the library")
)

// IncompatibleError rejects a swap that would break a hard constraint: a
// product that does not fit the slot or that an eater of the batch must not
// eat, or a template the plan cannot host. It is the owner's request that is
// wrong, so it is reported as a validation failure.
type IncompatibleError struct {
	Reason string
}

func (e *IncompatibleError) Error() string { return "swap is not allowed: " + e.Reason }

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

	if err := checkFill(input, recipeIndex, fillIndex, request.NewProductID); err != nil {
		return Preview{}, err
	}

	swapped, err := recomputePlannedMacros(
		withSwappedFill(input.Plan, recipeIndex, fillIndex, request.NewProductID),
		input.Catalogue,
	)
	if err != nil {
		return Preview{}, err
	}
	preview, err := diffPlans(input, swapped, recipeIndex)
	if err != nil {
		return Preview{}, err
	}
	preview.Level = levelOf(input, recipeIndex, fillIndex)
	preview.Summary = summarise(
		productName(input.Catalogue, oldProductID),
		productName(input.Catalogue, request.NewProductID),
		preview,
	)
	return preview, nil
}

// diffPlans evaluates the plan before and after a change to one batch and
// reports everything that moved.
func diffPlans(input Input, swapped planningdom.Plan, recipeIndex int) (Preview, error) {
	before, err := evaluatePlan(input, input.Plan)
	if err != nil {
		return Preview{}, err
	}
	after, err := evaluatePlan(input, swapped)
	if err != nil {
		return Preview{}, err
	}
	return Preview{
		MacroDiff:          macroDiff(before, after),
		AffectedMeals:      affectedMeals(swapped, recipeIndex),
		InvalidatedRecipes: invalidatedRecipes(input.Plan, recipeIndex),
		ShoppingDelta:      shoppingDelta(input.Plan, swapped),
		QualityDiff:        qualityDiff(before, after),
		Plan:               swapped,
		EaterDays:          snapshots(after),
		MacroGoalBreaches:  macroGoalBreaches(input.Goals, before, after),
	}, nil
}

// macroGoalBreaches counts the days the change itself pushes outside their
// goal. A day that was already out of range before is not charged to it.
func macroGoalBreaches(goals []GoalTarget, before, after []dayEvaluation) int {
	if len(goals) == 0 {
		return 0
	}
	targets := goalIndex(goals)
	previous := indexDays(before)
	breaches := 0
	for i := range after {
		key := dayKey(after[i].eaterID, after[i].date)
		target, ok := targets[key]
		if !ok || target.Contains(after[i].macros) {
			continue
		}
		if old, existed := previous[key]; existed && !target.Contains(old.macros) {
			continue
		}
		breaches++
	}
	return breaches
}

// checkFill keeps a manual product change inside the hard constraints: the
// product must fit the slot's role (HC-6) and be safe for everyone who eats
// from the batch (HC-1, HC-2, HC-9, HC-10).
func checkFill(input Input, recipeIndex, fillIndex int, productID catalogdom.ProductID) error {
	product := input.Catalogue.Products[productID]
	recipe := input.Plan.Recipes[recipeIndex]
	if template, ok := input.Catalogue.Templates[recipe.TemplateID]; ok {
		for _, slot := range template.Slots {
			if slot.ID == recipe.Fills[fillIndex].SlotID && !hasRole(product, slot.RoleTagCode) {
				return &IncompatibleError{Reason: fmt.Sprintf(
					"%s does not have the slot role %s", productLabel(product), slot.RoleTagCode,
				)}
			}
		}
	}
	return checkSafeForEaters(input, recipeIndex, product)
}

func checkSafeForEaters(input Input, recipeIndex int, product catalogdom.Product) error {
	for _, eaterID := range batchEaters(input.Plan, recipeIndex) {
		eater, ok := input.Catalogue.Eaters[eaterID]
		if !ok {
			continue
		}
		if rejection := solver.RejectProduct(eater, product); rejection != nil {
			return &IncompatibleError{Reason: fmt.Sprintf(
				"%s is excluded for eater %d (%s: %s)", productLabel(product), eaterID, rejection.Code, rejection.Detail,
			)}
		}
	}
	return nil
}

// batchEaters lists, in plan order, everyone who eats from one batch.
func batchEaters(plan planningdom.Plan, recipeIndex int) []eaterdom.ID {
	seen := make(map[eaterdom.ID]struct{})
	eaters := make([]eaterdom.ID, 0)
	for i := range plan.Meals {
		for _, component := range plan.Meals[i].Components {
			if component.PlanRecipeIndex != recipeIndex {
				continue
			}
			if _, exists := seen[plan.Meals[i].EaterID]; !exists {
				seen[plan.Meals[i].EaterID] = struct{}{}
				eaters = append(eaters, plan.Meals[i].EaterID)
			}
		}
	}
	return eaters
}

func hasRole(product catalogdom.Product, code string) bool {
	for _, role := range product.Roles {
		if role.Code == code {
			return true
		}
	}
	return false
}

func productLabel(product catalogdom.Product) string {
	if product.Name != "" {
		return product.Name
	}
	return fmt.Sprintf("product %d", product.ID)
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
	score   float64
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
			score:   evaluation.Score,
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
			Score:        day.score,
		})
	}
	return result
}

// partitionRules splits the flat verdict list into the daily and weekly groups
// the snapshot tables store separately.
func partitionRules(rules []nutrition.RuleEvaluationResult, rolling bool) []nutrition.RuleEvaluationResult {
	result := make([]nutrition.RuleEvaluationResult, 0, len(rules))
	for _, rule := range rules {
		if quality.IsRollingRule(rule.RuleCode) != rolling {
			continue
		}
		result = append(result, rule)
	}
	return result
}

func ruleResults(evaluation quality.Evaluation) []nutrition.RuleEvaluationResult {
	results := make([]nutrition.RuleEvaluationResult, 0, len(evaluation.Outcomes))
	for _, outcome := range evaluation.Outcomes {
		results = append(results, outcome.Result())
	}
	return results
}
