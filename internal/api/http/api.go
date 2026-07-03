package httpapi

import (
	"net/http"

	"github.com/ZetoOfficial/meal-planner/internal/api/openapi"
)

var _ openapi.ServerInterface = (*API)(nil)

// API adapts generated OpenAPI handlers to application use cases.
type API struct {
	deps Deps
}

func NewAPI(deps Deps) *API {
	return &API{deps: deps}
}

func (a *API) notImplemented(w http.ResponseWriter) {
	writeJSON(w, http.StatusNotImplemented, map[string]string{
		"error":   "not_implemented",
		"message": "endpoint is not implemented yet",
	})
}

func (a *API) GetMacroAnalytics(w http.ResponseWriter, r *http.Request, params openapi.GetMacroAnalyticsParams) {
	a.notImplemented(w)
}

func (a *API) GetNutritionQualityAnalytics(w http.ResponseWriter, r *http.Request, params openapi.GetNutritionQualityAnalyticsParams) {
	a.notImplemented(w)
}

func (a *API) GetTopRecipes(w http.ResponseWriter, r *http.Request, params openapi.GetTopRecipesParams) {
	a.notImplemented(w)
}

func (a *API) ListEaters(w http.ResponseWriter, r *http.Request) {
	a.notImplemented(w)
}

func (a *API) CreateEater(w http.ResponseWriter, r *http.Request) {
	a.notImplemented(w)
}

func (a *API) GetEater(w http.ResponseWriter, r *http.Request, eaterId openapi.EaterId) {
	a.notImplemented(w)
}

func (a *API) UpdateEater(w http.ResponseWriter, r *http.Request, eaterId openapi.EaterId) {
	a.notImplemented(w)
}

func (a *API) ListExclusions(w http.ResponseWriter, r *http.Request, eaterId openapi.EaterId) {
	a.notImplemented(w)
}

func (a *API) CreateExclusion(w http.ResponseWriter, r *http.Request, eaterId openapi.EaterId) {
	a.notImplemented(w)
}

func (a *API) ListFixedMeals(w http.ResponseWriter, r *http.Request, eaterId openapi.EaterId) {
	a.notImplemented(w)
}

func (a *API) CreateFixedMeal(w http.ResponseWriter, r *http.Request, eaterId openapi.EaterId) {
	a.notImplemented(w)
}

func (a *API) ListEaterGoals(w http.ResponseWriter, r *http.Request, eaterId openapi.EaterId) {
	a.notImplemented(w)
}

func (a *API) CreateEaterGoal(w http.ResponseWriter, r *http.Request, eaterId openapi.EaterId) {
	a.notImplemented(w)
}

func (a *API) DeleteExclusion(w http.ResponseWriter, r *http.Request, exclusionId int64) {
	a.notImplemented(w)
}

func (a *API) CreateFeedback(w http.ResponseWriter, r *http.Request) {
	a.notImplemented(w)
}

func (a *API) LogMeal(w http.ResponseWriter, r *http.Request, mealId int64) {
	a.notImplemented(w)
}

func (a *API) ListPlans(w http.ResponseWriter, r *http.Request) {
	a.notImplemented(w)
}

func (a *API) GeneratePlan(w http.ResponseWriter, r *http.Request) {
	a.notImplemented(w)
}

func (a *API) ArchivePlan(w http.ResponseWriter, r *http.Request, planId openapi.PlanId) {
	a.notImplemented(w)
}

func (a *API) GetPlan(w http.ResponseWriter, r *http.Request, planId openapi.PlanId) {
	a.notImplemented(w)
}

func (a *API) ListPlanMeals(w http.ResponseWriter, r *http.Request, planId openapi.PlanId, params openapi.ListPlanMealsParams) {
	a.notImplemented(w)
}

func (a *API) GetShoppingList(w http.ResponseWriter, r *http.Request, planId openapi.PlanId) {
	a.notImplemented(w)
}

func (a *API) RegenerateShoppingList(w http.ResponseWriter, r *http.Request, planId openapi.PlanId) {
	a.notImplemented(w)
}

func (a *API) ApplySwap(w http.ResponseWriter, r *http.Request, planId openapi.PlanId) {
	a.notImplemented(w)
}

func (a *API) PreviewSwap(w http.ResponseWriter, r *http.Request, planId openapi.PlanId) {
	a.notImplemented(w)
}

func (a *API) ListProducts(w http.ResponseWriter, r *http.Request, params openapi.ListProductsParams) {
	a.notImplemented(w)
}

func (a *API) CreateProduct(w http.ResponseWriter, r *http.Request) {
	a.notImplemented(w)
}

func (a *API) DeleteProduct(w http.ResponseWriter, r *http.Request, productId openapi.ProductId) {
	a.notImplemented(w)
}

func (a *API) GetProduct(w http.ResponseWriter, r *http.Request, productId openapi.ProductId) {
	a.notImplemented(w)
}

func (a *API) UpdateProduct(w http.ResponseWriter, r *http.Request, productId openapi.ProductId) {
	a.notImplemented(w)
}

func (a *API) ListRecipes(w http.ResponseWriter, r *http.Request, params openapi.ListRecipesParams) {
	a.notImplemented(w)
}

func (a *API) CreateRecipe(w http.ResponseWriter, r *http.Request) {
	a.notImplemented(w)
}

func (a *API) GetRecipe(w http.ResponseWriter, r *http.Request, recipeId openapi.RecipeId) {
	a.notImplemented(w)
}

func (a *API) PromoteRecipe(w http.ResponseWriter, r *http.Request, recipeId openapi.RecipeId) {
	a.notImplemented(w)
}

func (a *API) ListRoleTags(w http.ResponseWriter, r *http.Request) {
	a.notImplemented(w)
}

func (a *API) UpdateShoppingListItem(w http.ResponseWriter, r *http.Request, itemId int64) {
	a.notImplemented(w)
}
