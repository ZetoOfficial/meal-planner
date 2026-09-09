package swap

import (
	"fmt"

	catalogdom "github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
	"github.com/ZetoOfficial/meal-planner/internal/domain/nutrition"
	planningdom "github.com/ZetoOfficial/meal-planner/internal/domain/planning"
	"github.com/ZetoOfficial/meal-planner/internal/quality"
)

// materialMacroDelta is the smallest macro movement worth calling a change; a
// swap below it reads as "no change" in the summary.
const materialMacroDelta = 0.5

func macroDiff(before, after []dayEvaluation) []EaterDayMacroDelta {
	deltas := make([]EaterDayMacroDelta, 0, len(after))
	for i := range after {
		if i >= len(before) {
			break
		}
		delta := MacroDelta{
			ProteinG: after[i].macros.ProteinG - before[i].macros.ProteinG,
			FatG:     after[i].macros.FatG - before[i].macros.FatG,
			CarbG:    after[i].macros.CarbG - before[i].macros.CarbG,
			Kcal:     after[i].macros.Kcal - before[i].macros.Kcal,
		}
		deltas = append(deltas, EaterDayMacroDelta{
			EaterID: after[i].eaterID,
			Date:    after[i].date,
			Delta:   delta,
		})
	}
	return deltas
}

func qualityDiff(before, after []dayEvaluation) QualityDelta {
	delta := QualityDelta{ChangedRules: make([]RuleChange, 0)}
	for i := range after {
		if i >= len(before) {
			break
		}
		delta.FiberG += after[i].day.Totals.FiberG - before[i].day.Totals.FiberG
		delta.SodiumMg += after[i].day.Totals.SodiumMg - before[i].day.Totals.SodiumMg
		delta.SaturatedFatG += after[i].day.Totals.SaturatedFatG - before[i].day.Totals.SaturatedFatG
		delta.AddedSugarsG += after[i].day.Totals.AddedSugarsG - before[i].day.Totals.AddedSugarsG
		delta.FruitVegG += after[i].day.Totals.FruitVegG - before[i].day.Totals.FruitVegG
		delta.CoverageScore += after[i].day.Coverage.Score() - before[i].day.Coverage.Score()
		delta.ChangedRules = append(delta.ChangedRules, changedRules(before[i], after[i])...)
	}

	// Weekly exposure is cumulative, so the last day carries the projection.
	if len(after) > 0 && len(before) == len(after) {
		last := len(after) - 1
		delta.RedMeatGRolling7d = after[last].rolling.RedMeatG - before[last].rolling.RedMeatG
		delta.ProcessedMeatGRolling7d = after[last].rolling.ProcessedMeatG - before[last].rolling.ProcessedMeatG
	}
	if len(after) > 0 {
		delta.CoverageScore /= float64(len(after))
	}
	return delta
}

func changedRules(before, after dayEvaluation) []RuleChange {
	previous := make(map[string]nutrition.RuleEvaluationResult, len(before.rules))
	for _, rule := range before.rules {
		previous[rule.RuleCode] = rule
	}

	changes := make([]RuleChange, 0)
	for _, rule := range after.rules {
		old, ok := previous[rule.RuleCode]
		if !ok || old.Status == rule.Status {
			continue
		}
		changes = append(changes, RuleChange{
			EaterID: after.eaterID,
			Date:    after.date,
			Before:  old,
			After:   rule,
		})
	}
	return changes
}

// affectedMeals lists every meal that eats from the swapped batch.
func affectedMeals(plan planningdom.Plan, recipeIndex int) []planningdom.MealID {
	meals := make([]planningdom.MealID, 0)
	for i := range plan.Meals {
		for _, component := range plan.Meals[i].Components {
			if component.PlanRecipeIndex == recipeIndex {
				meals = append(meals, plan.Meals[i].ID)
				break
			}
		}
	}
	return meals
}

// invalidatedRecipes walks the prep graph: every dish cooked from the swapped
// batch becomes invalid, because its ingredient is no longer what it planned on.
func invalidatedRecipes(plan planningdom.Plan, recipeIndex int) []InvalidatedRecipe {
	if recipeIndex >= len(plan.Recipes) {
		return nil
	}
	sourceID := plan.Recipes[recipeIndex].ID

	invalidated := make([]InvalidatedRecipe, 0)
	seen := map[planningdom.PlanRecipeID]struct{}{}
	queue := []planningdom.PlanRecipeID{sourceID}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, dependency := range plan.PrepDependencies {
			if dependency.SourceRecipeID != current {
				continue
			}
			if _, exists := seen[dependency.DependentRecipeID]; exists {
				continue
			}
			seen[dependency.DependentRecipeID] = struct{}{}
			invalidated = append(invalidated, InvalidatedRecipe{
				PlanRecipeID: dependency.DependentRecipeID,
				Reason:       reasonFor(dependency),
			})
			queue = append(queue, dependency.DependentRecipeID)
		}
	}
	return invalidated
}

func reasonFor(dependency planningdom.PrepDependency) string {
	if dependency.Note != nil && *dependency.Note != "" {
		return *dependency.Note
	}
	return fmt.Sprintf("uses the output of plan recipe %d", dependency.SourceRecipeID)
}

// shoppingDelta reports products the list gains and loses. A product still used
// by another fill is neither added nor removed.
func shoppingDelta(
	before, after planningdom.Plan,
	oldProductID, newProductID catalogdom.ProductID,
) ShoppingDelta {
	delta := ShoppingDelta{Added: make([]catalogdom.ProductID, 0), Removed: make([]catalogdom.ProductID, 0)}
	if oldProductID == newProductID {
		return delta
	}
	if !usesProduct(before, newProductID) {
		delta.Added = append(delta.Added, newProductID)
	}
	if !usesProduct(after, oldProductID) {
		delta.Removed = append(delta.Removed, oldProductID)
	}
	return delta
}

func usesProduct(plan planningdom.Plan, productID catalogdom.ProductID) bool {
	for i := range plan.Recipes {
		for _, fill := range plan.Recipes[i].Fills {
			if fill.ProductID == productID {
				return true
			}
		}
	}
	return false
}

// levelOf separates a like-for-like fill change from a protein change, which
// moves the day's macros enough to shift the side portions.
func levelOf(input Input, recipeIndex, fillIndex int) Level {
	fill := input.Plan.Recipes[recipeIndex].Fills[fillIndex]
	template, ok := input.Catalogue.Templates[input.Plan.Recipes[recipeIndex].TemplateID]
	if !ok {
		return LevelFillToFill
	}
	for _, slot := range template.Slots {
		if slot.ID != fill.SlotID {
			continue
		}
		if role, found := roleOf(input.Catalogue, slot.RoleTagCode); found && role == catalogdom.RoleKindProtein {
			return LevelProteinChange
		}
	}
	return LevelFillToFill
}

func roleOf(catalogue quality.Catalogue, code string) (catalogdom.RoleKind, bool) {
	for id := range catalogue.Products {
		product := catalogue.Products[id]
		for _, role := range product.Roles {
			if role.Code == code {
				return role.Kind, true
			}
		}
	}
	return "", false
}

func summarise(
	catalogue quality.Catalogue,
	oldProductID, newProductID catalogdom.ProductID,
	preview Preview,
) string {
	oldName := productName(catalogue, oldProductID)
	newName := productName(catalogue, newProductID)
	if !materialChange(preview) {
		return fmt.Sprintf("swap %s for %s — no material change", oldName, newName)
	}
	return fmt.Sprintf(
		"swap %s for %s — %d meal(s) change, %d rule verdict(s) move, %d dish(es) invalidated",
		oldName, newName, len(preview.AffectedMeals), len(preview.QualityDiff.ChangedRules), len(preview.InvalidatedRecipes),
	)
}

func materialChange(preview Preview) bool {
	if len(preview.QualityDiff.ChangedRules) > 0 || len(preview.InvalidatedRecipes) > 0 {
		return true
	}
	for _, delta := range preview.MacroDiff {
		if absolute(delta.Delta.ProteinG) > materialMacroDelta ||
			absolute(delta.Delta.FatG) > materialMacroDelta ||
			absolute(delta.Delta.CarbG) > materialMacroDelta {
			return true
		}
	}
	return false
}

func absolute(value float64) float64 {
	if value < 0 {
		return -value
	}
	return value
}

func productName(catalogue quality.Catalogue, id catalogdom.ProductID) string {
	if product, ok := catalogue.Products[id]; ok {
		return product.Name
	}
	return fmt.Sprintf("product %d", id)
}
