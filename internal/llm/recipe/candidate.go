package recipe

import (
	"encoding/json"
	"math"
	"strings"

	"github.com/ZetoOfficial/meal-planner/internal/apperr"
	catalogdom "github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
	mealdom "github.com/ZetoOfficial/meal-planner/internal/domain/meal"
	recipedom "github.com/ZetoOfficial/meal-planner/internal/domain/recipe"
)

// parseCandidates turns structured output into draft templates and
// rejects anything the household cannot actually cook.
func parseCandidates(content string, input Input) ([]recipedom.Template, error) {
	var payload candidatePayload
	if err := json.Unmarshal([]byte(content), &payload); err != nil {
		return nil, apperr.Validation("llm returned malformed recipe candidates", err.Error())
	}
	if len(payload.Recipes) == 0 {
		return nil, apperr.Validation("llm returned no recipe candidates")
	}

	roles := codeSet(roleCodes(input.RoleTags))
	equipment := codeSet(input.Equipment)
	mealTypes := codeSet(mealTypeCodes(input.MealTypes))

	templates := make([]recipedom.Template, 0, len(payload.Recipes))
	for i := range payload.Recipes {
		template, err := payload.Recipes[i].toTemplate(roles, equipment, mealTypes)
		if err != nil {
			return nil, err
		}
		templates = append(templates, template)
	}
	return templates, nil
}

func (c candidate) toTemplate(roles, equipment, mealTypes map[string]struct{}) (recipedom.Template, error) {
	name := strings.TrimSpace(c.Name)
	if name == "" {
		return recipedom.Template{}, apperr.Validation("llm recipe candidate has no name")
	}
	kind := recipedom.ComponentKind(c.ComponentKind)
	storage := recipedom.StorageProfile(c.Storage)
	if !kind.Valid() || !storage.Valid() || c.BaseServings <= 0 {
		return recipedom.Template{}, apperr.Validation("llm recipe candidate has an invalid batch contract", name)
	}
	if c.ExpectedOutputG != nil && !positive(*c.ExpectedOutputG) {
		return recipedom.Template{}, apperr.Validation("llm recipe candidate has a non-positive expected output", name)
	}

	allowed, err := candidateMealTypes(name, c.AllowedMealTypes, mealTypes)
	if err != nil {
		return recipedom.Template{}, err
	}
	for _, item := range c.Equipment {
		if _, ok := equipment[item]; !ok {
			return recipedom.Template{}, apperr.Validation("llm recipe candidate needs unavailable equipment", item)
		}
	}
	slots, err := candidateSlots(name, c.Slots, roles)
	if err != nil {
		return recipedom.Template{}, err
	}

	return recipedom.Template{
		Name:             name,
		Status:           recipedom.StatusDraft,
		Source:           recipedom.SourceLLM,
		ComponentKind:    kind,
		AllowedMealTypes: allowed,
		BaseServings:     c.BaseServings,
		ExpectedOutputG:  c.ExpectedOutputG,
		Cuisine:          c.Cuisine,
		CookingMethod:    c.CookingMethod,
		Storage:          storage,
		StorageNotes:     c.StorageNotes,
		Equipment:        append([]string(nil), c.Equipment...),
		Instructions:     c.Instructions,
		Slots:            slots,
	}, nil
}

func candidateMealTypes(name string, values []string, allowed map[string]struct{}) ([]mealdom.Type, error) {
	if len(values) == 0 {
		return nil, apperr.Validation("llm recipe candidate allows no meal type", name)
	}
	result := make([]mealdom.Type, 0, len(values))
	for _, value := range values {
		mealType := mealdom.Type(value)
		if !mealType.Valid() {
			return nil, apperr.Validation("llm recipe candidate uses an unknown meal type", value)
		}
		if _, ok := allowed[value]; !ok {
			return nil, apperr.Validation("llm recipe candidate uses a meal type outside the request", value)
		}
		result = append(result, mealType)
	}
	return result, nil
}

func candidateSlots(name string, slots []candidateSlot, roles map[string]struct{}) ([]recipedom.Slot, error) {
	if len(slots) == 0 {
		return nil, apperr.Validation("llm recipe candidate has no slots", name)
	}
	result := make([]recipedom.Slot, 0, len(slots))
	for position, slot := range slots {
		if _, ok := roles[slot.RoleTagCode]; !ok {
			return nil, apperr.Validation("llm recipe candidate uses an unknown role tag", slot.RoleTagCode)
		}
		if !positive(slot.QtyG) {
			return nil, apperr.Validation("llm recipe slot quantity must be positive", slot.RoleTagCode)
		}
		if slot.PreparedQtyFactor != nil && !positive(*slot.PreparedQtyFactor) {
			return nil, apperr.Validation("llm recipe slot prepared factor must be positive", slot.RoleTagCode)
		}
		result = append(result, recipedom.Slot{
			RoleTagCode:       slot.RoleTagCode,
			Label:             strings.TrimSpace(slot.Label),
			QtyG:              slot.QtyG,
			PreparedQtyFactor: slot.PreparedQtyFactor,
			Position:          position,
			Optional:          slot.Optional,
		})
	}
	return result, nil
}

func roleCodes(roles []catalogdom.RoleTag) []string {
	codes := make([]string, 0, len(roles))
	for _, role := range roles {
		codes = append(codes, role.Code)
	}
	return codes
}

func mealTypeCodes(types []mealdom.Type) []string {
	codes := make([]string, 0, len(types))
	for _, mealType := range types {
		codes = append(codes, string(mealType))
	}
	return codes
}

func codeSet(values []string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}

func positive(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value > 0
}
