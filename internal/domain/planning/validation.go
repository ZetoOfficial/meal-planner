package planning

import (
	"errors"
	"fmt"
	"math"
	"time"

	eaterdom "github.com/ZetoOfficial/meal-planner/internal/domain/eater"
	mealdom "github.com/ZetoOfficial/meal-planner/internal/domain/meal"
)

// portionSumTolerance absorbs the rounding of repeated portion divisions so an
// exactly allocated batch is not rejected as over-allocated.
const portionSumTolerance = 1e-6

// Validate enforces the cross-row plan invariants the database documents but
// cannot express: allocations belong to the plan, generated meals are backed by
// at least one batch share, fixed meals are not, and no batch is over-served.
func (p Plan) Validate() error {
	if p.Days <= 0 || p.StartDate.IsZero() || p.CookDate.IsZero() {
		return errors.New("plan horizon is incomplete")
	}
	if p.Novelty < 0 || p.Novelty > maxNovelty {
		return fmt.Errorf("plan novelty %d is outside 0..%d", p.Novelty, maxNovelty)
	}
	if p.Status != "" && !p.Status.Valid() {
		return fmt.Errorf("plan status %q is not a known status", p.Status)
	}
	if len(p.EaterDays) == 0 {
		return errors.New("plan has no eater days")
	}
	if err := p.validateRecipes(); err != nil {
		return err
	}
	return p.validateMeals()
}

const maxNovelty = 3

func (p Plan) validateRecipes() error {
	for i := range p.Recipes {
		recipe := &p.Recipes[i]
		if !recipe.TemplateID.Valid() || !positive(recipe.ScaleFactor) {
			return fmt.Errorf("plan recipe %d has no template or a non-positive scale factor", i)
		}
		if recipe.ExpectedOutputG != nil && !positive(*recipe.ExpectedOutputG) {
			return fmt.Errorf("plan recipe %d has a non-positive expected output", i)
		}
		if len(recipe.Fills) == 0 {
			return fmt.Errorf("plan recipe %d has no slot fills", i)
		}
		for _, fill := range recipe.Fills {
			if !fill.ProductID.Valid() || !positive(fill.PlannedInputQtyG) {
				return fmt.Errorf("plan recipe %d has an incomplete slot fill", i)
			}
		}
	}
	return nil
}

func (p Plan) validateMeals() error {
	days := p.eaterDayIndex()
	portions := make([]float64, len(p.Recipes))
	for i := range p.Meals {
		meal := &p.Meals[i]
		if err := p.validateMeal(meal, days, portions); err != nil {
			return err
		}
	}
	for i, portion := range portions {
		if portion > 1+portionSumTolerance {
			return fmt.Errorf("plan recipe %d is allocated %.4f of its batch", i, portion)
		}
	}
	return nil
}

func (p Plan) validateMeal(meal *Meal, days map[eaterDayKey][]mealdom.Type, portions []float64) error {
	types, ok := days[eaterDayKey{eaterID: meal.EaterID, date: mealdom.DayOf(meal.Date)}]
	if !ok {
		return fmt.Errorf("meal for eater %d on %s has no plan eater day", meal.EaterID, meal.Date.Format(time.DateOnly))
	}
	if !meal.Planned.Valid() {
		return fmt.Errorf("meal for eater %d on %s has invalid planned macros", meal.EaterID, meal.Date.Format(time.DateOnly))
	}
	if meal.LogStatus != "" && !meal.LogStatus.Valid() {
		return fmt.Errorf("meal log status %q is not a known status", meal.LogStatus)
	}

	if meal.FixedMealID != nil {
		if len(meal.Components) > 0 {
			return fmt.Errorf("fixed meal for eater %d must not carry component allocations", meal.EaterID)
		}
		return nil
	}
	if len(meal.Components) == 0 {
		return fmt.Errorf("generated %s for eater %d has no component allocation", meal.Type, meal.EaterID)
	}
	if !containsMealType(types, meal.Type) {
		return fmt.Errorf("generated %s for eater %d is not a planned meal type", meal.Type, meal.EaterID)
	}
	return addComponents(meal, portions)
}

func addComponents(meal *Meal, portions []float64) error {
	seen := make(map[int]struct{}, len(meal.Components))
	for _, component := range meal.Components {
		if component.PlanRecipeIndex < 0 || component.PlanRecipeIndex >= len(portions) {
			return fmt.Errorf("component allocation points to plan recipe %d outside the plan", component.PlanRecipeIndex)
		}
		if _, exists := seen[component.PlanRecipeIndex]; exists {
			return fmt.Errorf("meal allocates plan recipe %d twice", component.PlanRecipeIndex)
		}
		seen[component.PlanRecipeIndex] = struct{}{}
		if !positive(component.PortionFactor) || component.PortionFactor > 1 {
			return fmt.Errorf("portion factor %v is outside (0,1]", component.PortionFactor)
		}
		if component.PlannedServingG != nil && !positive(*component.PlannedServingG) {
			return errors.New("planned serving grams must be positive when known")
		}
		portions[component.PlanRecipeIndex] += component.PortionFactor
	}
	return nil
}

type eaterDayKey struct {
	eaterID eaterdom.ID
	date    time.Time
}

func (p Plan) eaterDayIndex() map[eaterDayKey][]mealdom.Type {
	result := make(map[eaterDayKey][]mealdom.Type, len(p.EaterDays))
	for i := range p.EaterDays {
		day := &p.EaterDays[i]
		result[eaterDayKey{eaterID: day.EaterID, date: mealdom.DayOf(day.Date)}] = day.GeneratedMealTypes
	}
	return result
}

func containsMealType(types []mealdom.Type, mealType mealdom.Type) bool {
	for _, candidate := range types {
		if candidate == mealType {
			return true
		}
	}
	return false
}

func positive(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value > 0
}
