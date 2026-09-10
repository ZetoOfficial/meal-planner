package httpapi

import (
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
		EstTotal: list.EstimatedTotal(),
	}
}

func shoppingItemResponse(item shoppingdom.Item) model.ShoppingItem {
	name := item.ProductName
	return model.ShoppingItem{
		Id:          int64(item.ID),
		ProductId:   int64(item.ProductID),
		ProductName: &name,
		TotalQtyG:   item.TotalQtyG,
		Unit:        item.Unit,
		EstCost:     item.EstCost,
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

	byEater := make([]model.SwapEaterMacroDelta, 0, len(preview.MacroDiff))
	for _, delta := range preview.MacroDiff {
		date := openapiDate(delta.Date)
		eaterID := int64(delta.EaterID)
		macros := model.Macros{
			ProteinG: delta.Delta.ProteinG,
			FatG:     delta.Delta.FatG,
			CarbG:    delta.Delta.CarbG,
			Kcal:     delta.Delta.Kcal,
		}
		byEater = append(byEater, model.SwapEaterMacroDelta{Date: &date, Delta: &macros, EaterId: &eaterID})
	}
	response.MacroDiff = &model.SwapMacroDiff{ByEater: &byEater}

	added := productIDs(preview.ShoppingDelta.Added)
	removed := productIDs(preview.ShoppingDelta.Removed)
	response.ShoppingDelta = &model.SwapShoppingDelta{Added: &added, Removed: &removed}

	response.QualityDiff = qualityDiffResponse(preview.QualityDiff)
	return response
}

func qualityDiffResponse(diff swap.QualityDelta) *model.SwapQualityDiff {
	changed := changedRulesResponse(diff)
	return &model.SwapQualityDiff{
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
			ActualValue:   change.After.Actual,
			TargetValue:   change.After.Target,
			Unit:          optionalString(change.After.Unit),
			CoverageScore: change.After.Coverage,
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

func invalidatedRecipesResponse(preview swap.Preview) *[]model.SwapInvalidatedRecipe {
	entries := make([]model.SwapInvalidatedRecipe, 0, len(preview.InvalidatedRecipes))
	for _, invalidated := range preview.InvalidatedRecipes {
		id := int64(invalidated.PlanRecipeID)
		reason := invalidated.Reason
		entries = append(entries, model.SwapInvalidatedRecipe{PlanRecipeId: &id, Reason: &reason})
	}
	return &entries
}
