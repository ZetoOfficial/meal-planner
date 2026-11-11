package swap

import (
	"fmt"
	"time"

	catalogdom "github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
	eaterdom "github.com/ZetoOfficial/meal-planner/internal/domain/eater"
	"github.com/ZetoOfficial/meal-planner/internal/domain/nutrition"
	planningdom "github.com/ZetoOfficial/meal-planner/internal/domain/planning"
	"github.com/ZetoOfficial/meal-planner/internal/quality"
)

// materialMacroDelta is the smallest macro movement worth calling a change; a
// swap below it reads as "no change" in the summary.
const materialMacroDelta = 0.5

// indexDays keys evaluations by eater-day, so two plans line up even when
// their days are stored in a different order.
func indexDays(days []dayEvaluation) map[string]dayEvaluation {
	index := make(map[string]dayEvaluation, len(days))
	for i := range days {
		index[dayKey(days[i].eaterID, days[i].date)] = days[i]
	}
	return index
}

func macroDiff(before, after []dayEvaluation) []EaterDayMacroDelta {
	previous := indexDays(before)
	deltas := make([]EaterDayMacroDelta, 0, len(after))
	for i := range after {
		old, ok := previous[dayKey(after[i].eaterID, after[i].date)]
		if !ok {
			continue
		}
		deltas = append(deltas, EaterDayMacroDelta{
			EaterID: after[i].eaterID,
			Date:    after[i].date,
			Delta: MacroDelta{
				ProteinG: after[i].macros.ProteinG - old.macros.ProteinG,
				FatG:     after[i].macros.FatG - old.macros.FatG,
				CarbG:    after[i].macros.CarbG - old.macros.CarbG,
				Kcal:     after[i].macros.Kcal - old.macros.Kcal,
			},
		})
	}
	return deltas
}

func qualityDiff(before, after []dayEvaluation) QualityDelta {
	previous := indexDays(before)
	delta := QualityDelta{ChangedRules: make([]RuleChange, 0)}
	compared := 0
	// Weekly exposure is cumulative, so each eater's latest day carries their
	// projection. Eaters are kept in first-seen order to sum deterministically.
	weekly := make(map[eaterdom.ID]*weeklyDelta)
	eaters := make([]eaterdom.ID, 0)
	for i := range after {
		old, ok := previous[dayKey(after[i].eaterID, after[i].date)]
		if !ok {
			continue
		}
		compared++
		delta.FiberG += after[i].day.Totals.FiberG - old.day.Totals.FiberG
		delta.SodiumMg += after[i].day.Totals.SodiumMg - old.day.Totals.SodiumMg
		delta.SaturatedFatG += after[i].day.Totals.SaturatedFatG - old.day.Totals.SaturatedFatG
		delta.AddedSugarsG += after[i].day.Totals.AddedSugarsG - old.day.Totals.AddedSugarsG
		delta.FruitVegG += after[i].day.Totals.FruitVegG - old.day.Totals.FruitVegG
		delta.CoverageScore += after[i].day.Coverage.Score() - old.day.Coverage.Score()
		delta.ChangedRules = append(delta.ChangedRules, changedRules(old, after[i])...)

		latest, seen := weekly[after[i].eaterID]
		if !seen {
			latest = &weeklyDelta{}
			weekly[after[i].eaterID] = latest
			eaters = append(eaters, after[i].eaterID)
		}
		if !seen || !after[i].date.Before(latest.date) {
			latest.date = after[i].date
			latest.redMeatG = after[i].rolling.RedMeatG - old.rolling.RedMeatG
			latest.processedMeatG = after[i].rolling.ProcessedMeatG - old.rolling.ProcessedMeatG
		}
	}
	for _, eaterID := range eaters {
		delta.RedMeatGRolling7d += weekly[eaterID].redMeatG
		delta.ProcessedMeatGRolling7d += weekly[eaterID].processedMeatG
	}
	if compared > 0 {
		delta.CoverageScore /= float64(compared)
	}
	return delta
}

// weeklyDelta is the movement of one eater's rolling exposure on their latest
// compared day.
type weeklyDelta struct {
	date           time.Time
	redMeatG       float64
	processedMeatG float64
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

	invalidated := make([]InvalidatedRecipe, 0)
	seen := map[int]struct{}{recipeIndex: {}}
	queue := []int{recipeIndex}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, dependency := range plan.PrepDependencies {
			dependent := dependency.DependentRecipeIndex
			if dependency.SourceRecipeIndex != current || dependent < 0 || dependent >= len(plan.Recipes) {
				continue
			}
			if _, exists := seen[dependent]; exists {
				continue
			}
			seen[dependent] = struct{}{}
			invalidated = append(invalidated, InvalidatedRecipe{
				PlanRecipeID: plan.Recipes[dependent].ID,
				Reason:       reasonFor(plan, dependency),
			})
			queue = append(queue, dependent)
		}
	}
	return invalidated
}

func reasonFor(plan planningdom.Plan, dependency planningdom.PrepDependency) string {
	if dependency.Note != nil && *dependency.Note != "" {
		return *dependency.Note
	}
	return fmt.Sprintf("is assembled from plan recipe %d", plan.Recipes[dependency.SourceRecipeIndex].ID)
}

// shoppingDelta reports products the list gains and loses, in a stable order.
// A product still used by another fill is neither added nor removed.
func shoppingDelta(before, after planningdom.Plan) ShoppingDelta {
	delta := ShoppingDelta{Added: make([]catalogdom.ProductID, 0), Removed: make([]catalogdom.ProductID, 0)}
	old := quantitiesByProduct(before)
	current := quantitiesByProduct(after)
	for _, productID := range productOrder(after) {
		if _, had := old[productID]; !had {
			delta.Added = append(delta.Added, productID)
		}
	}
	for _, productID := range productOrder(before) {
		if _, has := current[productID]; !has {
			delta.Removed = append(delta.Removed, productID)
		}
	}
	return delta
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

func summarise(oldName, newName string, preview Preview) string {
	if !materialChange(preview) {
		return fmt.Sprintf("swap %s for %s — no material change", oldName, newName)
	}
	return fmt.Sprintf(
		"swap %s for %s — %d meal(s) change, %d rule verdict(s) move, %d dish(es) invalidated%s",
		oldName, newName, len(preview.AffectedMeals), len(preview.QualityDiff.ChangedRules), len(preview.InvalidatedRecipes),
		breachNote(preview.MacroGoalBreaches),
	)
}

// breachNote spells out the macro cost of a swap: portions are not rebalanced,
// so a day pushed out of range stays out until the owner adjusts a quantity.
func breachNote(breaches int) string {
	if breaches == 0 {
		return ""
	}
	return fmt.Sprintf("; %d eater-day(s) leave the macro goal, adjust a quantity to rebalance", breaches)
}

func materialChange(preview Preview) bool {
	if len(preview.QualityDiff.ChangedRules) > 0 || len(preview.InvalidatedRecipes) > 0 || preview.MacroGoalBreaches > 0 {
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
