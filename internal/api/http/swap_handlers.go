package httpapi

import (
	"context"
	"errors"

	"github.com/ZetoOfficial/meal-planner/api/sdk/go/api"
	"github.com/ZetoOfficial/meal-planner/api/sdk/go/model"
	"github.com/ZetoOfficial/meal-planner/internal/apperr"
	catalogdom "github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
	planningdom "github.com/ZetoOfficial/meal-planner/internal/domain/planning"
	recipedom "github.com/ZetoOfficial/meal-planner/internal/domain/recipe"
	usecaseplanning "github.com/ZetoOfficial/meal-planner/internal/usecase/planning"
)

func (s *server) PreviewSwap(ctx context.Context, request api.PreviewSwapRequestObject) (api.PreviewSwapResponseObject, error) {
	command, err := swapCommand(*request.Body)
	if err != nil {
		return nil, err
	}
	preview, err := s.planning.PreviewSwap(ctx, planningdom.PlanID(request.PlanId), command)
	if err != nil {
		return nil, err
	}
	return api.PreviewSwap200JSONResponse(swapPreviewResponse(preview)), nil
}

// ApplySwap answers 409 with the same preview when the owner has not confirmed,
// so the client can show exactly what it is about to accept.
func (s *server) ApplySwap(ctx context.Context, request api.ApplySwapRequestObject) (api.ApplySwapResponseObject, error) {
	command, err := swapCommand(model.SwapInput{
		Level:         model.SwapInputLevel(request.Body.Level),
		SlotFillId:    request.Body.SlotFillId,
		NewProductId:  request.Body.NewProductId,
		PlanRecipeId:  request.Body.PlanRecipeId,
		NewTemplateId: request.Body.NewTemplateId,
	})
	if err != nil {
		return nil, err
	}

	result, err := s.planning.ApplySwap(ctx, planningdom.PlanID(request.PlanId), command, request.Body.Confirm)
	var unconfirmed *usecaseplanning.UnconfirmedSwapError
	if errors.As(err, &unconfirmed) {
		return api.ApplySwap409JSONResponse(swapPreviewResponse(unconfirmed.Preview)), nil
	}
	if err != nil {
		return nil, err
	}
	return api.ApplySwap200JSONResponse(planResponse(result)), nil
}

// swapCommand checks that the request carries the ids its level needs.
func swapCommand(body model.SwapInput) (usecaseplanning.SwapCommand, error) {
	switch body.Level {
	case model.SwapInputLevelFill:
		if body.SlotFillId == nil || body.NewProductId == nil {
			return usecaseplanning.SwapCommand{}, apperr.Validation(
				"fill-level swaps need slot_fill_id and new_product_id", "level",
			)
		}
		return usecaseplanning.SwapCommand{
			Level:        planningdom.SwapFill,
			SlotFillID:   planningdom.SlotFillID(*body.SlotFillId),
			NewProductID: catalogdom.ProductID(*body.NewProductId),
		}, nil
	case model.SwapInputLevelRecipe:
		if body.PlanRecipeId == nil || body.NewTemplateId == nil {
			return usecaseplanning.SwapCommand{}, apperr.Validation(
				"recipe-level swaps need plan_recipe_id and new_template_id", "level",
			)
		}
		return usecaseplanning.SwapCommand{
			Level:         planningdom.SwapRecipe,
			PlanRecipeID:  planningdom.PlanRecipeID(*body.PlanRecipeId),
			NewTemplateID: recipedom.ID(*body.NewTemplateId),
		}, nil
	case model.SwapInputLevelRotation:
		return usecaseplanning.SwapCommand{Level: planningdom.SwapRotation}, nil
	default:
		return usecaseplanning.SwapCommand{}, apperr.Validation("level must be fill, recipe or rotation", string(body.Level))
	}
}
