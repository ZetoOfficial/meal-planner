package solver

import (
	"context"
	"errors"
	"fmt"
	"math"

	catalogdom "github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
	eaterdom "github.com/ZetoOfficial/meal-planner/internal/domain/eater"
	mealdom "github.com/ZetoOfficial/meal-planner/internal/domain/meal"
	"github.com/ZetoOfficial/meal-planner/internal/domain/nutrition"
	planningdom "github.com/ZetoOfficial/meal-planner/internal/domain/planning"
	recipedom "github.com/ZetoOfficial/meal-planner/internal/domain/recipe"
	"github.com/ZetoOfficial/meal-planner/internal/quality"
)

type searchState uint8

const gramsPer100 = 100

const (
	searchExhausted searchState = iota
	searchFound
	searchLimited
)

type componentVariant struct {
	recipe     recipedom.Template
	baseFills  []planningdom.SlotFill
	baseMacros nutrition.Macros
	slots      []variantSlot
}

type searchVariable struct {
	mealIndex int
	variant   componentVariant
}

type componentUse struct {
	mealIndex    int
	variant      componentVariant
	servingUnits float64
}

type daySolution struct {
	mealMacros []nutrition.Macros
	uses       []componentUse
}

type daySearch struct {
	config             Config
	target             eaterdom.MacroRange
	variables          []searchVariable
	suffixMaximums     []nutrition.Macros
	chosen             []float64
	mealComponentCount []int
	visited            int
	limited            bool

	evaluator dayEvaluator
	ruleset   quality.Ruleset
	mealTypes []mealdom.Type
	priorUses map[int64]int
	accepted  int
	best      *rankedDay
	bestScore dayScore
}

// rankedDay carries the winning candidate together with the evaluation that
// won it, so callers explain exactly what the objective compared.
type rankedDay struct {
	solution   daySolution
	day        nutrition.DaySummary
	rolling    nutrition.RollingSummary
	evaluation quality.Evaluation
}

// errSearchBudget and errCandidateBudget unwind the recursion without turning a
// bounded search into an application error.
var (
	errSearchBudget    = errors.New("solver search budget reached")
	errCandidateBudget = errors.New("solver candidate budget reached")
)

func searchDay(
	ctx context.Context,
	config Config,
	evaluator dayEvaluator,
	day EaterDayInput,
	fixedMacros nutrition.Macros,
	recipes []RecipeCandidate,
	priorUses map[int64]int,
) (rankedDay, searchState, error) {
	variables, err := searchVariables(day.GeneratedMealTypes, recipes)
	if err != nil {
		return rankedDay{}, searchExhausted, err
	}

	suffixMaximums, err := suffixMaximums(variables, config.ServingMultipliers)
	if err != nil {
		return rankedDay{}, searchExhausted, err
	}
	search := daySearch{
		config:             config,
		target:             day.Goal.Range,
		variables:          variables,
		suffixMaximums:     suffixMaximums,
		chosen:             make([]float64, len(variables)),
		mealComponentCount: make([]int, len(day.GeneratedMealTypes)),
		evaluator:          evaluator,
		ruleset:            evaluator.ruleset,
		mealTypes:          day.GeneratedMealTypes,
		priorUses:          priorUses,
	}

	switch err := search.walk(ctx, 0, fixedMacros); {
	case errors.Is(err, errSearchBudget), errors.Is(err, errCandidateBudget):
	case err != nil:
		return rankedDay{}, searchExhausted, err
	}

	if search.best == nil {
		if search.limited {
			return rankedDay{}, searchLimited, nil
		}
		return rankedDay{}, searchExhausted, nil
	}
	return *search.best, searchFound, nil
}

func (search *daySearch) walk(ctx context.Context, index int, total nutrition.Macros) error {
	done, err := search.enter(ctx, index, total)
	if done || err != nil {
		return err
	}
	if err := search.walk(ctx, index+1, total); err != nil {
		return err
	}

	for _, multiplier := range search.config.ServingMultipliers {
		err := search.tryMultiplier(ctx, index, total, multiplier)
		search.chosen[index] = 0
		if err != nil {
			return err
		}
	}
	return nil
}

func (search *daySearch) enter(ctx context.Context, index int, total nutrition.Macros) (done bool, err error) {
	if err := ctx.Err(); err != nil {
		return true, err
	}
	search.visited++
	if search.visited > search.config.SearchBudget {
		search.limited = true
		return true, errSearchBudget
	}
	if exceedsMaximum(total, search.target) || cannotReachMinimum(total, search.suffixMaximums[index], search.target) {
		return true, nil
	}
	if index == len(search.variables) {
		if !allMealsPopulated(search.mealComponentCount) || !search.target.Contains(total) {
			return true, nil
		}
		return true, search.consider()
	}
	return false, nil
}

// consider ranks one macro-feasible candidate. Hard quality violations keep the
// candidate infeasible; soft failures only lower its rank.
func (search *daySearch) consider() error {
	solution, err := search.solution(len(search.mealTypes))
	if err != nil {
		return err
	}
	evaluated, err := search.evaluator.evaluate(solution, search.mealTypes)
	if err != nil {
		return err
	}
	if len(evaluated.hard) > 0 {
		return nil
	}

	score := scoreDay(search.ruleset, evaluated.evaluation, solution, search.priorUses)
	if search.best == nil || score.betterThan(search.bestScore) {
		search.best = &rankedDay{
			solution:   solution,
			day:        evaluated.day,
			rolling:    evaluated.rolling,
			evaluation: evaluated.evaluation,
		}
		search.bestScore = score
	}

	search.accepted++
	if search.accepted >= search.config.CandidateBudget {
		return errCandidateBudget
	}
	return nil
}

func (search *daySearch) tryMultiplier(
	ctx context.Context,
	index int,
	total nutrition.Macros,
	multiplier float64,
) error {
	variable := &search.variables[index]
	macros, err := variable.variant.macros(multiplier)
	if err != nil {
		return err
	}
	next, err := total.Add(macros)
	if err != nil {
		return err
	}
	search.chosen[index] = multiplier
	search.mealComponentCount[variable.mealIndex]++
	err = search.walk(ctx, index+1, next)
	search.mealComponentCount[variable.mealIndex]--
	return err
}

func (search *daySearch) solution(mealCount int) (daySolution, error) {
	result := daySolution{mealMacros: make([]nutrition.Macros, mealCount)}
	for i := range search.variables {
		if search.chosen[i] == 0 {
			continue
		}
		variable := &search.variables[i]
		macros, err := variable.variant.macros(search.chosen[i])
		if err != nil {
			return daySolution{}, err
		}
		result.mealMacros[variable.mealIndex], err = result.mealMacros[variable.mealIndex].Add(macros)
		if err != nil {
			return daySolution{}, err
		}
		result.uses = append(result.uses, componentUse{
			mealIndex:    variable.mealIndex,
			variant:      variable.variant,
			servingUnits: search.chosen[i],
		})
	}
	return result, nil
}

func searchVariables(mealTypes []mealdom.Type, recipes []RecipeCandidate) ([]searchVariable, error) {
	variables := make([]searchVariable, 0)
	for mealIndex, mealType := range mealTypes {
		for recipeIndex := range recipes {
			recipe := &recipes[recipeIndex]
			if !allowsMealType(recipe.Recipe.AllowedMealTypes, mealType) {
				continue
			}
			variants, err := recipeVariants(*recipe)
			if err != nil {
				return nil, err
			}
			for variantIndex := range variants {
				variables = append(variables, searchVariable{mealIndex: mealIndex, variant: variants[variantIndex]})
			}
		}
	}
	return variables, nil
}

func recipeVariants(candidate RecipeCandidate) ([]componentVariant, error) {
	if candidate.Recipe.BaseServings <= 0 {
		return nil, fmt.Errorf("recipe %d has no positive base servings", candidate.Recipe.ID)
	}
	variants := make([]componentVariant, 0)
	products := make([]catalogdom.Product, len(candidate.Slots))
	included := make([]bool, len(candidate.Slots))

	var expand func(int) error
	expand = func(index int) error {
		if index == len(candidate.Slots) {
			baseMacros, fills, slots, err := filledRecipeMacros(candidate.Slots, products, included)
			if err != nil {
				return err
			}
			variants = append(variants, componentVariant{
				recipe:     candidate.Recipe,
				baseFills:  fills,
				baseMacros: baseMacros,
				slots:      slots,
			})
			return nil
		}

		slot := &candidate.Slots[index]
		if slot.Slot.Optional {
			included[index] = false
			if err := expand(index + 1); err != nil {
				return err
			}
		}
		for productIndex := range slot.Products {
			included[index] = true
			products[index] = slot.Products[productIndex]
			if err := expand(index + 1); err != nil {
				return err
			}
		}
		return nil
	}
	if err := expand(0); err != nil {
		return nil, err
	}
	return variants, nil
}

func filledRecipeMacros(
	slots []SlotCandidates,
	products []catalogdom.Product,
	included []bool,
) (nutrition.Macros, []planningdom.SlotFill, []variantSlot, error) {
	var proteinG, fatG, carbG, kcal float64
	fills := make([]planningdom.SlotFill, 0, len(slots))
	variantSlots := make([]variantSlot, 0, len(slots))
	for i := range slots {
		if !included[i] {
			continue
		}
		product := &products[i]
		qtyG := slots[i].Slot.QtyG
		if !validProductMacros(*product) || math.IsNaN(qtyG) || math.IsInf(qtyG, 0) || qtyG <= 0 {
			return nutrition.Macros{}, nil, nil, errors.New("recipe product macros and quantity must be finite and positive")
		}
		factor := qtyG / gramsPer100
		proteinG += product.ProteinPer100G * factor
		fatG += product.FatPer100G * factor
		carbG += product.CarbPer100G * factor
		kcal += product.KcalPer100G * factor
		fills = append(fills, planningdom.SlotFill{
			SlotID:           slots[i].Slot.ID,
			ProductID:        product.ID,
			PlannedInputQtyG: qtyG,
		})
		variantSlots = append(variantSlots, newVariantSlot(slots[i], *product))
	}
	result := nutrition.Macros{ProteinG: proteinG, FatG: fatG, CarbG: carbG, Kcal: int(math.Round(kcal))}
	if !result.Valid() {
		return nutrition.Macros{}, nil, nil, errors.New("filled recipe macros must be finite and nonnegative")
	}
	return result, fills, variantSlots, nil
}

func validProductMacros(product catalogdom.Product) bool {
	grams := nutrition.Macros{ProteinG: product.ProteinPer100G, FatG: product.FatPer100G, CarbG: product.CarbPer100G}
	return grams.Valid() && !math.IsNaN(product.KcalPer100G) && !math.IsInf(product.KcalPer100G, 0) && product.KcalPer100G >= 0
}

func suffixMaximums(variables []searchVariable, multipliers []float64) ([]nutrition.Macros, error) {
	maximumMultiplier := multipliers[0]
	for _, multiplier := range multipliers[1:] {
		maximumMultiplier = max(maximumMultiplier, multiplier)
	}
	result := make([]nutrition.Macros, len(variables)+1)
	for i := len(variables) - 1; i >= 0; i-- {
		maximum, err := variables[i].variant.macros(maximumMultiplier)
		if err != nil {
			return nil, err
		}
		result[i], err = result[i+1].Add(maximum)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (variant componentVariant) macros(servingUnits float64) (nutrition.Macros, error) {
	return variant.baseMacros.Scale(servingUnits / float64(variant.recipe.BaseServings))
}

func exceedsMaximum(macros nutrition.Macros, target eaterdom.MacroRange) bool {
	return macros.ProteinG > target.ProteinMaxG || macros.FatG > target.FatMaxG ||
		macros.CarbG > target.CarbMaxG || macros.Kcal > target.KcalMax
}

func cannotReachMinimum(current, remaining nutrition.Macros, target eaterdom.MacroRange) bool {
	return current.ProteinG+remaining.ProteinG < target.ProteinMinG ||
		current.FatG+remaining.FatG < target.FatMinG ||
		current.CarbG+remaining.CarbG < target.CarbMinG ||
		current.Kcal+remaining.Kcal < target.KcalMin
}

func allMealsPopulated(componentCounts []int) bool {
	for _, count := range componentCounts {
		if count == 0 {
			return false
		}
	}
	return true
}

func allowsMealType(allowed []mealdom.Type, mealType mealdom.Type) bool {
	for _, candidate := range allowed {
		if candidate == mealType {
			return true
		}
	}
	return false
}
