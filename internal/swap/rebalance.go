package swap

import (
	"math"
	"strconv"
	"time"

	eaterdom "github.com/ZetoOfficial/meal-planner/internal/domain/eater"
	mealdom "github.com/ZetoOfficial/meal-planner/internal/domain/meal"
	"github.com/ZetoOfficial/meal-planner/internal/domain/nutrition"
	planningdom "github.com/ZetoOfficial/meal-planner/internal/domain/planning"
	"github.com/ZetoOfficial/meal-planner/internal/solver"
)

// rebalance finds the batch shares that keep every eater-day inside its macro
// range at the locked quantity. Only the affected batch moves, and each
// eater-day is solved on its own, so the blast radius stays as small as the
// change allows. It returns the shares keyed by meal; meals whose share does
// not move are still included so the caller can compare.
func rebalance(
	input Input,
	plan planningdom.Plan,
	recipeIndex int,
	goals []GoalTarget,
) (map[planningdom.MealID]float64, error) {
	batch, err := batchMacros(plan.Recipes[recipeIndex], input.Catalogue)
	if err != nil {
		return nil, err
	}
	ranges := goalIndex(goals)

	portions := make(map[planningdom.MealID]float64)
	for _, day := range eaterDays(plan) {
		target, hasTarget := ranges[dayKey(day.eaterID, day.date)]
		base, err := macrosOutsideBatch(plan, day, recipeIndex, input)
		if err != nil {
			return nil, err
		}
		current, mealID, found := currentPortion(plan, day, recipeIndex)
		if !found {
			continue
		}
		if !hasTarget {
			portions[mealID] = current
			continue
		}

		chosen, ok := choosePortion(base, batch, target, current)
		if !ok {
			return nil, &InfeasibleQuantityError{
				Constraints: unreachableAtAnyPortion(base, batch, target),
				EaterID:     day.eaterID,
				Date:        day.date,
			}
		}
		portions[mealID] = chosen
	}

	if err := checkBatchCapacity(portions); err != nil {
		return nil, err
	}
	return portions, nil
}

// choosePortion prefers the share closest to the current one, so a quantity
// change never reshuffles the plan more than it has to.
func choosePortion(base, batch nutrition.Macros, target eaterdom.MacroRange, current float64) (float64, bool) {
	best := 0.0
	found := false
	for portion := minimumPortion; portion <= 1+portionEpsilon; portion += portionStep {
		share, err := batch.Scale(portion)
		if err != nil {
			continue
		}
		total, err := base.Add(share)
		if err != nil {
			continue
		}
		if !target.Contains(total) {
			continue
		}
		if !found || math.Abs(portion-current) < math.Abs(best-current)-portionEpsilon {
			best = portion
			found = true
		}
	}
	return round2(best), found
}

// unreachableAtAnyPortion reports the bounds that stay unmet across the whole
// portion grid, which is what makes the request provably impossible.
func unreachableAtAnyPortion(base, batch nutrition.Macros, target eaterdom.MacroRange) []solver.MacroConstraint {
	minimum, err := batch.Scale(minimumPortion)
	if err != nil {
		return nil
	}
	maximum := batch

	floor, err := base.Add(minimum)
	if err != nil {
		return nil
	}
	ceiling, err := base.Add(maximum)
	if err != nil {
		return nil
	}
	return solver.UnreachableMacros(target, floor, ceiling)
}

func checkBatchCapacity(portions map[planningdom.MealID]float64) error {
	total := 0.0
	for _, portion := range portions {
		total += portion
	}
	if total > 1+portionEpsilon {
		return &InfeasibleQuantityError{}
	}
	return nil
}

type planDayKey struct {
	eaterID eaterdom.ID
	date    time.Time
}

func eaterDays(plan planningdom.Plan) []planDayKey {
	days := make([]planDayKey, 0, len(plan.EaterDays))
	for i := range plan.EaterDays {
		days = append(days, planDayKey{
			eaterID: plan.EaterDays[i].EaterID,
			date:    mealdom.DayOf(plan.EaterDays[i].Date),
		})
	}
	return days
}

// macrosOutsideBatch sums everything the eater-day gets that the adjusted batch
// does not supply: fixed meals and every other batch share.
func macrosOutsideBatch(
	plan planningdom.Plan,
	day planDayKey,
	recipeIndex int,
	input Input,
) (nutrition.Macros, error) {
	total := nutrition.Macros{}
	for i := range plan.Meals {
		meal := &plan.Meals[i]
		if meal.EaterID != day.eaterID || !mealdom.DayOf(meal.Date).Equal(day.date) {
			continue
		}
		if meal.FixedMealID != nil {
			var err error
			if total, err = total.Add(meal.Planned); err != nil {
				return nutrition.Macros{}, err
			}
			continue
		}
		for _, component := range meal.Components {
			if component.PlanRecipeIndex == recipeIndex {
				continue
			}
			batch, err := batchMacros(plan.Recipes[component.PlanRecipeIndex], input.Catalogue)
			if err != nil {
				return nutrition.Macros{}, err
			}
			share, err := batch.Scale(component.PortionFactor)
			if err != nil {
				return nutrition.Macros{}, err
			}
			if total, err = total.Add(share); err != nil {
				return nutrition.Macros{}, err
			}
		}
	}
	return total, nil
}

func currentPortion(
	plan planningdom.Plan,
	day planDayKey,
	recipeIndex int,
) (portion float64, mealID planningdom.MealID, found bool) {
	for i := range plan.Meals {
		meal := &plan.Meals[i]
		if meal.EaterID != day.eaterID || !mealdom.DayOf(meal.Date).Equal(day.date) {
			continue
		}
		for _, component := range meal.Components {
			if component.PlanRecipeIndex == recipeIndex {
				return component.PortionFactor, meal.ID, true
			}
		}
	}
	return 0, 0, false
}

func goalIndex(goals []GoalTarget) map[string]eaterdom.MacroRange {
	result := make(map[string]eaterdom.MacroRange, len(goals))
	for _, goal := range goals {
		result[dayKey(goal.EaterID, goal.Date)] = goal.Range
	}
	return result
}

func dayKey(eaterID eaterdom.ID, date time.Time) string {
	return mealdom.DayOf(date).Format(time.DateOnly) + "#" + strconv.FormatInt(int64(eaterID), 10)
}

// round2 keeps portion factors at the two decimals the database stores.
func round2(value float64) float64 {
	return math.Round(value*percentScale) / percentScale
}
