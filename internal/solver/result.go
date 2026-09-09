package solver

import (
	"errors"
	"time"

	eaterdom "github.com/ZetoOfficial/meal-planner/internal/domain/eater"
	"github.com/ZetoOfficial/meal-planner/internal/domain/nutrition"
	planningdom "github.com/ZetoOfficial/meal-planner/internal/domain/planning"
	"github.com/ZetoOfficial/meal-planner/internal/quality"
)

type FailureCode string

const (
	FailureMacroInfeasible    FailureCode = "macro_infeasible"
	FailureSearchLimitReached FailureCode = "search_limit_reached"
)

type Failure struct {
	Code       FailureCode
	EaterID    eaterdom.ID
	Date       time.Time
	Exhaustive bool
	Rejections []HardRejection
	// MacroConstraints lists target bounds the candidates cannot reach at all.
	MacroConstraints []MacroConstraint
}

// EaterDayQuality is the evaluation that ranked the selected plan, reused
// verbatim for API explanations.
type EaterDayQuality struct {
	EaterID    eaterdom.ID
	Date       time.Time
	Day        nutrition.DaySummary
	Rolling    nutrition.RollingSummary
	Evaluation quality.Evaluation
}

type Result struct {
	Plan    planningdom.Plan
	Quality []EaterDayQuality
	Failure *Failure
}

type allocationDraft struct {
	mealIndex      int
	componentIndex int
	recipeIndex    int
	servingUnits   float64
}

type planBuilder struct {
	plan                planningdom.Plan
	recipeServingTotals []float64
	recipeBaseServings  []int
	allocations         []allocationDraft
}

func newPlanBuilder(input Input) *planBuilder {
	return &planBuilder{plan: planningdom.Plan{
		StartDate: input.StartDate,
		Days:      input.Days,
		CookDate:  input.CookDate,
		Novelty:   input.Novelty,
	}}
}

func fixedMealsForDay(eaterInput EaterInput, date time.Time) ([]planningdom.Meal, nutrition.Macros, error) {
	meals := make([]planningdom.Meal, 0, len(eaterInput.FixedMeals))
	total := nutrition.Macros{}
	for i := range eaterInput.FixedMeals {
		fixed := &eaterInput.FixedMeals[i]
		if !fixed.Active {
			continue
		}
		var err error
		total, err = total.Add(fixed.Macros)
		if err != nil {
			return nil, nutrition.Macros{}, err
		}
		fixedID := fixed.ID
		meals = append(meals, planningdom.Meal{
			EaterID:     eaterInput.Eater.ID,
			Date:        date,
			Type:        fixed.MealType,
			FixedMealID: &fixedID,
			Planned:     fixed.Macros,
		})
	}
	return meals, total, nil
}

func (b *planBuilder) addDay(eaterInput EaterInput, day EaterDayInput, fixedMeals []planningdom.Meal, solution daySolution) {
	b.plan.EaterDays = append(b.plan.EaterDays, planningdom.EaterDay{
		EaterID:            eaterInput.Eater.ID,
		Date:               day.Date,
		MacroGoalID:        day.Goal.ID,
		GeneratedMealTypes: append([]eaterdom.MealType(nil), day.GeneratedMealTypes...),
	})
	b.plan.Meals = append(b.plan.Meals, fixedMeals...)
	generatedStart := len(b.plan.Meals)
	for i, mealType := range day.GeneratedMealTypes {
		b.plan.Meals = append(b.plan.Meals, planningdom.Meal{
			EaterID: eaterInput.Eater.ID,
			Date:    day.Date,
			Type:    mealType,
			Planned: solution.mealMacros[i],
		})
	}

	for i := range solution.uses {
		use := &solution.uses[i]
		recipeIndex := b.planRecipe(use.variant)
		b.recipeServingTotals[recipeIndex] += use.servingUnits
		mealIndex := generatedStart + use.mealIndex
		componentIndex := len(b.plan.Meals[mealIndex].Components)
		b.plan.Meals[mealIndex].Components = append(b.plan.Meals[mealIndex].Components, planningdom.ComponentAllocation{
			PlanRecipeIndex: recipeIndex,
		})
		b.allocations = append(b.allocations, allocationDraft{
			mealIndex:      mealIndex,
			componentIndex: componentIndex,
			recipeIndex:    recipeIndex,
			servingUnits:   use.servingUnits,
		})
	}
}

func (b *planBuilder) planRecipe(variant componentVariant) int {
	for i := range b.plan.Recipes {
		if b.plan.Recipes[i].TemplateID == variant.recipe.ID && sameFills(b.plan.Recipes[i].Fills, variant.baseFills) {
			return i
		}
	}
	b.plan.Recipes = append(b.plan.Recipes, planningdom.PlanRecipe{
		TemplateID:      variant.recipe.ID,
		ExpectedOutputG: cloneFloat(variant.recipe.ExpectedOutputG),
		Fills:           append([]planningdom.SlotFill(nil), variant.baseFills...),
	})
	b.recipeServingTotals = append(b.recipeServingTotals, 0)
	b.recipeBaseServings = append(b.recipeBaseServings, variant.recipe.BaseServings)
	return len(b.plan.Recipes) - 1
}

func (b *planBuilder) build() (planningdom.Plan, error) {
	for i := range b.plan.Recipes {
		if b.recipeBaseServings[i] <= 0 || b.recipeServingTotals[i] <= 0 {
			return planningdom.Plan{}, errors.New("selected plan recipe has invalid serving totals")
		}
		scaleFactor := b.recipeServingTotals[i] / float64(b.recipeBaseServings[i])
		b.plan.Recipes[i].ScaleFactor = scaleFactor
		if b.plan.Recipes[i].ExpectedOutputG != nil {
			*b.plan.Recipes[i].ExpectedOutputG *= scaleFactor
		}
		for fillIndex := range b.plan.Recipes[i].Fills {
			b.plan.Recipes[i].Fills[fillIndex].PlannedInputQtyG *= scaleFactor
		}
	}
	for i := range b.allocations {
		draft := &b.allocations[i]
		component := &b.plan.Meals[draft.mealIndex].Components[draft.componentIndex]
		component.PortionFactor = draft.servingUnits / b.recipeServingTotals[draft.recipeIndex]
		if output := b.plan.Recipes[draft.recipeIndex].ExpectedOutputG; output != nil {
			servingG := *output * component.PortionFactor
			component.PlannedServingG = &servingG
		}
	}
	return b.plan, nil
}

func sameFills(left, right []planningdom.SlotFill) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i].SlotID != right[i].SlotID || left[i].ProductID != right[i].ProductID {
			return false
		}
	}
	return true
}

func cloneFloat(value *float64) *float64 {
	if value == nil {
		return nil
	}
	result := *value
	return &result
}

// DayRules returns the verdicts of the daily rules behind this evaluation.
func (q EaterDayQuality) DayRules() []nutrition.RuleEvaluationResult {
	return q.rules(false)
}

// RollingRules returns the verdicts of the weekly rules behind this evaluation.
func (q EaterDayQuality) RollingRules() []nutrition.RuleEvaluationResult {
	return q.rules(true)
}

func (q EaterDayQuality) rules(rolling bool) []nutrition.RuleEvaluationResult {
	results := make([]nutrition.RuleEvaluationResult, 0, len(q.Evaluation.Outcomes))
	for _, outcome := range q.Evaluation.Outcomes {
		if isRollingRule(outcome.Code) != rolling {
			continue
		}
		results = append(results, outcome.Result())
	}
	return results
}

// Warnings counts the outcomes the owner should read: soft failures and warns.
func (q EaterDayQuality) Warnings() int {
	count := 0
	for _, outcome := range q.Evaluation.Outcomes {
		if outcome.Status == nutrition.StatusSoftFail || outcome.Status == nutrition.StatusWarn {
			count++
		}
	}
	return count
}
