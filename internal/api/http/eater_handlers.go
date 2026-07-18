package httpapi

import (
	"context"

	"github.com/ZetoOfficial/meal-planner/internal/api/openapi"
	eaterdom "github.com/ZetoOfficial/meal-planner/internal/domain/eater"
)

func (a *api) ListEaters(ctx context.Context, _ openapi.ListEatersRequestObject) (openapi.ListEatersResponseObject, error) {
	eaters, err := a.eater.ListEaters(ctx)
	if err != nil {
		return nil, err
	}
	return openapi.ListEaters200JSONResponse(eatersResponse(eaters)), nil
}

func (a *api) CreateEater(ctx context.Context, request openapi.CreateEaterRequestObject) (openapi.CreateEaterResponseObject, error) {
	eater, err := a.eater.CreateEater(ctx, eaterCommand(*request.Body))
	if err != nil {
		return nil, err
	}
	return openapi.CreateEater201JSONResponse{
		EaterOKJSONResponse: openapi.EaterOKJSONResponse(eaterResponse(eater)),
	}, nil
}

func (a *api) GetEater(ctx context.Context, request openapi.GetEaterRequestObject) (openapi.GetEaterResponseObject, error) {
	eater, err := a.eater.GetEater(ctx, eaterdom.ID(request.EaterId))
	if err != nil {
		return nil, err
	}
	return openapi.GetEater200JSONResponse{
		EaterOKJSONResponse: openapi.EaterOKJSONResponse(eaterResponse(eater)),
	}, nil
}

func (a *api) UpdateEater(ctx context.Context, request openapi.UpdateEaterRequestObject) (openapi.UpdateEaterResponseObject, error) {
	eater, err := a.eater.UpdateEater(ctx, eaterdom.ID(request.EaterId), eaterCommand(*request.Body))
	if err != nil {
		return nil, err
	}
	return openapi.UpdateEater200JSONResponse{
		EaterOKJSONResponse: openapi.EaterOKJSONResponse(eaterResponse(eater)),
	}, nil
}

func (a *api) ListEaterGoals(ctx context.Context, request openapi.ListEaterGoalsRequestObject) (openapi.ListEaterGoalsResponseObject, error) {
	goals, err := a.eater.ListGoals(ctx, eaterdom.ID(request.EaterId))
	if err != nil {
		return nil, err
	}
	return openapi.ListEaterGoals200JSONResponse(macroGoalsResponse(goals)), nil
}

func (a *api) CreateEaterGoal(ctx context.Context, request openapi.CreateEaterGoalRequestObject) (openapi.CreateEaterGoalResponseObject, error) {
	goal, err := a.eater.CreateGoal(ctx, eaterdom.ID(request.EaterId), macroGoalCommand(*request.Body))
	if err != nil {
		return nil, err
	}
	return openapi.CreateEaterGoal201JSONResponse(macroGoalResponse(goal)), nil
}

func (a *api) ListFixedMeals(ctx context.Context, request openapi.ListFixedMealsRequestObject) (openapi.ListFixedMealsResponseObject, error) {
	meals, err := a.eater.ListFixedMeals(ctx, eaterdom.ID(request.EaterId))
	if err != nil {
		return nil, err
	}
	return openapi.ListFixedMeals200JSONResponse(fixedMealsResponse(meals)), nil
}

func (a *api) CreateFixedMeal(ctx context.Context, request openapi.CreateFixedMealRequestObject) (openapi.CreateFixedMealResponseObject, error) {
	meal, err := a.eater.CreateFixedMeal(ctx, eaterdom.ID(request.EaterId), fixedMealCommand(*request.Body))
	if err != nil {
		return nil, err
	}
	return openapi.CreateFixedMeal201JSONResponse(fixedMealResponse(meal)), nil
}

func (a *api) ListExclusions(ctx context.Context, request openapi.ListExclusionsRequestObject) (openapi.ListExclusionsResponseObject, error) {
	exclusions, err := a.eater.ListExclusions(ctx, eaterdom.ID(request.EaterId))
	if err != nil {
		return nil, err
	}
	return openapi.ListExclusions200JSONResponse(exclusionsResponse(exclusions)), nil
}

func (a *api) CreateExclusion(ctx context.Context, request openapi.CreateExclusionRequestObject) (openapi.CreateExclusionResponseObject, error) {
	exclusion, err := a.eater.CreateExclusion(ctx, eaterdom.ID(request.EaterId), exclusionCommand(*request.Body))
	if err != nil {
		return nil, err
	}
	return openapi.CreateExclusion201JSONResponse(exclusionResponse(exclusion)), nil
}

func (a *api) DeleteExclusion(ctx context.Context, request openapi.DeleteExclusionRequestObject) (openapi.DeleteExclusionResponseObject, error) {
	if err := a.eater.DeleteExclusion(ctx, eaterdom.ExclusionID(request.ExclusionId)); err != nil {
		return nil, err
	}
	return openapi.DeleteExclusion204Response{}, nil
}
