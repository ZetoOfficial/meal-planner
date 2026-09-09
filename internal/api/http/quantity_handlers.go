package httpapi

import (
	"context"
	"errors"

	"github.com/ZetoOfficial/meal-planner/api/sdk/go/api"
	planningdom "github.com/ZetoOfficial/meal-planner/internal/domain/planning"
	"github.com/ZetoOfficial/meal-planner/internal/swap"
	usecaseplanning "github.com/ZetoOfficial/meal-planner/internal/usecase/planning"
)

func (s *server) PreviewQuantityAdjustment(
	ctx context.Context,
	request api.PreviewQuantityAdjustmentRequestObject,
) (api.PreviewQuantityAdjustmentResponseObject, error) {
	preview, err := s.planning.PreviewQuantityAdjustment(
		ctx, planningdom.PlanID(request.PlanId), quantityCommand(*request.Body),
	)
	var infeasible *swap.InfeasibleQuantityError
	if errors.As(err, &infeasible) {
		return api.PreviewQuantityAdjustment422JSONResponse(quantityInfeasibleResponse(infeasible)), nil
	}
	if err != nil {
		return nil, err
	}
	return api.PreviewQuantityAdjustment200JSONResponse(quantityPreviewResponse(preview)), nil
}

// ApplyQuantityAdjustment answers 409 with the same preview until the owner
// confirms, so nothing is rewritten before the blast radius has been seen.
func (s *server) ApplyQuantityAdjustment(
	ctx context.Context,
	request api.ApplyQuantityAdjustmentRequestObject,
) (api.ApplyQuantityAdjustmentResponseObject, error) {
	confirmed := request.Body.Confirm != nil && *request.Body.Confirm
	result, err := s.planning.ApplyQuantityAdjustment(
		ctx, planningdom.PlanID(request.PlanId), quantityCommand(*request.Body), confirmed,
	)

	var unconfirmed *usecaseplanning.UnconfirmedQuantityError
	if errors.As(err, &unconfirmed) {
		return api.ApplyQuantityAdjustment409JSONResponse(quantityPreviewResponse(unconfirmed.Preview)), nil
	}
	var infeasible *swap.InfeasibleQuantityError
	if errors.As(err, &infeasible) {
		return api.ApplyQuantityAdjustment422JSONResponse(quantityInfeasibleResponse(infeasible)), nil
	}
	if err != nil {
		return nil, err
	}
	return api.ApplyQuantityAdjustment200JSONResponse(planResponse(result)), nil
}

func quantityCommand(body api.PreviewQuantityAdjustmentJSONRequestBody) usecaseplanning.QuantityCommand {
	return usecaseplanning.QuantityCommand{
		SlotFillID:       planningdom.SlotFillID(body.SlotFillId),
		PlannedInputQtyG: float64(body.PlannedInputQtyG),
	}
}
