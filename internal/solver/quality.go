package solver

import (
	catalogdom "github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
	eaterdom "github.com/ZetoOfficial/meal-planner/internal/domain/eater"
	mealdom "github.com/ZetoOfficial/meal-planner/internal/domain/meal"
	"github.com/ZetoOfficial/meal-planner/internal/domain/nutrition"
	"github.com/ZetoOfficial/meal-planner/internal/quality"
)

const defaultRollingWindowDays = 7

type variantSlot struct {
	baseQtyG          float64
	preparedQtyFactor *float64
	macrosPer100G     nutrition.MacrosPer100G
	facts             catalogdom.NutritionFacts
	classification    catalogdom.FoodClassification
}

func newVariantSlot(slot SlotCandidates, product catalogdom.Product) variantSlot {
	return variantSlot{
		baseQtyG:          slot.Slot.QtyG,
		preparedQtyFactor: slot.Slot.PreparedQtyFactor,
		macrosPer100G: nutrition.MacrosPer100G{
			ProteinG: product.ProteinPer100G,
			FatG:     product.FatPer100G,
			CarbG:    product.CarbPer100G,
			Kcal:     product.KcalPer100G,
		},
		facts:          product.Nutrition,
		classification: product.Classification,
	}
}

// qualityComponent folds the serving scale into planned input grams so the
// portion factor stays inside the (0,1] range the quality package validates.
func (variant componentVariant) qualityComponent(servingUnits float64) quality.ComponentInput {
	scale := servingUnits / float64(variant.recipe.BaseServings)
	slots := make([]quality.SlotInput, 0, len(variant.slots))
	for i := range variant.slots {
		slot := &variant.slots[i]
		slots = append(slots, quality.SlotInput{
			PlannedInputQtyG:  slot.baseQtyG * scale,
			PreparedQtyFactor: slot.preparedQtyFactor,
			MacrosPer100G:     slot.macrosPer100G,
			Facts:             slot.facts,
			Classification:    slot.classification,
		})
	}
	return quality.ComponentInput{PortionFactor: 1, Slots: slots}
}

func (variant componentVariant) containsLegume() bool {
	for i := range variant.slots {
		legume := variant.slots[i].classification.IsLegume
		if legume != nil && *legume {
			return true
		}
	}
	return false
}

// dayEvaluator turns a candidate day solution into the same evaluation the API
// returns, so ranking and explanations cannot disagree.
type dayEvaluator struct {
	ruleset    quality.Ruleset
	sex        eaterdom.Sex
	windowDays int
	fixedMeals []quality.MealInput
	history    []nutrition.DaySummary
}

func newDayEvaluator(input Input, eaterInput EaterInput, history []nutrition.DaySummary) dayEvaluator {
	return dayEvaluator{
		ruleset:    input.ruleset(),
		sex:        eaterInput.Eater.Sex.OrUnspecified(),
		windowDays: input.rollingWindowDays(),
		fixedMeals: fixedQualityMeals(eaterInput.FixedMeals, eaterInput.Eater.ID),
		history:    history,
	}
}

func fixedQualityMeals(fixedMeals []eaterdom.FixedMeal, eaterID eaterdom.ID) []quality.MealInput {
	result := make([]quality.MealInput, 0, len(fixedMeals))
	for i := range fixedMeals {
		fixed := &fixedMeals[i]
		if !fixed.Active || fixed.EaterID != eaterID {
			continue
		}
		result = append(result, quality.MealInput{
			Type:  fixed.MealType,
			Fixed: &quality.FixedMealInput{Macros: fixed.Macros, Quality: fixed.Quality},
		})
	}
	return result
}

type dayEvaluation struct {
	day        nutrition.DaySummary
	rolling    nutrition.RollingSummary
	evaluation quality.Evaluation
	hard       []quality.HardViolation
}

func (e dayEvaluator) evaluate(solution daySolution, mealTypes []mealdom.Type) (dayEvaluation, error) {
	dayInput := e.dayInput(solution, mealTypes)
	if violations := quality.DetectHardViolations(dayInput); len(violations) > 0 {
		return dayEvaluation{hard: violations}, nil
	}

	day, err := quality.BuildDaySummary(dayInput, quality.SummaryPurposeWarnings)
	if err != nil {
		return dayEvaluation{}, err
	}
	rolling, err := quality.BuildRollingSummary(e.ruleset, e.windowDays, e.rollingDays(&day))
	if err != nil {
		return dayEvaluation{}, err
	}
	planInput := quality.PlanInput{DistinctLegumeComponents: distinctLegumeComponents(solution)}
	return dayEvaluation{
		day:        day,
		rolling:    rolling,
		evaluation: quality.Evaluate(e.ruleset, e.sex, day, &rolling, &planInput),
	}, nil
}

func (e dayEvaluator) dayInput(solution daySolution, mealTypes []mealdom.Type) quality.DayInput {
	meals := make([]quality.MealInput, 0, len(e.fixedMeals)+len(mealTypes))
	meals = append(meals, e.fixedMeals...)
	generated := make([]quality.MealInput, len(mealTypes))
	for i, mealType := range mealTypes {
		generated[i] = quality.MealInput{Type: mealType}
	}
	for i := range solution.uses {
		use := &solution.uses[i]
		generated[use.mealIndex].Components = append(
			generated[use.mealIndex].Components,
			use.variant.qualityComponent(use.servingUnits),
		)
	}
	meals = append(meals, generated...)
	return quality.DayInput{Sex: e.sex, Meals: meals}
}

// rollingDays keeps the candidate day last so the projection always includes it
// even when the stored history already fills the window.
func (e dayEvaluator) rollingDays(candidate *nutrition.DaySummary) []*nutrition.DaySummary {
	history := e.history
	if len(history) > e.windowDays-1 {
		history = history[len(history)-(e.windowDays-1):]
	}
	days := make([]*nutrition.DaySummary, 0, len(history)+1)
	for i := range history {
		days = append(days, &history[i])
	}
	return append(days, candidate)
}

func distinctLegumeComponents(solution daySolution) int {
	seen := make(map[int]struct{}, len(solution.uses))
	count := 0
	for i := range solution.uses {
		use := &solution.uses[i]
		if !use.variant.containsLegume() {
			continue
		}
		if _, exists := seen[int(use.variant.recipe.ID)]; exists {
			continue
		}
		seen[int(use.variant.recipe.ID)] = struct{}{}
		count++
	}
	return count
}

func (i Input) ruleset() quality.Ruleset {
	if i.Ruleset != nil {
		return *i.Ruleset
	}
	return quality.DefaultRuleset()
}

func (i Input) rollingWindowDays() int {
	if i.RollingWindowDays > 0 {
		return i.RollingWindowDays
	}
	return defaultRollingWindowDays
}
