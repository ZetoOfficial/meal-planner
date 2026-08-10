package quality

import (
	"errors"
	"math"

	"github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
	"github.com/ZetoOfficial/meal-planner/internal/domain/eater"
	mealdom "github.com/ZetoOfficial/meal-planner/internal/domain/meal"
	"github.com/ZetoOfficial/meal-planner/internal/domain/nutrition"
)

const per100g = 100.0

type DayInput struct {
	Sex   eater.Sex
	Meals []MealInput
}

type MealInput struct {
	Type       mealdom.Type
	Fixed      *FixedMealInput
	Components []ComponentInput
}

type FixedMealInput struct {
	Macros  nutrition.Macros
	Quality eater.FixedMealQuality
}

type ComponentInput struct {
	PortionFactor float64
	Slots         []SlotInput
}

type SlotInput struct {
	PlannedInputQtyG  float64
	PreparedQtyFactor *float64
	MacrosPer100G     nutrition.MacrosPer100G
	Facts             catalog.NutritionFacts
	Classification    catalog.FoodClassification
}

type SummaryPurpose uint8

const (
	SummaryPurposeAnalytics SummaryPurpose = iota + 1
	SummaryPurposeWarnings
)

func BuildDaySummary(input DayInput, purpose SummaryPurpose) (nutrition.DaySummary, error) {
	if !input.Sex.Valid() {
		return nutrition.DaySummary{}, errors.New("invalid eater sex")
	}
	if purpose != SummaryPurposeAnalytics && purpose != SummaryPurposeWarnings {
		return nutrition.DaySummary{}, errors.New("invalid summary purpose")
	}

	result := nutrition.DaySummary{
		Basis:        nutrition.SummaryBasisPlanned,
		RuleCoverage: make(map[string]nutrition.Coverage),
	}

	for _, meal := range input.Meals {
		if err := addMealToSummary(&result, meal, purpose); err != nil {
			return nutrition.DaySummary{}, err
		}
	}

	result.Coverage = aggregateCoverage(result.RuleCoverage)
	return result, nil
}

func addMealToSummary(summary *nutrition.DaySummary, meal MealInput, purpose SummaryPurpose) error {
	if meal.Fixed != nil {
		return addFixedMeal(summary, *meal.Fixed, purpose)
	}
	for _, comp := range meal.Components {
		if err := addComponent(summary, comp); err != nil {
			return err
		}
	}
	return nil
}

func addFixedMeal(summary *nutrition.DaySummary, fixed FixedMealInput, purpose SummaryPurpose) error {
	m := fixed.Macros
	if !m.Valid() {
		return errors.New("fixed meal macros must be finite and nonnegative")
	}

	updated, err := summary.Macros.Add(m)
	if err != nil {
		return err
	}
	summary.Macros = updated

	q := fixed.Quality
	includeQuality := purpose == SummaryPurposeAnalytics && q.IncludeInAnalytics ||
		purpose == SummaryPurposeWarnings && q.IncludeInWarnings
	if !includeQuality {
		return nil
	}

	kcalWeight := float64(m.Kcal)

	addFixedNutrient(summary, RuleSatFat, q.SaturatedFatG, kcalWeight, &summary.Totals.SaturatedFatG)
	addFixedNutrient(summary, RuleFiber, q.FiberG, kcalWeight, &summary.Totals.FiberG)

	addCoverageKnownWeight(summary, RuleIndustrialTransFat, false, kcalWeight)
	if q.TransFatG != nil {
		summary.Totals.TransFatG += *q.TransFatG
	}
	addFixedNutrient(summary, RuleSodium, q.SodiumMg, kcalWeight, &summary.Totals.SodiumMg)
	addFixedNutrient(summary, RuleAddedSugar, q.AddedSugarsG, kcalWeight, &summary.Totals.AddedSugarsG)

	addFixedFoodGroup(summary, RuleFruitVeg, q.FruitVegG, kcalWeight, &summary.Totals.FruitVegG)

	wgKnown := q.GrainGEquiv != nil && q.WholeGrainGEquiv != nil
	addCoverageKnownWeight(summary, RuleWholeGrain, wgKnown, kcalWeight)
	if wgKnown {
		summary.Totals.GrainGEquiv += *q.GrainGEquiv
		summary.Totals.WholeGrainGEquiv += *q.WholeGrainGEquiv
	}

	addFixedFoodGroup(summary, RuleLegumes, q.LegumeG, kcalWeight, &summary.Totals.LegumeG)
	addFixedFoodGroup(summary, RuleRedMeat, q.RedMeatG, kcalWeight, &summary.Totals.RedMeatG)

	pmKnown := q.ProcessedMeatG != nil
	addCoverageKnownWeight(summary, RuleProcessedMeat, pmKnown, kcalWeight)
	if pmKnown {
		summary.Totals.ProcessedMeatG += *q.ProcessedMeatG
		summary.Totals.FixedProcessedMeatG += *q.ProcessedMeatG
	}

	upfKnown := q.UPFKcal != nil
	addCoverageKnownWeight(summary, RuleUPF, upfKnown, kcalWeight)
	if upfKnown {
		if summary.Shares.UPFKcal == nil {
			summary.Shares.UPFKcal = float64Pointer(0)
		}
		*summary.Shares.UPFKcal += *q.UPFKcal
	}

	return nil
}

func addFixedNutrient(summary *nutrition.DaySummary, rule string, value *float64, weight float64, total *float64) {
	addCoverageKnownWeight(summary, rule, value != nil, weight)
	if value != nil {
		*total += *value
	}
}

func addFixedFoodGroup(summary *nutrition.DaySummary, rule string, value *float64, weight float64, total *float64) {
	addCoverageKnownWeight(summary, rule, value != nil, weight)
	if value != nil {
		*total += *value
	}
}

func addCoverageKnownWeight(summary *nutrition.DaySummary, rule string, known bool, weight float64) {
	addCoverageSlot(summary, rule, known, weight)
}

func addComponent(summary *nutrition.DaySummary, comp ComponentInput) error {
	if math.IsNaN(comp.PortionFactor) || math.IsInf(comp.PortionFactor, 0) ||
		comp.PortionFactor <= 0 || comp.PortionFactor > 1 {
		return errors.New("portion factor must be in (0,1]")
	}

	for i := range comp.Slots {
		if err := addSlot(summary, comp.PortionFactor, &comp.Slots[i]); err != nil {
			return err
		}
	}
	return nil
}

func addSlot(summary *nutrition.DaySummary, portionFactor float64, slot *SlotInput) error {
	if math.IsNaN(slot.PlannedInputQtyG) || math.IsInf(slot.PlannedInputQtyG, 0) ||
		slot.PlannedInputQtyG <= 0 {
		return errors.New("planned input quantity must be finite and positive")
	}
	if !slot.MacrosPer100G.Valid() {
		return errors.New("slot macros per 100 g must be finite and nonnegative")
	}

	allocatedG := slot.PlannedInputQtyG * portionFactor

	macros, err := slot.MacrosPer100G.Scale(allocatedG)
	if err != nil {
		return err
	}
	updated, err := summary.Macros.Add(macros)
	if err != nil {
		return err
	}
	summary.Macros = updated

	facts := slot.Facts
	addNutrient(summary, RuleFiber, facts.FiberPer100G, allocatedG)
	addNutrient(summary, RuleSodium, facts.EffectiveSodiumMgPer100G(), allocatedG)
	addNutrient(summary, RuleSatFat, facts.SaturatedFatPer100G, allocatedG)
	addNutrient(summary, RuleAddedSugar, facts.AddedSugarsPer100G, allocatedG)

	addTransFat(summary, facts, allocatedG)
	addUPF(summary, macros, slot.Classification)
	addFoodGroups(summary, allocatedG, slot.PreparedQtyFactor, slot.Classification)

	return nil
}

func addTransFat(summary *nutrition.DaySummary, facts catalog.NutritionFacts, allocatedG float64) {
	if facts.TransFatPer100G != nil {
		summary.Totals.TransFatG += *facts.TransFatPer100G * allocatedG / per100g
	}
	addCoverageSlot(summary, RuleIndustrialTransFat, industrialTransFatKnown(facts), allocatedG)
}

func addUPF(summary *nutrition.DaySummary, macros nutrition.Macros, class catalog.FoodClassification) {
	upfKcal := 0.0
	upfKnown := class.ProcessingLevel.Known()
	if upfKnown && class.ProcessingLevel == catalog.ProcessingLevelUltraProcessed {
		upfKcal = float64(macros.Kcal)
	}
	if upfKnown {
		if summary.Shares.UPFKcal == nil {
			summary.Shares.UPFKcal = float64Pointer(0)
		}
		*summary.Shares.UPFKcal += upfKcal
	}
	addCoverageSlot(summary, RuleUPF, upfKnown, float64(macros.Kcal))
}

func addFoodGroups(summary *nutrition.DaySummary, allocatedG float64, factor *float64, class catalog.FoodClassification) {
	preparedG, preparedKnown := preparedGrams(allocatedG, factor)

	addFruitVeg(summary, preparedG, preparedKnown, class)
	addGrain(summary, preparedG, preparedKnown, class)
	addLegume(summary, preparedG, preparedKnown, class)
	addRedMeat(summary, preparedG, preparedKnown, class)
	addProcessedMeat(summary, preparedG, preparedKnown, class)
}

func addFruitVeg(summary *nutrition.DaySummary, preparedG float64, preparedKnown bool, class catalog.FoodClassification) {
	fvKnown := countsToFruitVegKnown(class) && preparedKnown
	addCoverageSlot(summary, RuleFruitVeg, fvKnown, preparedG)
	if fvKnown && class.CountsToFruitVeg != nil && *class.CountsToFruitVeg {
		summary.Totals.FruitVegG += preparedG
	}
}

func addGrain(summary *nutrition.DaySummary, preparedG float64, preparedKnown bool, class catalog.FoodClassification) {
	gKnown := grainStatusKnown(class) && preparedKnown
	addCoverageSlot(summary, RuleWholeGrain, gKnown, preparedG)
	if gKnown {
		summary.Totals.GrainGEquiv += preparedG
		summary.Totals.WholeGrainGEquiv += wholeGrainGrams(preparedG, class)
	}
}

func addLegume(summary *nutrition.DaySummary, preparedG float64, preparedKnown bool, class catalog.FoodClassification) {
	lKnown := class.IsLegume != nil && preparedKnown
	addCoverageSlot(summary, RuleLegumes, lKnown, preparedG)
	if lKnown && *class.IsLegume {
		summary.Totals.LegumeG += preparedG
	}
}

func addRedMeat(summary *nutrition.DaySummary, preparedG float64, preparedKnown bool, class catalog.FoodClassification) {
	rmKnown := class.IsRedMeat != nil && preparedKnown
	addCoverageSlot(summary, RuleRedMeat, rmKnown, preparedG)
	if rmKnown && *class.IsRedMeat {
		summary.Totals.RedMeatG += preparedG
	}
}

func addProcessedMeat(summary *nutrition.DaySummary, preparedG float64, preparedKnown bool, class catalog.FoodClassification) {
	pmKnown := class.IsProcessedMeat != nil && preparedKnown
	addCoverageSlot(summary, RuleProcessedMeat, pmKnown, preparedG)
	if pmKnown && *class.IsProcessedMeat {
		summary.Totals.ProcessedMeatG += preparedG
		summary.Totals.GeneratedProcessedMeatG += preparedG
	}
}

func addNutrient(summary *nutrition.DaySummary, rule string, per100gValue *float64, allocatedG float64) {
	known := per100gValue != nil
	if known {
		addValue(summary, rule, *per100gValue*allocatedG/per100g)
	}
	addCoverageSlot(summary, rule, known, allocatedG)
}

func addValue(summary *nutrition.DaySummary, rule string, value float64) {
	switch rule {
	case RuleFiber:
		summary.Totals.FiberG += value
	case RuleSodium:
		summary.Totals.SodiumMg += value
	case RuleSatFat:
		summary.Totals.SaturatedFatG += value
	case RuleAddedSugar:
		summary.Totals.AddedSugarsG += value
	case RuleIndustrialTransFat:
		summary.Totals.TransFatG += value
	}
}

func addCoverageSlot(summary *nutrition.DaySummary, rule string, known bool, weight float64) {
	if math.IsNaN(weight) || math.IsInf(weight, 0) || weight < 0 {
		return
	}
	cov, err := nutrition.CoverageForInput(known, weight)
	if err != nil {
		return
	}
	summary.RuleCoverage[rule] = summary.RuleCoverage[rule].Add(cov)
}

func preparedGrams(allocatedG float64, factor *float64) (float64, bool) {
	if factor == nil {
		return allocatedG, false
	}
	if math.IsNaN(*factor) || math.IsInf(*factor, 0) || *factor <= 0 {
		return allocatedG, false
	}
	return allocatedG * *factor, true
}

func countsToFruitVegKnown(class catalog.FoodClassification) bool {
	return class.CountsToFruitVeg != nil
}

func grainStatusKnown(class catalog.FoodClassification) bool {
	return class.IsWholeGrain != nil || class.WholeGrainRatio != nil
}

func industrialTransFatKnown(facts catalog.NutritionFacts) bool {
	return facts.ContainsPartiallyHydrogenatedOils != nil && facts.IndustrialTransFatPer100G != nil
}

func wholeGrainGrams(preparedG float64, class catalog.FoodClassification) float64 {
	if class.WholeGrainRatio != nil {
		return preparedG * *class.WholeGrainRatio
	}
	if class.IsWholeGrain != nil && *class.IsWholeGrain {
		return preparedG
	}
	return 0
}

func aggregateCoverage(coverages map[string]nutrition.Coverage) nutrition.Coverage {
	var sumScores float64
	for _, cov := range coverages {
		sumScores += cov.Score()
	}
	if len(coverages) == 0 {
		return nutrition.Coverage{}
	}
	avg := sumScores / float64(len(coverages))
	known, _ := nutrition.CoverageForInput(true, avg)
	unknown, _ := nutrition.CoverageForInput(false, 1-avg)
	return known.Add(unknown)
}

func float64Pointer(v float64) *float64 { return &v }
