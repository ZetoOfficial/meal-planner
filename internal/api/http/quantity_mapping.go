package httpapi

import (
	"github.com/ZetoOfficial/meal-planner/api/sdk/go/model"
	"github.com/ZetoOfficial/meal-planner/internal/swap"
)

func quantityPreviewResponse(preview swap.QuantityPreview) model.QuantityAdjustmentPreview {
	response := model.QuantityAdjustmentPreview{
		Summary:        preview.Summary,
		AffectedMeals:  make([]int64, 0, len(preview.AffectedMeals)),
		PortionChanges: make([]model.QuantityPortionChange, 0, len(preview.PortionChanges)),
		MacroDiff:      make([]model.QuantityMacroDelta, 0, len(preview.MacroDiff)),
		ShoppingDiff:   make([]model.QuantityShoppingChange, 0, len(preview.ShoppingDiff)),
		Warnings:       preview.Warnings,
	}
	if response.Warnings == nil {
		response.Warnings = []string{}
	}

	for _, id := range preview.AffectedMeals {
		response.AffectedMeals = append(response.AffectedMeals, int64(id))
	}
	for _, change := range preview.PortionChanges {
		date := openapiDate(change.Date)
		response.PortionChanges = append(response.PortionChanges, model.QuantityPortionChange{
			After:   change.After,
			Before:  change.Before,
			Date:    &date,
			EaterId: int64(change.EaterID),
			MealId:  int64(change.MealID),
		})
	}
	for _, delta := range preview.MacroDiff {
		date := openapiDate(delta.Date)
		response.MacroDiff = append(response.MacroDiff, model.QuantityMacroDelta{
			Date:    &date,
			EaterId: int64(delta.EaterID),
			Delta: model.Macros{
				ProteinG: delta.Delta.ProteinG,
				FatG:     delta.Delta.FatG,
				CarbG:    delta.Delta.CarbG,
				Kcal:     delta.Delta.Kcal,
			},
		})
	}
	for _, change := range preview.ShoppingDiff {
		response.ShoppingDiff = append(response.ShoppingDiff, model.QuantityShoppingChange{
			AfterG:    change.AfterG,
			BeforeG:   change.BeforeG,
			ProductId: int64(change.ProductID),
		})
	}

	changed := changedRulesResponse(preview.QualityDiff)
	response.QualityDiff = &model.QualityDiff{
		AddedSugarsG:  floatValuePointer(preview.QualityDiff.AddedSugarsG),
		ChangedRules:  &changed,
		CoverageScore: floatValuePointer(preview.QualityDiff.CoverageScore),
		FiberG:        floatValuePointer(preview.QualityDiff.FiberG),
		FruitVegG:     floatValuePointer(preview.QualityDiff.FruitVegG),
		SaturatedFatG: floatValuePointer(preview.QualityDiff.SaturatedFatG),
		SodiumMg:      floatValuePointer(preview.QualityDiff.SodiumMg),
	}
	return response
}

// quantityInfeasibleResponse reuses the solver error shape so an impossible
// quantity is explained exactly like an impossible plan.
func quantityInfeasibleResponse(infeasible *swap.InfeasibleQuantityError) model.SolverError {
	response := model.SolverError{
		Error:       model.SolverErrorErrorSolverError,
		Code:        model.MacroInfeasible,
		Exhaustive:  true,
		Message:     infeasible.Error(),
		Diagnostics: make([]model.SolverDiagnostic, 0, len(infeasible.Constraints)),
	}
	eaterID := int64(infeasible.EaterID)
	date := openapiDate(infeasible.Date)
	for _, constraint := range infeasible.Constraints {
		response.Diagnostics = append(response.Diagnostics, macroDiagnostic(eaterID, date, constraint))
	}
	return response
}
