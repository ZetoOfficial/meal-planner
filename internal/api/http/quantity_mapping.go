package httpapi

import (
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/ZetoOfficial/meal-planner/api/sdk/go/model"
	"github.com/ZetoOfficial/meal-planner/internal/swap"
)

type quantityMacroDelta = struct {
	Date    *openapi_types.Date `json:"date,omitempty"`
	Delta   model.Macros        `json:"delta"`
	EaterId int64               `json:"eater_id"`
}

type quantityPortionChange = struct {
	After   float32             `json:"after"`
	Before  float32             `json:"before"`
	Date    *openapi_types.Date `json:"date,omitempty"`
	EaterId int64               `json:"eater_id"`
	MealId  int64               `json:"meal_id"`
}

type quantityShoppingChange = struct {
	AfterG    float32 `json:"after_g"`
	BeforeG   float32 `json:"before_g"`
	ProductId int64   `json:"product_id"`
}

type quantityQualityDiff = struct {
	AddedSugarsG  *float32                `json:"added_sugars_g,omitempty"`
	ChangedRules  *[]model.RuleEvaluation `json:"changed_rules,omitempty"`
	CoverageScore *float32                `json:"coverage_score,omitempty"`
	FiberG        *float32                `json:"fiber_g,omitempty"`
	FruitVegG     *float32                `json:"fruit_veg_g,omitempty"`
	SaturatedFatG *float32                `json:"saturated_fat_g,omitempty"`
	SodiumMg      *float32                `json:"sodium_mg,omitempty"`
}

func quantityPreviewResponse(preview swap.QuantityPreview) model.QuantityAdjustmentPreview {
	response := model.QuantityAdjustmentPreview{
		Summary:        preview.Summary,
		AffectedMeals:  make([]int64, 0, len(preview.AffectedMeals)),
		PortionChanges: make([]quantityPortionChange, 0, len(preview.PortionChanges)),
		MacroDiff:      make([]quantityMacroDelta, 0, len(preview.MacroDiff)),
		ShoppingDiff:   make([]quantityShoppingChange, 0, len(preview.ShoppingDiff)),
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
		response.PortionChanges = append(response.PortionChanges, quantityPortionChange{
			After:   float32(change.After),
			Before:  float32(change.Before),
			Date:    &date,
			EaterId: int64(change.EaterID),
			MealId:  int64(change.MealID),
		})
	}
	for _, delta := range preview.MacroDiff {
		date := openapiDate(delta.Date)
		response.MacroDiff = append(response.MacroDiff, quantityMacroDelta{
			Date:    &date,
			EaterId: int64(delta.EaterID),
			Delta: model.Macros{
				ProteinG: float32(delta.Delta.ProteinG),
				FatG:     float32(delta.Delta.FatG),
				CarbG:    float32(delta.Delta.CarbG),
				Kcal:     delta.Delta.Kcal,
			},
		})
	}
	for _, change := range preview.ShoppingDiff {
		response.ShoppingDiff = append(response.ShoppingDiff, quantityShoppingChange{
			AfterG:    float32(change.AfterG),
			BeforeG:   float32(change.BeforeG),
			ProductId: int64(change.ProductID),
		})
	}

	changed := changedRulesResponse(preview.QualityDiff)
	response.QualityDiff = &quantityQualityDiff{
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
