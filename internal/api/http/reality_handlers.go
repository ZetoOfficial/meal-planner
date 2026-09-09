package httpapi

import (
	"context"

	"github.com/ZetoOfficial/meal-planner/api/sdk/go/api"
	eaterdom "github.com/ZetoOfficial/meal-planner/internal/domain/eater"
	"github.com/ZetoOfficial/meal-planner/internal/domain/nutrition"
	planningdom "github.com/ZetoOfficial/meal-planner/internal/domain/planning"
	usecasereality "github.com/ZetoOfficial/meal-planner/internal/usecase/reality"
)

func (s *server) ListPlanMeals(ctx context.Context, request api.ListPlanMealsRequestObject) (api.ListPlanMealsResponseObject, error) {
	var eaterID *eaterdom.ID
	if request.Params.EaterId != nil {
		id := eaterdom.ID(*request.Params.EaterId)
		eaterID = &id
	}
	meals, err := s.reality.ListPlanMeals(ctx, planningdom.PlanID(request.PlanId), eaterID)
	if err != nil {
		return nil, err
	}
	return api.ListPlanMeals200JSONResponse(mealsResponse(planningdom.PlanID(request.PlanId), meals)), nil
}

func (s *server) LogMeal(ctx context.Context, request api.LogMealRequestObject) (api.LogMealResponseObject, error) {
	meal, err := s.reality.LogMeal(ctx, planningdom.MealID(request.MealId), mealLogCommand(*request.Body))
	if err != nil {
		return nil, err
	}
	return api.LogMeal200JSONResponse(mealResponseWithPlan(0, meal)), nil
}

func (s *server) CreateFeedback(ctx context.Context, request api.CreateFeedbackRequestObject) (api.CreateFeedbackResponseObject, error) {
	event, err := s.reality.CreateFeedback(ctx, feedbackCommand(*request.Body))
	if err != nil {
		return nil, err
	}
	return api.CreateFeedback201JSONResponse(feedbackResponse(event)), nil
}

func (s *server) GetMacroAnalytics(ctx context.Context, request api.GetMacroAnalyticsRequestObject) (api.GetMacroAnalyticsResponseObject, error) {
	analytics, err := s.reality.MacroAnalytics(ctx, usecasereality.Period{
		EaterID: eaterdom.ID(request.Params.EaterId),
		From:    request.Params.From.Time,
		To:      request.Params.To.Time,
	})
	if err != nil {
		return nil, err
	}
	return api.GetMacroAnalytics200JSONResponse(macroAnalyticsResponse(analytics)), nil
}

func (s *server) GetNutritionQualityAnalytics(
	ctx context.Context,
	request api.GetNutritionQualityAnalyticsRequestObject,
) (api.GetNutritionQualityAnalyticsResponseObject, error) {
	basis := nutrition.SummaryBasisActual
	if request.Params.Basis != nil {
		basis = nutrition.SummaryBasis(*request.Params.Basis)
	}
	analytics, err := s.reality.QualityAnalytics(ctx, usecasereality.Period{
		EaterID: eaterdom.ID(request.Params.EaterId),
		From:    request.Params.From.Time,
		To:      request.Params.To.Time,
	}, basis)
	if err != nil {
		return nil, err
	}
	return qualityAnalyticsResponse(analytics), nil
}

func (s *server) GetTopRecipes(ctx context.Context, request api.GetTopRecipesRequestObject) (api.GetTopRecipesResponseObject, error) {
	limit := 0
	if request.Params.Limit != nil {
		limit = *request.Params.Limit
	}
	recipes, err := s.reality.TopRecipes(ctx, limit)
	if err != nil {
		return nil, err
	}
	return api.GetTopRecipes200JSONResponse(topRecipesResponse(recipes)), nil
}
