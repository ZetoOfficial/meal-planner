package httpapi

import (
	"context"
	"errors"

	"github.com/ZetoOfficial/meal-planner/api/sdk/go/api"
	"github.com/ZetoOfficial/meal-planner/api/sdk/go/model"
	"github.com/ZetoOfficial/meal-planner/internal/apperr"
	catalogdom "github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
	planningdom "github.com/ZetoOfficial/meal-planner/internal/domain/planning"
	usecaseplanning "github.com/ZetoOfficial/meal-planner/internal/usecase/planning"
)

func (s *server) PreviewSwap(ctx context.Context, request api.PreviewSwapRequestObject) (api.PreviewSwapResponseObject, error) {
	command, err := swapCommand(request.Body.Level, request.Body.SlotFillId, request.Body.NewProductId)
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
	command, err := swapCommand(
		model.SwapInputLevel(request.Body.Level),
		request.Body.SlotFillId,
		request.Body.NewProductId,
	)
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

// swapCommand accepts only fill-level swaps: recipe and rotation swaps need a
// full re-solve and are not part of v1.
func swapCommand(
	level model.SwapInputLevel,
	slotFillID, newProductID *int64,
) (usecaseplanning.SwapCommand, error) {
	if level != model.Fill {
		return usecaseplanning.SwapCommand{}, apperr.Validation(
			"only fill-level swaps are supported", string(level),
		)
	}
	if slotFillID == nil || newProductID == nil {
		return usecaseplanning.SwapCommand{}, apperr.Validation(
			"fill-level swaps need slot_fill_id and new_product_id", "level",
		)
	}
	return usecaseplanning.SwapCommand{
		SlotFillID:   planningdom.SlotFillID(*slotFillID),
		NewProductID: catalogdom.ProductID(*newProductID),
	}, nil
}
