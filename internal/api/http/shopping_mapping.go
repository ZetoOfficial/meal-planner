package httpapi

import (
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/ZetoOfficial/meal-planner/api/sdk/go/model"
	catalogdom "github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
	shoppingdom "github.com/ZetoOfficial/meal-planner/internal/domain/shopping"
	"github.com/ZetoOfficial/meal-planner/internal/swap"
)

func shoppingListResponse(list shoppingdom.List) model.ShoppingList {
	items := make([]model.ShoppingItem, 0, len(list.Items))
	for _, item := range list.Items {
		items = append(items, shoppingItemResponse(item))
	}
	return model.ShoppingList{
		Id:       int64(list.ID),
		PlanId:   int64(list.PlanID),
		Items:    items,
		EstTotal: float32Pointer(list.EstimatedTotal()),
	}
}

func shoppingItemResponse(item shoppingdom.Item) model.ShoppingItem {
	name := item.ProductName
	return model.ShoppingItem{
		Id:          int64(item.ID),
		ProductId:   int64(item.ProductID),
		ProductName: &name,
		TotalQtyG:   float32(item.TotalQtyG),
		Unit:        item.Unit,
		EstCost:     float32Pointer(item.EstCost),
		Checked:     item.Checked,
	}
}

func swapPreviewResponse(preview swap.Preview) model.SwapPreview {
	response := model.SwapPreview{
		Level:              model.SwapPreviewLevel(preview.Level),
		Summary:            preview.Summary,
		AffectedMeals:      affectedMealIDs(preview),
		InvalidatedRecipes: invalidatedRecipesResponse(preview),
	}

	byEater := make([]struct {
		Date    *openapi_types.Date `json:"date,omitempty"`
		Delta   *model.Macros       `json:"delta,omitempty"`
		EaterId *int64              `json:"eater_id,omitempty"`
	}, 0, len(preview.MacroDiff))
	for _, delta := range preview.MacroDiff {
		date := openapiDate(delta.Date)
		eaterID := int64(delta.EaterID)
		macros := model.Macros{
			ProteinG: float32(delta.Delta.ProteinG),
			FatG:     float32(delta.Delta.FatG),
			CarbG:    float32(delta.Delta.CarbG),
			Kcal:     delta.Delta.Kcal,
		}
		byEater = append(byEater, struct {
			Date    *openapi_types.Date `json:"date,omitempty"`
			Delta   *model.Macros       `json:"delta,omitempty"`
			EaterId *int64              `json:"eater_id,omitempty"`
		}{Date: &date, Delta: &macros, EaterId: &eaterID})
	}
	response.MacroDiff = &struct {
		ByEater *[]struct {
			Date    *openapi_types.Date `json:"date,omitempty"`
			Delta   *model.Macros       `json:"delta,omitempty"`
			EaterId *int64              `json:"eater_id,omitempty"`
		} `json:"by_eater,omitempty"`
	}{ByEater: &byEater}

	added := productIDs(preview.ShoppingDelta.Added)
	removed := productIDs(preview.ShoppingDelta.Removed)
	response.ShoppingDelta = &struct {
		Added   *[]int64 `json:"added,omitempty"`
		Removed *[]int64 `json:"removed,omitempty"`
	}{Added: &added, Removed: &removed}

	response.QualityDiff = qualityDiffResponse(preview.QualityDiff)
	return response
}

func qualityDiffResponse(diff swap.QualityDelta) *struct {
	AddedSugarsG            *float32                `json:"added_sugars_g,omitempty"`
	ChangedRules            *[]model.RuleEvaluation `json:"changed_rules,omitempty"`
	CoverageScore           *float32                `json:"coverage_score,omitempty"`
	FiberG                  *float32                `json:"fiber_g,omitempty"`
	FruitVegG               *float32                `json:"fruit_veg_g,omitempty"`
	ProcessedMeatGRolling7d *float32                `json:"processed_meat_g_rolling_7d,omitempty"`
	RedMeatGRolling7d       *float32                `json:"red_meat_g_rolling_7d,omitempty"`
	SaturatedFatG           *float32                `json:"saturated_fat_g,omitempty"`
	SodiumMg                *float32                `json:"sodium_mg,omitempty"`
} {
	changed := changedRulesResponse(diff)
	return &struct {
		AddedSugarsG            *float32                `json:"added_sugars_g,omitempty"`
		ChangedRules            *[]model.RuleEvaluation `json:"changed_rules,omitempty"`
		CoverageScore           *float32                `json:"coverage_score,omitempty"`
		FiberG                  *float32                `json:"fiber_g,omitempty"`
		FruitVegG               *float32                `json:"fruit_veg_g,omitempty"`
		ProcessedMeatGRolling7d *float32                `json:"processed_meat_g_rolling_7d,omitempty"`
		RedMeatGRolling7d       *float32                `json:"red_meat_g_rolling_7d,omitempty"`
		SaturatedFatG           *float32                `json:"saturated_fat_g,omitempty"`
		SodiumMg                *float32                `json:"sodium_mg,omitempty"`
	}{
		AddedSugarsG:            floatValuePointer(diff.AddedSugarsG),
		ChangedRules:            &changed,
		CoverageScore:           floatValuePointer(diff.CoverageScore),
		FiberG:                  floatValuePointer(diff.FiberG),
		FruitVegG:               floatValuePointer(diff.FruitVegG),
		ProcessedMeatGRolling7d: floatValuePointer(diff.ProcessedMeatGRolling7d),
		RedMeatGRolling7d:       floatValuePointer(diff.RedMeatGRolling7d),
		SaturatedFatG:           floatValuePointer(diff.SaturatedFatG),
		SodiumMg:                floatValuePointer(diff.SodiumMg),
	}
}

// changedRulesResponse renders the verdicts that moved, so a diff explains
// itself rather than only reporting numbers.
func changedRulesResponse(diff swap.QualityDelta) []model.RuleEvaluation {
	changed := make([]model.RuleEvaluation, 0, len(diff.ChangedRules))
	for i := range diff.ChangedRules {
		change := &diff.ChangedRules[i]
		eaterID := int64(change.EaterID)
		date := openapiDate(change.Date)
		changed = append(changed, model.RuleEvaluation{
			RuleCode:      change.After.RuleCode,
			EntityType:    model.RuleEvaluationEntityType("day"),
			EaterId:       &eaterID,
			Basis:         model.RuleEvaluationBasis("planned"),
			Status:        model.RuleEvaluationStatus(change.After.Status),
			ActualValue:   float32Pointer(change.After.Actual),
			TargetValue:   float32Pointer(change.After.Target),
			Unit:          optionalString(change.After.Unit),
			CoverageScore: float32(change.After.Coverage),
			Message:       optionalString(change.After.Message),
			Date:          &date,
		})
	}
	return changed
}

func productIDs(ids []catalogdom.ProductID) []int64 {
	result := make([]int64, 0, len(ids))
	for _, id := range ids {
		result = append(result, int64(id))
	}
	return result
}

func affectedMealIDs(preview swap.Preview) *[]int64 {
	ids := make([]int64, 0, len(preview.AffectedMeals))
	for _, id := range preview.AffectedMeals {
		ids = append(ids, int64(id))
	}
	return &ids
}

func invalidatedRecipesResponse(preview swap.Preview) *[]struct {
	PlanRecipeId *int64  `json:"plan_recipe_id,omitempty"`
	Reason       *string `json:"reason,omitempty"`
} {
	entries := make([]struct {
		PlanRecipeId *int64  `json:"plan_recipe_id,omitempty"`
		Reason       *string `json:"reason,omitempty"`
	}, 0, len(preview.InvalidatedRecipes))
	for _, invalidated := range preview.InvalidatedRecipes {
		id := int64(invalidated.PlanRecipeID)
		reason := invalidated.Reason
		entries = append(entries, struct {
			PlanRecipeId *int64  `json:"plan_recipe_id,omitempty"`
			Reason       *string `json:"reason,omitempty"`
		}{PlanRecipeId: &id, Reason: &reason})
	}
	return &entries
}
