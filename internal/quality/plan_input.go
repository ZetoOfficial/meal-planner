package quality

import (
	"fmt"
	"time"

	catalogdom "github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
	eaterdom "github.com/ZetoOfficial/meal-planner/internal/domain/eater"
	"github.com/ZetoOfficial/meal-planner/internal/domain/nutrition"
	planningdom "github.com/ZetoOfficial/meal-planner/internal/domain/planning"
	recipedom "github.com/ZetoOfficial/meal-planner/internal/domain/recipe"
)

// Catalogue is everything a stored plan needs to be re-read as quality input.
// Quality owns this assembly because the inputs are SlotFill × Product ×
// MealComponentAllocation plus fixed meals (ARCHITECTURE §5.6).
type Catalogue struct {
	Products   map[catalogdom.ProductID]catalogdom.Product
	Templates  map[recipedom.ID]recipedom.Template
	Eaters     map[eaterdom.ID]eaterdom.Eater
	FixedMeals map[eaterdom.FixedMealID]eaterdom.FixedMeal
}

// PlanDay is one eater-day of a stored plan, ready to be summarised.
type PlanDay struct {
	EaterID eaterdom.ID
	Date    time.Time
	Input   DayInput
}

// BuildPlanDays rebuilds the planned quality input of every eater-day.
func BuildPlanDays(plan planningdom.Plan, catalogue Catalogue) ([]PlanDay, error) {
	return BuildPlanDaysForBasis(plan, catalogue, nutrition.SummaryBasisPlanned)
}

// BuildPlanDaysForBasis rebuilds the quality input of every eater-day in a
// stored plan. Days keep the plan's own order so before/after comparisons line
// up. On the actual basis a skipped meal contributes nothing and a modified one
// contributes its logged macros with unknown quality.
func BuildPlanDaysForBasis(
	plan planningdom.Plan,
	catalogue Catalogue,
	basis nutrition.SummaryBasis,
) ([]PlanDay, error) {
	days := make([]PlanDay, 0, len(plan.EaterDays))
	positions := make(map[string]int, len(plan.EaterDays))
	for i := range plan.EaterDays {
		day := &plan.EaterDays[i]
		eater, ok := catalogue.Eaters[day.EaterID]
		if !ok {
			return nil, fmt.Errorf("plan references unknown eater %d", day.EaterID)
		}
		positions[dayKey(day.EaterID, day.Date)] = len(days)
		days = append(days, PlanDay{
			EaterID: day.EaterID,
			Date:    dayOf(day.Date),
			Input:   DayInput{Sex: sexOrUnspecified(eater.Sex)},
		})
	}

	for i := range plan.Meals {
		meal := &plan.Meals[i]
		position, ok := positions[dayKey(meal.EaterID, meal.Date)]
		if !ok {
			return nil, fmt.Errorf("plan meal for eater %d has no eater day", meal.EaterID)
		}
		if basis == nutrition.SummaryBasisActual && !meal.Eaten() {
			continue
		}
		if basis == nutrition.SummaryBasisActual && !meal.QualityKnown() {
			days[position].Input.Meals = append(days[position].Input.Meals, MealInput{
				Type:    meal.Type,
				Unknown: &UnknownMealInput{Macros: meal.EffectiveMacros()},
			})
			continue
		}
		mealInput, err := mealQualityInput(plan, *meal, catalogue)
		if err != nil {
			return nil, err
		}
		days[position].Input.Meals = append(days[position].Input.Meals, mealInput)
	}
	return days, nil
}

func mealQualityInput(plan planningdom.Plan, meal planningdom.Meal, catalogue Catalogue) (MealInput, error) {
	if meal.FixedMealID != nil {
		fixed, ok := catalogue.FixedMeals[*meal.FixedMealID]
		if !ok {
			// A deleted fixed meal still has its planned macros on the meal row;
			// quality data is simply unknown for it.
			return MealInput{
				Type:  meal.Type,
				Fixed: &FixedMealInput{Macros: meal.Planned},
			}, nil
		}
		return MealInput{
			Type:  meal.Type,
			Fixed: &FixedMealInput{Macros: fixed.Macros, Quality: fixed.Quality},
		}, nil
	}

	input := MealInput{Type: meal.Type, Components: make([]ComponentInput, 0, len(meal.Components))}
	for _, component := range meal.Components {
		if component.PlanRecipeIndex < 0 || component.PlanRecipeIndex >= len(plan.Recipes) {
			return MealInput{}, fmt.Errorf("meal allocation points to plan recipe %d outside the plan", component.PlanRecipeIndex)
		}
		recipe := plan.Recipes[component.PlanRecipeIndex]
		slots, err := slotQualityInputs(recipe, catalogue)
		if err != nil {
			return MealInput{}, err
		}
		input.Components = append(input.Components, ComponentInput{
			PortionFactor: component.PortionFactor,
			Slots:         slots,
		})
	}
	return input, nil
}

func slotQualityInputs(recipe planningdom.PlanRecipe, catalogue Catalogue) ([]SlotInput, error) {
	template, hasTemplate := catalogue.Templates[recipe.TemplateID]
	factors := make(map[recipedom.SlotID]*float64, len(template.Slots))
	if hasTemplate {
		for _, slot := range template.Slots {
			factors[slot.ID] = slot.PreparedQtyFactor
		}
	}

	slots := make([]SlotInput, 0, len(recipe.Fills))
	for _, fill := range recipe.Fills {
		product, ok := catalogue.Products[fill.ProductID]
		if !ok {
			return nil, fmt.Errorf("plan references unknown product %d", fill.ProductID)
		}
		slots = append(slots, SlotInput{
			PlannedInputQtyG:  fill.PlannedInputQtyG,
			PreparedQtyFactor: factors[fill.SlotID],
			MacrosPer100G: nutrition.MacrosPer100G{
				ProteinG: product.ProteinPer100G,
				FatG:     product.FatPer100G,
				CarbG:    product.CarbPer100G,
				Kcal:     product.KcalPer100G,
			},
			Facts:          product.Nutrition,
			Classification: product.Classification,
		})
	}
	return slots, nil
}

func dayKey(eaterID eaterdom.ID, date time.Time) string {
	return fmt.Sprintf("%d@%s", eaterID, dayOf(date).Format(time.DateOnly))
}

const dayDuration = 24 * time.Hour

func dayOf(date time.Time) time.Time {
	return date.UTC().Truncate(dayDuration)
}

func sexOrUnspecified(sex eaterdom.Sex) eaterdom.Sex {
	if sex.Valid() {
		return sex
	}
	return eaterdom.SexUnspecified
}
