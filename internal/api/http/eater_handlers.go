package httpapi

import (
	"context"

	"github.com/ZetoOfficial/meal-planner/api/sdk/go/api"
	eaterdom "github.com/ZetoOfficial/meal-planner/internal/domain/eater"
)

func (s *server) ListEaters(ctx context.Context, _ api.ListEatersRequestObject) (api.ListEatersResponseObject, error) {
	eaters, err := s.eater.ListEaters(ctx)
	if err != nil {
		return nil, err
	}
	return api.ListEaters200JSONResponse(eatersResponse(eaters)), nil
}

func (s *server) CreateEater(ctx context.Context, request api.CreateEaterRequestObject) (api.CreateEaterResponseObject, error) {
	eater, err := s.eater.CreateEater(ctx, eaterCommand(*request.Body))
	if err != nil {
		return nil, err
	}
	return api.CreateEater201JSONResponse{
		EaterOKJSONResponse: api.EaterOKJSONResponse(eaterResponse(eater)),
	}, nil
}

func (s *server) GetEater(ctx context.Context, request api.GetEaterRequestObject) (api.GetEaterResponseObject, error) {
	eater, err := s.eater.GetEater(ctx, eaterdom.ID(request.EaterId))
	if err != nil {
		return nil, err
	}
	return api.GetEater200JSONResponse{
		EaterOKJSONResponse: api.EaterOKJSONResponse(eaterResponse(eater)),
	}, nil
}

func (s *server) UpdateEater(ctx context.Context, request api.UpdateEaterRequestObject) (api.UpdateEaterResponseObject, error) {
	eater, err := s.eater.UpdateEater(ctx, eaterdom.ID(request.EaterId), eaterCommand(*request.Body))
	if err != nil {
		return nil, err
	}
	return api.UpdateEater200JSONResponse{
		EaterOKJSONResponse: api.EaterOKJSONResponse(eaterResponse(eater)),
	}, nil
}

func (s *server) ListEaterGoals(ctx context.Context, request api.ListEaterGoalsRequestObject) (api.ListEaterGoalsResponseObject, error) {
	goals, err := s.eater.ListGoals(ctx, eaterdom.ID(request.EaterId))
	if err != nil {
		return nil, err
	}
	return api.ListEaterGoals200JSONResponse(macroGoalsResponse(goals)), nil
}

func (s *server) CreateEaterGoal(ctx context.Context, request api.CreateEaterGoalRequestObject) (api.CreateEaterGoalResponseObject, error) {
	goal, err := s.eater.CreateGoal(ctx, eaterdom.ID(request.EaterId), macroGoalCommand(*request.Body))
	if err != nil {
		return nil, err
	}
	return api.CreateEaterGoal201JSONResponse(macroGoalResponse(goal)), nil
}

func (s *server) ListFixedMeals(ctx context.Context, request api.ListFixedMealsRequestObject) (api.ListFixedMealsResponseObject, error) {
	meals, err := s.eater.ListFixedMeals(ctx, eaterdom.ID(request.EaterId))
	if err != nil {
		return nil, err
	}
	return api.ListFixedMeals200JSONResponse(fixedMealsResponse(meals)), nil
}

func (s *server) CreateFixedMeal(ctx context.Context, request api.CreateFixedMealRequestObject) (api.CreateFixedMealResponseObject, error) {
	meal, err := s.eater.CreateFixedMeal(ctx, eaterdom.ID(request.EaterId), fixedMealCommand(*request.Body))
	if err != nil {
		return nil, err
	}
	return api.CreateFixedMeal201JSONResponse(fixedMealResponse(meal)), nil
}

func (s *server) ListExclusions(ctx context.Context, request api.ListExclusionsRequestObject) (api.ListExclusionsResponseObject, error) {
	exclusions, err := s.eater.ListExclusions(ctx, eaterdom.ID(request.EaterId))
	if err != nil {
		return nil, err
	}
	return api.ListExclusions200JSONResponse(exclusionsResponse(exclusions)), nil
}

func (s *server) CreateExclusion(ctx context.Context, request api.CreateExclusionRequestObject) (api.CreateExclusionResponseObject, error) {
	exclusion, err := s.eater.CreateExclusion(ctx, eaterdom.ID(request.EaterId), exclusionCommand(*request.Body))
	if err != nil {
		return nil, err
	}
	return api.CreateExclusion201JSONResponse(exclusionResponse(exclusion)), nil
}

func (s *server) DeleteExclusion(ctx context.Context, request api.DeleteExclusionRequestObject) (api.DeleteExclusionResponseObject, error) {
	if err := s.eater.DeleteExclusion(ctx, eaterdom.ExclusionID(request.ExclusionId)); err != nil {
		return nil, err
	}
	return api.DeleteExclusion204Response{}, nil
}
