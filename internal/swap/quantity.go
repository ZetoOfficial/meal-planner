package swap

import (
	"errors"
	"fmt"
	"math"
	"time"

	catalogdom "github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
	eaterdom "github.com/ZetoOfficial/meal-planner/internal/domain/eater"
	"github.com/ZetoOfficial/meal-planner/internal/domain/nutrition"
	planningdom "github.com/ZetoOfficial/meal-planner/internal/domain/planning"
	"github.com/ZetoOfficial/meal-planner/internal/quality"
	"github.com/ZetoOfficial/meal-planner/internal/solver"
)

const (
	// portionStep is the grid the rebalance searches. It is coarse on purpose:
	// a household divides a tray into practical shares, not into micro-portions.
	portionStep    = 0.05
	minimumPortion = 0.05
	portionEpsilon = 1e-9
	// percentScale renders a batch reserve as whole percent.
	percentScale = 100.0
)

// QuantityRequest locks one slot fill to a measured quantity. The owner weighed
// the pack; the plan has to work around that number, not the other way round.
type QuantityRequest struct {
	SlotFillID       planningdom.SlotFillID
	PlannedInputQtyG float64
}

// GoalTarget is the macro range one eater must stay inside on one date.
type GoalTarget struct {
	EaterID eaterdom.ID
	Date    time.Time
	Range   eaterdom.MacroRange
}

type PortionChange struct {
	MealID  planningdom.MealID
	EaterID eaterdom.ID
	Date    time.Time
	Before  float64
	After   float64
}

type ShoppingQuantityChange struct {
	ProductID catalogdom.ProductID
	BeforeG   float64
	AfterG    float64
}

type QuantityPreview struct {
	Plan           planningdom.Plan
	AffectedMeals  []planningdom.MealID
	PortionChanges []PortionChange
	MacroDiff      []EaterDayMacroDelta
	QualityDiff    QualityDelta
	ShoppingDiff   []ShoppingQuantityChange
	Warnings       []string
	EaterDays      []DaySnapshot
	Summary        string
}

// InfeasibleQuantityError reports that no practical portion split keeps every
// eater-day inside its macro range at the locked quantity.
type InfeasibleQuantityError struct {
	Constraints []solver.MacroConstraint
	EaterID     eaterdom.ID
	Date        time.Time
}

func (e *InfeasibleQuantityError) Error() string {
	return fmt.Sprintf(
		"no portion split keeps eater %d inside the macro range on %s at this quantity",
		e.EaterID, e.Date.Format(time.DateOnly),
	)
}

// ErrNonPositiveQuantity is a request-level rejection, like [ErrUnknownFill].
var ErrNonPositiveQuantity = errors.New("planned input quantity must be positive")

// ComputeQuantity previews a locked quantity change: it rebalances only the
// shares of the affected batch, keeps every other batch untouched, and reports
// what moved. It never mutates the supplied plan.
func ComputeQuantity(input Input, goals []GoalTarget, request QuantityRequest) (QuantityPreview, error) {
	if request.PlannedInputQtyG <= 0 || math.IsNaN(request.PlannedInputQtyG) || math.IsInf(request.PlannedInputQtyG, 0) {
		return QuantityPreview{}, ErrNonPositiveQuantity
	}
	recipeIndex, fillIndex, err := locateFill(input.Plan, request.SlotFillID)
	if err != nil {
		return QuantityPreview{}, err
	}

	before, err := evaluatePlan(input, input.Plan)
	if err != nil {
		return QuantityPreview{}, err
	}

	adjusted := withAdjustedQuantity(input.Plan, recipeIndex, fillIndex, request.PlannedInputQtyG)
	portions, err := rebalance(input, adjusted, recipeIndex, goals)
	if err != nil {
		return QuantityPreview{}, err
	}
	adjusted = withPortions(adjusted, recipeIndex, portions)
	adjusted, err = recomputePlannedMacros(adjusted, input.Catalogue)
	if err != nil {
		return QuantityPreview{}, err
	}

	after, err := evaluatePlan(input, adjusted)
	if err != nil {
		return QuantityPreview{}, err
	}

	preview := QuantityPreview{
		Plan:           adjusted,
		AffectedMeals:  affectedMeals(adjusted, recipeIndex),
		PortionChanges: portionChanges(input.Plan, adjusted, recipeIndex),
		MacroDiff:      macroDiff(before, after),
		QualityDiff:    qualityDiff(before, after),
		ShoppingDiff:   shoppingQuantityDiff(input.Plan, adjusted),
		EaterDays:      snapshots(after),
	}
	preview.Warnings = quantityWarnings(adjusted, recipeIndex)
	preview.Summary = quantitySummary(input.Plan, recipeIndex, fillIndex, request, preview)
	return preview, nil
}

func withAdjustedQuantity(plan planningdom.Plan, recipeIndex, fillIndex int, quantityG float64) planningdom.Plan {
	adjusted := plan
	adjusted.Recipes = append([]planningdom.PlanRecipe(nil), plan.Recipes...)
	fills := append([]planningdom.SlotFill(nil), plan.Recipes[recipeIndex].Fills...)
	fills[fillIndex].PlannedInputQtyG = quantityG
	adjusted.Recipes[recipeIndex].Fills = fills
	return adjusted
}

func withPortions(plan planningdom.Plan, recipeIndex int, portions map[planningdom.MealID]float64) planningdom.Plan {
	adjusted := plan
	adjusted.Meals = append([]planningdom.Meal(nil), plan.Meals...)
	for i := range adjusted.Meals {
		portion, ok := portions[adjusted.Meals[i].ID]
		if !ok {
			continue
		}
		components := append([]planningdom.ComponentAllocation(nil), adjusted.Meals[i].Components...)
		for j := range components {
			if components[j].PlanRecipeIndex != recipeIndex {
				continue
			}
			components[j].PortionFactor = portion
			components[j].PlannedServingG = servingGrams(adjusted.Recipes[recipeIndex], portion)
		}
		adjusted.Meals[i].Components = components
	}
	return adjusted
}

func servingGrams(recipe planningdom.PlanRecipe, portion float64) *float64 {
	if recipe.ExpectedOutputG == nil {
		return nil
	}
	grams := *recipe.ExpectedOutputG * portion
	return &grams
}

// recomputePlannedMacros re-derives every generated meal's planned macros from
// the batches it is served from, so the stored plan cannot drift from its fills.
func recomputePlannedMacros(plan planningdom.Plan, catalogue quality.Catalogue) (planningdom.Plan, error) {
	totals := make([]nutrition.Macros, len(plan.Recipes))
	for i := range plan.Recipes {
		total, err := batchMacros(plan.Recipes[i], catalogue)
		if err != nil {
			return planningdom.Plan{}, err
		}
		totals[i] = total
	}

	updated := plan
	updated.Meals = append([]planningdom.Meal(nil), plan.Meals...)
	for i := range updated.Meals {
		if updated.Meals[i].FixedMealID != nil {
			continue
		}
		planned := nutrition.Macros{}
		for _, component := range updated.Meals[i].Components {
			if component.PlanRecipeIndex < 0 || component.PlanRecipeIndex >= len(totals) {
				return planningdom.Plan{}, fmt.Errorf("meal allocates plan recipe %d outside the plan", component.PlanRecipeIndex)
			}
			share, err := totals[component.PlanRecipeIndex].Scale(component.PortionFactor)
			if err != nil {
				return planningdom.Plan{}, err
			}
			planned, err = planned.Add(share)
			if err != nil {
				return planningdom.Plan{}, err
			}
		}
		updated.Meals[i].Planned = planned
	}
	return updated, nil
}

func batchMacros(recipe planningdom.PlanRecipe, catalogue quality.Catalogue) (nutrition.Macros, error) {
	total := nutrition.Macros{}
	for _, fill := range recipe.Fills {
		product, ok := catalogue.Products[fill.ProductID]
		if !ok {
			return nutrition.Macros{}, fmt.Errorf("plan references unknown product %d", fill.ProductID)
		}
		per100 := nutrition.MacrosPer100G{
			ProteinG: product.ProteinPer100G,
			FatG:     product.FatPer100G,
			CarbG:    product.CarbPer100G,
			Kcal:     product.KcalPer100G,
		}
		share, err := per100.Scale(fill.PlannedInputQtyG)
		if err != nil {
			return nutrition.Macros{}, err
		}
		total, err = total.Add(share)
		if err != nil {
			return nutrition.Macros{}, err
		}
	}
	return total, nil
}

func portionChanges(before, after planningdom.Plan, recipeIndex int) []PortionChange {
	previous := make(map[planningdom.MealID]float64, len(before.Meals))
	for i := range before.Meals {
		for _, component := range before.Meals[i].Components {
			if component.PlanRecipeIndex == recipeIndex {
				previous[before.Meals[i].ID] = component.PortionFactor
			}
		}
	}

	changes := make([]PortionChange, 0)
	for i := range after.Meals {
		meal := &after.Meals[i]
		for _, component := range meal.Components {
			if component.PlanRecipeIndex != recipeIndex {
				continue
			}
			old := previous[meal.ID]
			if math.Abs(old-component.PortionFactor) <= portionEpsilon {
				continue
			}
			changes = append(changes, PortionChange{
				MealID:  meal.ID,
				EaterID: meal.EaterID,
				Date:    meal.Date,
				Before:  old,
				After:   component.PortionFactor,
			})
		}
	}
	return changes
}

func shoppingQuantityDiff(before, after planningdom.Plan) []ShoppingQuantityChange {
	previous := quantitiesByProduct(before)
	current := quantitiesByProduct(after)

	changes := make([]ShoppingQuantityChange, 0)
	for _, productID := range productOrder(after) {
		if math.Abs(previous[productID]-current[productID]) <= portionEpsilon {
			continue
		}
		changes = append(changes, ShoppingQuantityChange{
			ProductID: productID,
			BeforeG:   previous[productID],
			AfterG:    current[productID],
		})
	}
	return changes
}

func quantitiesByProduct(plan planningdom.Plan) map[catalogdom.ProductID]float64 {
	result := make(map[catalogdom.ProductID]float64)
	for i := range plan.Recipes {
		for _, fill := range plan.Recipes[i].Fills {
			result[fill.ProductID] += fill.PlannedInputQtyG
		}
	}
	return result
}

func productOrder(plan planningdom.Plan) []catalogdom.ProductID {
	seen := make(map[catalogdom.ProductID]struct{})
	order := make([]catalogdom.ProductID, 0)
	for i := range plan.Recipes {
		for _, fill := range plan.Recipes[i].Fills {
			if _, exists := seen[fill.ProductID]; exists {
				continue
			}
			seen[fill.ProductID] = struct{}{}
			order = append(order, fill.ProductID)
		}
	}
	return order
}

// quantityWarnings surfaces the leftover the owner will be looking at after
// cooking, because a large reserve is a planning decision, not a silent detail.
func quantityWarnings(plan planningdom.Plan, recipeIndex int) []string {
	allocated := 0.0
	for i := range plan.Meals {
		for _, component := range plan.Meals[i].Components {
			if component.PlanRecipeIndex == recipeIndex {
				allocated += component.PortionFactor
			}
		}
	}
	warnings := make([]string, 0)
	if reserve := 1 - allocated; reserve > portionStep {
		warnings = append(warnings, fmt.Sprintf("%.0f%% of the batch stays unallocated", reserve*percentScale))
	}
	return warnings
}

func quantitySummary(
	plan planningdom.Plan,
	recipeIndex, fillIndex int,
	request QuantityRequest,
	preview QuantityPreview,
) string {
	old := plan.Recipes[recipeIndex].Fills[fillIndex].PlannedInputQtyG
	return fmt.Sprintf(
		"lock %.0f g to %.0f g — %d portion change(s), %d meal(s) affected",
		old, request.PlannedInputQtyG, len(preview.PortionChanges), len(preview.AffectedMeals),
	)
}
