package httpapi

import (
	"github.com/ZetoOfficial/meal-planner/internal/api/openapi"
	recipedom "github.com/ZetoOfficial/meal-planner/internal/domain/recipe"
	usecaserecipe "github.com/ZetoOfficial/meal-planner/internal/usecase/recipe"
)

func recipeCommand(input openapi.RecipeTemplateInput) usecaserecipe.CreateRecipeCommand {
	command := usecaserecipe.CreateRecipeCommand{
		Name:          input.Name,
		Cuisine:       input.Cuisine,
		CookingMethod: input.CookingMethod,
		Storage:       recipedom.StorageProfile(input.Storage),
		StorageNotes:  input.StorageNotes,
		Instructions:  input.Instructions,
		Slots:         make([]usecaserecipe.SlotCommand, len(input.Slots)),
	}
	if input.Equipment != nil {
		command.Equipment = append([]string(nil), (*input.Equipment)...)
	}
	for i, slot := range input.Slots {
		command.Slots[i] = usecaserecipe.SlotCommand{
			RoleTagCode: slot.RoleTag,
			Label:       slot.Label,
			Qty:         float64(slot.Qty),
			Position:    slot.Position,
		}
		if slot.Unit != nil {
			command.Slots[i].Unit = *slot.Unit
		}
		if slot.Optional != nil {
			command.Slots[i].Optional = *slot.Optional
		}
	}
	return command
}

func recipeResponse(value recipedom.Template) openapi.RecipeTemplate {
	source := openapi.RecipeTemplateSource(value.Source)
	weight := float32(value.Weight)
	cookedCount := value.CookedCount
	equipment := append([]string{}, value.Equipment...)
	return openapi.RecipeTemplate{
		Id:            int64(value.ID),
		Name:          value.Name,
		Status:        openapi.RecipeStatus(value.Status),
		Source:        &source,
		Cuisine:       value.Cuisine,
		CookingMethod: value.CookingMethod,
		Storage:       openapi.StorageProfile(value.Storage),
		StorageNotes:  value.StorageNotes,
		Equipment:     &equipment,
		Instructions:  value.Instructions,
		Weight:        &weight,
		CookedCount:   &cookedCount,
		Slots:         slotsResponse(value.Slots),
	}
}

func recipesResponse(values []recipedom.Template) []openapi.RecipeTemplate {
	result := make([]openapi.RecipeTemplate, len(values))
	for i := range values {
		result[i] = recipeResponse(values[i])
	}
	return result
}

func slotsResponse(values []recipedom.Slot) []openapi.Slot {
	result := make([]openapi.Slot, len(values))
	for i, value := range values {
		id := int64(value.ID)
		optional := value.Optional
		result[i] = openapi.Slot{
			Id:       &id,
			RoleTag:  value.RoleTagCode,
			Label:    value.Label,
			Qty:      float32(value.Qty),
			Unit:     value.Unit,
			Position: value.Position,
			Optional: &optional,
		}
	}
	return result
}
