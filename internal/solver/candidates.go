package solver

import (
	catalogdom "github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
	eaterdom "github.com/ZetoOfficial/meal-planner/internal/domain/eater"
	recipedom "github.com/ZetoOfficial/meal-planner/internal/domain/recipe"
)

type SlotCandidates struct {
	Slot     recipedom.Slot
	Products []catalogdom.Product
}

type RecipeCandidate struct {
	Recipe recipedom.Template
	Slots  []SlotCandidates
}

type EaterCandidates struct {
	EaterID  eaterdom.ID
	Products []catalogdom.Product
	Recipes  []RecipeCandidate
}

type Candidates struct {
	Eaters     []EaterCandidates
	Rejections []HardRejection
}

func FilterCandidates(input Input) Candidates {
	result := Candidates{Eaters: make([]EaterCandidates, 0, len(input.Eaters))}
	for i := range input.Eaters {
		eaterInput := &input.Eaters[i]
		products, rejections := filterProducts(eaterInput.Eater, input.Products)
		recipes, recipeRejections := filterRecipes(
			eaterInput.Eater.ID,
			input.Days,
			input.AvailableEquipment,
			products,
			input.Recipes,
		)

		result.Eaters = append(result.Eaters, EaterCandidates{
			EaterID:  eaterInput.Eater.ID,
			Products: products,
			Recipes:  recipes,
		})
		result.Rejections = append(result.Rejections, rejections...)
		result.Rejections = append(result.Rejections, recipeRejections...)
	}
	return result
}

func filterProducts(eater eaterdom.Eater, products []catalogdom.Product) ([]catalogdom.Product, []HardRejection) {
	accepted := make([]catalogdom.Product, 0, len(products))
	rejections := make([]HardRejection, 0)
	for i := range products {
		product := &products[i]
		if rejection := rejectProduct(eater, *product); rejection != nil {
			rejections = append(rejections, *rejection)
			continue
		}
		accepted = append(accepted, *product)
	}
	return accepted, rejections
}

func filterRecipes(
	eaterID eaterdom.ID,
	days int,
	availableEquipment []string,
	products []catalogdom.Product,
	recipes []recipedom.Template,
) ([]RecipeCandidate, []HardRejection) {
	accepted := make([]RecipeCandidate, 0, len(recipes))
	rejections := make([]HardRejection, 0)
	for i := range recipes {
		recipe := &recipes[i]
		if days > 1 && !recipe.Storage.SupportsMultiDay() {
			rejections = append(rejections, recipeRejection(RejectionStorage, eaterID, recipe.ID, string(recipe.Storage)))
			continue
		}
		if missing := missingEquipment(recipe.Equipment, availableEquipment); missing != "" {
			rejections = append(rejections, recipeRejection(RejectionEquipment, eaterID, recipe.ID, missing))
			continue
		}

		slots, missingRole := candidatesBySlot(recipe.Slots, products)
		if missingRole != "" {
			rejections = append(rejections, recipeRejection(RejectionRoleCompatibility, eaterID, recipe.ID, missingRole))
			continue
		}
		accepted = append(accepted, RecipeCandidate{Recipe: *recipe, Slots: slots})
	}
	return accepted, rejections
}

func candidatesBySlot(slots []recipedom.Slot, products []catalogdom.Product) (result []SlotCandidates, missingRole string) {
	result = make([]SlotCandidates, 0, len(slots))
	for i := range slots {
		slot := &slots[i]
		matches := make([]catalogdom.Product, 0)
		for j := range products {
			product := &products[j]
			if productHasRole(*product, slot.RoleTagCode) {
				matches = append(matches, *product)
			}
		}
		if len(matches) == 0 && !slot.Optional {
			return nil, slot.RoleTagCode
		}
		result = append(result, SlotCandidates{Slot: *slot, Products: matches})
	}
	return result, ""
}

func productHasRole(product catalogdom.Product, roleCode string) bool {
	for _, role := range product.Roles {
		if role.Code == roleCode {
			return true
		}
	}
	return false
}
