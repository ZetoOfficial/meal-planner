package httpapi

import (
	"github.com/ZetoOfficial/meal-planner/api/sdk/go/model"
	"github.com/ZetoOfficial/meal-planner/internal/domain/meal"
	recipedom "github.com/ZetoOfficial/meal-planner/internal/domain/recipe"
	usecaserecipe "github.com/ZetoOfficial/meal-planner/internal/usecase/recipe"
)

func recipeCommand(input model.RecipeTemplateInput) usecaserecipe.CreateRecipeCommand {
	command := usecaserecipe.CreateRecipeCommand{
		Name:             input.Name,
		ComponentKind:    recipedom.ComponentKind(input.ComponentKind),
		AllowedMealTypes: mealTypesCommand(input.AllowedMealTypes),
		BaseServings:     input.BaseServings,
		ExpectedOutputG:  float64Pointer(input.ExpectedOutputG),
		Cuisine:          input.Cuisine,
		CookingMethod:    input.CookingMethod,
		Storage:          recipedom.StorageProfile(input.Storage),
		StorageNotes:     input.StorageNotes,
		Instructions:     input.Instructions,
		Slots:            make([]usecaserecipe.SlotCommand, len(input.Slots)),
	}
	if input.Equipment != nil {
		command.Equipment = append([]string(nil), (*input.Equipment)...)
	}
	for i, slot := range input.Slots {
		command.Slots[i] = usecaserecipe.SlotCommand{
			RoleTagCode:       slot.RoleTag,
			Label:             slot.Label,
			QtyG:              float64(slot.QtyG),
			PreparedQtyFactor: float64Pointer(slot.PreparedQtyFactor),
			Position:          slot.Position,
		}
		if slot.Optional != nil {
			command.Slots[i].Optional = *slot.Optional
		}
	}
	return command
}

func recipeResponse(value recipedom.Template) model.RecipeTemplate {
	source := model.RecipeTemplateSource(value.Source)
	weight := float32(value.Weight)
	cookedCount := value.CookedCount
	equipment := append([]string{}, value.Equipment...)
	return model.RecipeTemplate{
		Id:               int64(value.ID),
		Name:             value.Name,
		Status:           model.RecipeStatus(value.Status),
		Source:           source,
		ComponentKind:    model.ComponentKind(value.ComponentKind),
		AllowedMealTypes: mealTypesResponse(value.AllowedMealTypes),
		BaseServings:     value.BaseServings,
		ExpectedOutputG:  float32Pointer(value.ExpectedOutputG),
		Cuisine:          value.Cuisine,
		CookingMethod:    value.CookingMethod,
		Storage:          model.StorageProfile(value.Storage),
		StorageNotes:     value.StorageNotes,
		Equipment:        &equipment,
		Instructions:     value.Instructions,
		Weight:           &weight,
		CookedCount:      &cookedCount,
		Slots:            slotsResponse(value.Slots),
	}
}

func recipesResponse(values []recipedom.Template) []model.RecipeTemplate {
	result := make([]model.RecipeTemplate, len(values))
	for i := range values {
		result[i] = recipeResponse(values[i])
	}
	return result
}

func slotsResponse(values []recipedom.Slot) []model.Slot {
	result := make([]model.Slot, len(values))
	for i, value := range values {
		id := int64(value.ID)
		optional := value.Optional
		result[i] = model.Slot{
			Id:                &id,
			RoleTag:           value.RoleTagCode,
			Label:             value.Label,
			QtyG:              float32(value.QtyG),
			PreparedQtyFactor: float32Pointer(value.PreparedQtyFactor),
			Position:          value.Position,
			Optional:          &optional,
		}
	}
	return result
}

func mealTypesCommand(values []model.MealType) []meal.Type {
	result := make([]meal.Type, len(values))
	for i, value := range values {
		result[i] = meal.Type(value)
	}
	return result
}

func mealTypesResponse(values []meal.Type) []model.MealType {
	result := make([]model.MealType, len(values))
	for i, value := range values {
		result[i] = model.MealType(value)
	}
	return result
}
