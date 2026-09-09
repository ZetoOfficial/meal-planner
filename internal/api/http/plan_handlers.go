package httpapi

import (
	"context"
	"errors"

	"github.com/ZetoOfficial/meal-planner/api/sdk/go/api"
	planningdom "github.com/ZetoOfficial/meal-planner/internal/domain/planning"
	usecaseplanning "github.com/ZetoOfficial/meal-planner/internal/usecase/planning"
)

func (s *server) ListPlans(ctx context.Context, _ api.ListPlansRequestObject) (api.ListPlansResponseObject, error) {
	plans, err := s.planning.ListPlans(ctx)
	if err != nil {
		return nil, err
	}
	return api.ListPlans200JSONResponse(planSummariesResponse(plans)), nil
}

// GeneratePlan answers a proven impossibility with 422 diagnostics instead of a
// generic error, so the owner learns which constraint blocked the plan.
func (s *server) GeneratePlan(ctx context.Context, request api.GeneratePlanRequestObject) (api.GeneratePlanResponseObject, error) {
	result, err := s.planning.GeneratePlan(ctx, generatePlanCommand(*request.Body))
	var infeasible *usecaseplanning.InfeasibleError
	if errors.As(err, &infeasible) {
		return api.GeneratePlan422JSONResponse(solverErrorResponse(infeasible.Failure)), nil
	}
	if err != nil {
		return nil, err
	}
	return api.GeneratePlan201JSONResponse{
		PlanOKJSONResponse: api.PlanOKJSONResponse(planResponse(result)),
	}, nil
}

func (s *server) GetPlan(ctx context.Context, request api.GetPlanRequestObject) (api.GetPlanResponseObject, error) {
	result, err := s.planning.GetPlan(ctx, planningdom.PlanID(request.PlanId))
	if err != nil {
		return nil, err
	}
	return api.GetPlan200JSONResponse{
		PlanOKJSONResponse: api.PlanOKJSONResponse(planResponse(result)),
	}, nil
}

func (s *server) ArchivePlan(ctx context.Context, request api.ArchivePlanRequestObject) (api.ArchivePlanResponseObject, error) {
	if err := s.planning.ArchivePlan(ctx, planningdom.PlanID(request.PlanId)); err != nil {
		return nil, err
	}
	return api.ArchivePlan204Response{}, nil
}
