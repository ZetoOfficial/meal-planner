package recipe

import (
	"math"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
	"github.com/ZetoOfficial/meal-planner/internal/domain/meal"
	recipedom "github.com/ZetoOfficial/meal-planner/internal/domain/recipe"
	"github.com/ZetoOfficial/meal-planner/internal/postgres/pgvalue"
	"github.com/ZetoOfficial/meal-planner/internal/postgres/sqlc"
)

func templateParams(template recipedom.Template) sqlc.CreateRecipeTemplateParams {
	if template.CookedCount < 0 || int64(template.CookedCount) > math.MaxInt32 {
		panic("recipe cooked count must fit a nonnegative int32")
	}
	if template.BaseServings <= 0 || int64(template.BaseServings) > math.MaxInt32 {
		panic("recipe base servings must fit a positive int32")
	}
	return sqlc.CreateRecipeTemplateParams{
		Name:             template.Name,
		Status:           sqlc.RecipeStatus(template.Status),
		Source:           sqlc.RecipeSource(template.Source),
		ComponentKind:    sqlc.ComponentKind(template.ComponentKind),
		AllowedMealTypes: mealTypesToRows(template.AllowedMealTypes),
		BaseServings:     int32(template.BaseServings),
		ExpectedOutputG:  pgvalue.OptionalNumeric(template.ExpectedOutputG),
		Cuisine:          template.Cuisine,
		CookingMethod:    template.CookingMethod,
		Storage:          sqlc.StorageProfile(template.Storage),
		StorageNotes:     template.StorageNotes,
		Equipment:        template.Equipment,
		Instructions:     template.Instructions,
		Weight:           pgvalue.Numeric(template.Weight),
		CookedCount:      int32(template.CookedCount),
	}
}

func slotParams(templateID int64, slot recipedom.Slot, role catalog.RoleTag) sqlc.CreateRecipeSlotParams {
	if slot.Position < 0 || int64(slot.Position) > math.MaxInt32 {
		panic("recipe slot position must fit a nonnegative int32")
	}
	return sqlc.CreateRecipeSlotParams{
		TemplateID:        templateID,
		RoleTagID:         role.ID,
		Label:             slot.Label,
		QtyG:              pgvalue.Numeric(slot.QtyG),
		PreparedQtyFactor: pgvalue.OptionalNumeric(slot.PreparedQtyFactor),
		Position:          int32(slot.Position),
		Optional:          slot.Optional,
	}
}

func templateFromRow(row sqlc.RecipeTemplate) (recipedom.Template, error) {
	weight, err := pgvalue.Float64(row.Weight)
	if err != nil {
		return recipedom.Template{}, err
	}
	expectedOutput, err := pgvalue.OptionalFloat64(row.ExpectedOutputG)
	if err != nil {
		return recipedom.Template{}, err
	}
	var promotedAt *time.Time
	if row.PromotedAt.Valid {
		value := row.PromotedAt.Time
		promotedAt = &value
	}
	return recipedom.Template{
		ID:               recipedom.ID(row.ID),
		Name:             row.Name,
		Status:           recipedom.Status(row.Status),
		Source:           recipedom.Source(row.Source),
		ComponentKind:    recipedom.ComponentKind(row.ComponentKind),
		AllowedMealTypes: mealTypesFromRows(row.AllowedMealTypes),
		BaseServings:     int(row.BaseServings),
		ExpectedOutputG:  expectedOutput,
		Cuisine:          row.Cuisine,
		CookingMethod:    row.CookingMethod,
		Storage:          recipedom.StorageProfile(row.Storage),
		StorageNotes:     row.StorageNotes,
		Equipment:        append([]string(nil), row.Equipment...),
		Instructions:     row.Instructions,
		Weight:           weight,
		CookedCount:      int(row.CookedCount),
		PromotedAt:       promotedAt,
	}, nil
}

func slotFromRow(row sqlc.ListRecipeSlotsByTemplateRow) (recipedom.Slot, error) {
	quantity, err := pgvalue.Float64(row.QtyG)
	if err != nil {
		return recipedom.Slot{}, err
	}
	preparedFactor, err := pgvalue.OptionalFloat64(row.PreparedQtyFactor)
	if err != nil {
		return recipedom.Slot{}, err
	}
	return recipedom.Slot{
		ID:                recipedom.SlotID(row.ID),
		RoleTagCode:       row.RoleCode,
		Label:             row.Label,
		QtyG:              quantity,
		PreparedQtyFactor: preparedFactor,
		Position:          int(row.Position),
		Optional:          row.Optional,
	}, nil
}

func mealTypesToRows(values []meal.Type) []string {
	result := make([]string, len(values))
	for i, value := range values {
		result[i] = string(value)
	}
	return result
}

func mealTypesFromRows(values []string) []meal.Type {
	result := make([]meal.Type, len(values))
	for i, value := range values {
		result[i] = meal.Type(value)
	}
	return result
}

func roleTagFromRow(row sqlc.RoleTag) catalog.RoleTag {
	return catalog.RoleTag{
		ID:       row.ID,
		Code:     row.Code,
		Kind:     catalog.RoleKind(row.Kind),
		Label:    row.Label,
		ParentID: row.ParentID,
	}
}

func promotedAtValue(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value, Valid: true}
}
