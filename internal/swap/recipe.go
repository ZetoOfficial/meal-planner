package swap

import (
	"fmt"
	"sort"

	catalogdom "github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
	eaterdom "github.com/ZetoOfficial/meal-planner/internal/domain/eater"
	planningdom "github.com/ZetoOfficial/meal-planner/internal/domain/planning"
	recipedom "github.com/ZetoOfficial/meal-planner/internal/domain/recipe"
	"github.com/ZetoOfficial/meal-planner/internal/quality"
	"github.com/ZetoOfficial/meal-planner/internal/solver"
)

// RecipeRequest replaces the template of one batch. The batch keeps the number
// of servings and the shares its meals already hold; only what is cooked
// changes.
type RecipeRequest struct {
	PlanRecipeID  planningdom.PlanRecipeID
	NewTemplateID recipedom.ID
}

// ComputeRecipe previews a dish-for-dish swap. It refuses a template the plan
// cannot host (wrong meal type, storage, equipment, or a slot nobody at the
// table can eat) instead of quietly producing an unsafe plan.
func ComputeRecipe(input Input, request RecipeRequest) (Preview, error) {
	recipeIndex := locateRecipe(input.Plan, request.PlanRecipeID)
	if recipeIndex < 0 {
		return Preview{}, ErrUnknownRecipe
	}
	template, ok := input.Catalogue.Templates[request.NewTemplateID]
	if !ok {
		return Preview{}, fmt.Errorf("%w: %d", ErrUnknownTemplate, request.NewTemplateID)
	}
	if template.Status != recipedom.StatusPermanent {
		// Generation only cooks from the permanent library; a manual swap must
		// not bring in a draft or a rejected recipe.
		return Preview{}, &IncompatibleError{Reason: templateName(template) + " is not a permanent recipe"}
	}
	current := input.Plan.Recipes[recipeIndex]
	oldTemplate, ok := input.Catalogue.Templates[current.TemplateID]
	if !ok || oldTemplate.BaseServings <= 0 {
		return Preview{}, fmt.Errorf("plan recipe %d has no usable template %d", current.ID, current.TemplateID)
	}
	if err := checkTemplate(input, recipeIndex, template); err != nil {
		return Preview{}, err
	}

	fills, err := fillTemplate(input, recipeIndex, template)
	if err != nil {
		return Preview{}, err
	}
	servings := current.ScaleFactor * float64(oldTemplate.BaseServings)
	swapped, err := recomputePlannedMacros(
		withSwappedRecipe(input, recipeIndex, template, fills, servings/float64(template.BaseServings)),
		input.Catalogue,
	)
	if err != nil {
		return Preview{}, err
	}

	preview, err := diffPlans(input, swapped, recipeIndex)
	if err != nil {
		return Preview{}, err
	}
	preview.Level = LevelRecipeChange
	preview.Summary = summarise(templateName(oldTemplate), templateName(template), preview)
	return preview, nil
}

func locateRecipe(plan planningdom.Plan, id planningdom.PlanRecipeID) int {
	for i := range plan.Recipes {
		if plan.Recipes[i].ID == id {
			return i
		}
	}
	return -1
}

// checkTemplate applies the template-level hard constraints: HC-3 storage,
// HC-4 equipment and the meal types the batch is already served in.
func checkTemplate(input Input, recipeIndex int, template recipedom.Template) error {
	if template.BaseServings <= 0 || len(template.Slots) == 0 {
		return &IncompatibleError{Reason: templateName(template) + " has no servings or slots"}
	}
	if input.Plan.Days > 1 && !template.Storage.SupportsMultiDay() {
		return &IncompatibleError{Reason: fmt.Sprintf(
			"%s keeps as %s and cannot feed a %d-day plan", templateName(template), template.Storage, input.Plan.Days,
		)}
	}
	if input.AvailableEquipment != nil {
		available := make(map[string]struct{}, len(input.AvailableEquipment))
		for _, item := range input.AvailableEquipment {
			available[item] = struct{}{}
		}
		for _, item := range template.Equipment {
			if _, ok := available[item]; !ok {
				return &IncompatibleError{Reason: fmt.Sprintf("%s needs %s", templateName(template), item)}
			}
		}
	}
	for i := range input.Plan.Meals {
		meal := &input.Plan.Meals[i]
		for _, component := range meal.Components {
			if component.PlanRecipeIndex == recipeIndex && !allowsMealType(template, meal) {
				return &IncompatibleError{Reason: fmt.Sprintf(
					"%s is not allowed for %s", templateName(template), meal.Type,
				)}
			}
		}
	}
	return nil
}

func allowsMealType(template recipedom.Template, meal *planningdom.Meal) bool {
	for _, allowed := range template.AllowedMealTypes {
		if allowed == meal.Type {
			return true
		}
	}
	return false
}

// fillTemplate picks one product per required slot. The choice is
// deterministic and conservative: a product safe for everyone who eats the
// batch, preferring one the plan already buys, then the lowest id. Optional
// slots stay empty; the owner can fill them with a fill-level swap.
func fillTemplate(input Input, recipeIndex int, template recipedom.Template) ([]planningdom.SlotFill, error) {
	eaters := batchEaters(input.Plan, recipeIndex)
	bought := quantitiesByProduct(input.Plan)

	products := make([]catalogdom.Product, 0, len(input.Catalogue.Products))
	for id := range input.Catalogue.Products {
		products = append(products, input.Catalogue.Products[id])
	}
	sort.Slice(products, func(i, j int) bool {
		_, leftBought := bought[products[i].ID]
		_, rightBought := bought[products[j].ID]
		if leftBought != rightBought {
			return leftBought
		}
		return products[i].ID < products[j].ID
	})

	fills := make([]planningdom.SlotFill, 0, len(template.Slots))
	for _, slot := range template.Slots {
		if slot.Optional {
			continue
		}
		product, found := firstSafeProduct(input, eaters, products, slot.RoleTagCode)
		if !found {
			return nil, &IncompatibleError{Reason: fmt.Sprintf(
				"no product with role %s is safe for everyone who eats this batch", slot.RoleTagCode,
			)}
		}
		fills = append(fills, planningdom.SlotFill{SlotID: slot.ID, ProductID: product, PlannedInputQtyG: slot.QtyG})
	}
	if len(fills) == 0 {
		return nil, &IncompatibleError{Reason: templateName(template) + " has no required slot to fill"}
	}
	return fills, nil
}

func firstSafeProduct(
	input Input,
	eaters []eaterdom.ID,
	products []catalogdom.Product,
	roleCode string,
) (catalogdom.ProductID, bool) {
	for i := range products {
		if !hasRole(products[i], roleCode) {
			continue
		}
		safe := true
		for _, eaterID := range eaters {
			eater, known := input.Catalogue.Eaters[eaterID]
			if known && solver.RejectProduct(eater, products[i]) != nil {
				safe = false
				break
			}
		}
		if safe {
			return products[i].ID, true
		}
	}
	return 0, false
}

// withSwappedRecipe replaces one batch in a copy of the plan and rebuilds the
// prep graph, because what depends on what follows from the templates.
func withSwappedRecipe(
	input Input,
	recipeIndex int,
	template recipedom.Template,
	fills []planningdom.SlotFill,
	scale float64,
) planningdom.Plan {
	swapped := input.Plan
	swapped.Recipes = append([]planningdom.PlanRecipe(nil), input.Plan.Recipes...)
	recipe := &swapped.Recipes[recipeIndex]
	recipe.TemplateID = template.ID
	recipe.Label = nil
	recipe.ScaleFactor = scale
	recipe.ExpectedOutputG = nil
	if template.ExpectedOutputG != nil {
		output := *template.ExpectedOutputG * scale
		recipe.ExpectedOutputG = &output
	}
	for i := range fills {
		fills[i].PlannedInputQtyG *= scale
	}
	recipe.Fills = fills

	swapped.Meals = append([]planningdom.Meal(nil), input.Plan.Meals...)
	for i := range swapped.Meals {
		components := append([]planningdom.ComponentAllocation(nil), swapped.Meals[i].Components...)
		for j := range components {
			if components[j].PlanRecipeIndex == recipeIndex {
				components[j].PlannedServingG = servingGrams(*recipe, components[j].PortionFactor)
			}
		}
		swapped.Meals[i].Components = components
	}
	swapped.PrepDependencies = planningdom.DerivePrepDependencies(swapped, storageOf(input.Catalogue))
	return swapped
}

func storageOf(catalogue quality.Catalogue) func(recipedom.ID) (recipedom.StorageProfile, bool) {
	return func(id recipedom.ID) (recipedom.StorageProfile, bool) {
		template, ok := catalogue.Templates[id]
		return template.Storage, ok
	}
}

func templateName(template recipedom.Template) string {
	if template.Name != "" {
		return template.Name
	}
	return fmt.Sprintf("recipe %d", template.ID)
}

// ComputeRotation previews replacing the whole plan with another one built for
// the same eaters and days. Every generated meal changes, so the radius is the
// plan itself; the diff says what the new week costs and gains.
func ComputeRotation(input Input, replacement planningdom.Plan) (Preview, error) {
	before, err := evaluatePlan(input, input.Plan)
	if err != nil {
		return Preview{}, err
	}
	after, err := evaluatePlan(input, replacement)
	if err != nil {
		return Preview{}, err
	}

	affected := make([]planningdom.MealID, 0, len(input.Plan.Meals))
	for i := range input.Plan.Meals {
		if input.Plan.Meals[i].FixedMealID == nil {
			affected = append(affected, input.Plan.Meals[i].ID)
		}
	}
	preview := Preview{
		Level:              LevelFullRotation,
		MacroDiff:          macroDiff(before, after),
		AffectedMeals:      affected,
		InvalidatedRecipes: make([]InvalidatedRecipe, 0),
		ShoppingDelta:      shoppingDelta(input.Plan, replacement),
		QualityDiff:        qualityDiff(before, after),
		Plan:               replacement,
		EaterDays:          snapshots(after),
		MacroGoalBreaches:  macroGoalBreaches(input.Goals, before, after),
	}
	preview.Summary = fmt.Sprintf(
		"new rotation — %d batch(es) instead of %d, %d product(s) added to the shopping list, %d removed, %d rule verdict(s) move%s",
		len(replacement.Recipes), len(input.Plan.Recipes),
		len(preview.ShoppingDelta.Added), len(preview.ShoppingDelta.Removed), len(preview.QualityDiff.ChangedRules),
		breachNote(preview.MacroGoalBreaches),
	)
	return preview, nil
}
