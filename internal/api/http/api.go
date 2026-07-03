package httpapi

import (
	"context"

	api "github.com/ZetoOfficial/meal-planner/internal/api/openapi"
)

var _ api.StrictServerInterface = (*API)(nil)

// API adapts generated OpenAPI handlers to application use cases.
type API struct {
	deps Deps
}

func NewAPI(deps Deps) *API {
	return &API{deps: deps}
}

func notImplemented() api.NotImplementedJSONResponse {
	return api.NotImplementedJSONResponse{
		Error:   "not_implemented",
		Message: "endpoint is not implemented yet",
	}
}

func (a *API) GetMacroAnalytics(ctx context.Context, request api.GetMacroAnalyticsRequestObject) (api.GetMacroAnalyticsResponseObject, error) {
	return api.GetMacroAnalytics501JSONResponse{NotImplementedJSONResponse: notImplemented()}, nil
}

func (a *API) GetNutritionQualityAnalytics(ctx context.Context, request api.GetNutritionQualityAnalyticsRequestObject) (api.GetNutritionQualityAnalyticsResponseObject, error) {
	return api.GetNutritionQualityAnalytics501JSONResponse{NotImplementedJSONResponse: notImplemented()}, nil
}

func (a *API) GetTopRecipes(ctx context.Context, request api.GetTopRecipesRequestObject) (api.GetTopRecipesResponseObject, error) {
	return api.GetTopRecipes501JSONResponse{NotImplementedJSONResponse: notImplemented()}, nil
}

func (a *API) ListEaters(ctx context.Context, request api.ListEatersRequestObject) (api.ListEatersResponseObject, error) {
	return api.ListEaters501JSONResponse{NotImplementedJSONResponse: notImplemented()}, nil
}

func (a *API) CreateEater(ctx context.Context, request api.CreateEaterRequestObject) (api.CreateEaterResponseObject, error) {
	return api.CreateEater501JSONResponse{NotImplementedJSONResponse: notImplemented()}, nil
}

func (a *API) GetEater(ctx context.Context, request api.GetEaterRequestObject) (api.GetEaterResponseObject, error) {
	return api.GetEater501JSONResponse{NotImplementedJSONResponse: notImplemented()}, nil
}

func (a *API) UpdateEater(ctx context.Context, request api.UpdateEaterRequestObject) (api.UpdateEaterResponseObject, error) {
	return api.UpdateEater501JSONResponse{NotImplementedJSONResponse: notImplemented()}, nil
}

func (a *API) ListExclusions(ctx context.Context, request api.ListExclusionsRequestObject) (api.ListExclusionsResponseObject, error) {
	return api.ListExclusions501JSONResponse{NotImplementedJSONResponse: notImplemented()}, nil
}

func (a *API) CreateExclusion(ctx context.Context, request api.CreateExclusionRequestObject) (api.CreateExclusionResponseObject, error) {
	return api.CreateExclusion501JSONResponse{NotImplementedJSONResponse: notImplemented()}, nil
}

func (a *API) ListFixedMeals(ctx context.Context, request api.ListFixedMealsRequestObject) (api.ListFixedMealsResponseObject, error) {
	return api.ListFixedMeals501JSONResponse{NotImplementedJSONResponse: notImplemented()}, nil
}

func (a *API) CreateFixedMeal(ctx context.Context, request api.CreateFixedMealRequestObject) (api.CreateFixedMealResponseObject, error) {
	return api.CreateFixedMeal501JSONResponse{NotImplementedJSONResponse: notImplemented()}, nil
}

func (a *API) ListEaterGoals(ctx context.Context, request api.ListEaterGoalsRequestObject) (api.ListEaterGoalsResponseObject, error) {
	return api.ListEaterGoals501JSONResponse{NotImplementedJSONResponse: notImplemented()}, nil
}

func (a *API) CreateEaterGoal(ctx context.Context, request api.CreateEaterGoalRequestObject) (api.CreateEaterGoalResponseObject, error) {
	return api.CreateEaterGoal501JSONResponse{NotImplementedJSONResponse: notImplemented()}, nil
}

func (a *API) DeleteExclusion(ctx context.Context, request api.DeleteExclusionRequestObject) (api.DeleteExclusionResponseObject, error) {
	return api.DeleteExclusion501JSONResponse{NotImplementedJSONResponse: notImplemented()}, nil
}

func (a *API) CreateFeedback(ctx context.Context, request api.CreateFeedbackRequestObject) (api.CreateFeedbackResponseObject, error) {
	return api.CreateFeedback501JSONResponse{NotImplementedJSONResponse: notImplemented()}, nil
}

func (a *API) LogMeal(ctx context.Context, request api.LogMealRequestObject) (api.LogMealResponseObject, error) {
	return api.LogMeal501JSONResponse{NotImplementedJSONResponse: notImplemented()}, nil
}

func (a *API) ListPlans(ctx context.Context, request api.ListPlansRequestObject) (api.ListPlansResponseObject, error) {
	return api.ListPlans501JSONResponse{NotImplementedJSONResponse: notImplemented()}, nil
}

func (a *API) GeneratePlan(ctx context.Context, request api.GeneratePlanRequestObject) (api.GeneratePlanResponseObject, error) {
	return api.GeneratePlan501JSONResponse{NotImplementedJSONResponse: notImplemented()}, nil
}

func (a *API) ArchivePlan(ctx context.Context, request api.ArchivePlanRequestObject) (api.ArchivePlanResponseObject, error) {
	return api.ArchivePlan501JSONResponse{NotImplementedJSONResponse: notImplemented()}, nil
}

func (a *API) GetPlan(ctx context.Context, request api.GetPlanRequestObject) (api.GetPlanResponseObject, error) {
	return api.GetPlan501JSONResponse{NotImplementedJSONResponse: notImplemented()}, nil
}

func (a *API) ListPlanMeals(ctx context.Context, request api.ListPlanMealsRequestObject) (api.ListPlanMealsResponseObject, error) {
	return api.ListPlanMeals501JSONResponse{NotImplementedJSONResponse: notImplemented()}, nil
}

func (a *API) GetShoppingList(ctx context.Context, request api.GetShoppingListRequestObject) (api.GetShoppingListResponseObject, error) {
	return api.GetShoppingList501JSONResponse{NotImplementedJSONResponse: notImplemented()}, nil
}

func (a *API) RegenerateShoppingList(ctx context.Context, request api.RegenerateShoppingListRequestObject) (api.RegenerateShoppingListResponseObject, error) {
	return api.RegenerateShoppingList501JSONResponse{NotImplementedJSONResponse: notImplemented()}, nil
}

func (a *API) ApplySwap(ctx context.Context, request api.ApplySwapRequestObject) (api.ApplySwapResponseObject, error) {
	return api.ApplySwap501JSONResponse{NotImplementedJSONResponse: notImplemented()}, nil
}

func (a *API) PreviewSwap(ctx context.Context, request api.PreviewSwapRequestObject) (api.PreviewSwapResponseObject, error) {
	return api.PreviewSwap501JSONResponse{NotImplementedJSONResponse: notImplemented()}, nil
}

func (a *API) ListProducts(ctx context.Context, request api.ListProductsRequestObject) (api.ListProductsResponseObject, error) {
	return api.ListProducts501JSONResponse{NotImplementedJSONResponse: notImplemented()}, nil
}

func (a *API) CreateProduct(ctx context.Context, request api.CreateProductRequestObject) (api.CreateProductResponseObject, error) {
	return api.CreateProduct501JSONResponse{NotImplementedJSONResponse: notImplemented()}, nil
}

func (a *API) DeleteProduct(ctx context.Context, request api.DeleteProductRequestObject) (api.DeleteProductResponseObject, error) {
	return api.DeleteProduct501JSONResponse{NotImplementedJSONResponse: notImplemented()}, nil
}

func (a *API) GetProduct(ctx context.Context, request api.GetProductRequestObject) (api.GetProductResponseObject, error) {
	return api.GetProduct501JSONResponse{NotImplementedJSONResponse: notImplemented()}, nil
}

func (a *API) UpdateProduct(ctx context.Context, request api.UpdateProductRequestObject) (api.UpdateProductResponseObject, error) {
	return api.UpdateProduct501JSONResponse{NotImplementedJSONResponse: notImplemented()}, nil
}

func (a *API) ListRecipes(ctx context.Context, request api.ListRecipesRequestObject) (api.ListRecipesResponseObject, error) {
	return api.ListRecipes501JSONResponse{NotImplementedJSONResponse: notImplemented()}, nil
}

func (a *API) CreateRecipe(ctx context.Context, request api.CreateRecipeRequestObject) (api.CreateRecipeResponseObject, error) {
	return api.CreateRecipe501JSONResponse{NotImplementedJSONResponse: notImplemented()}, nil
}

func (a *API) GetRecipe(ctx context.Context, request api.GetRecipeRequestObject) (api.GetRecipeResponseObject, error) {
	return api.GetRecipe501JSONResponse{NotImplementedJSONResponse: notImplemented()}, nil
}

func (a *API) PromoteRecipe(ctx context.Context, request api.PromoteRecipeRequestObject) (api.PromoteRecipeResponseObject, error) {
	return api.PromoteRecipe501JSONResponse{NotImplementedJSONResponse: notImplemented()}, nil
}

func (a *API) ListRoleTags(ctx context.Context, request api.ListRoleTagsRequestObject) (api.ListRoleTagsResponseObject, error) {
	return api.ListRoleTags501JSONResponse{NotImplementedJSONResponse: notImplemented()}, nil
}

func (a *API) UpdateShoppingListItem(ctx context.Context, request api.UpdateShoppingListItemRequestObject) (api.UpdateShoppingListItemResponseObject, error) {
	return api.UpdateShoppingListItem501JSONResponse{NotImplementedJSONResponse: notImplemented()}, nil
}
