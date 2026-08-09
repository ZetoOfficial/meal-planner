package solver

import (
	"errors"
	"time"

	eaterdom "github.com/ZetoOfficial/meal-planner/internal/domain/eater"
	"github.com/ZetoOfficial/meal-planner/internal/domain/nutrition"
	planningdom "github.com/ZetoOfficial/meal-planner/internal/domain/planning"
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
}

type Result struct {
	Plan    planningdom.Plan
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
