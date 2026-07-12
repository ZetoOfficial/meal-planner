package httpapi

import (
	"context"
	"net/http"

	"github.com/ZetoOfficial/meal-planner/internal/api/openapi"
	catalogdom "github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
	eaterdom "github.com/ZetoOfficial/meal-planner/internal/domain/eater"
	usecasecatalog "github.com/ZetoOfficial/meal-planner/internal/usecase/catalog"
	usecaseeater "github.com/ZetoOfficial/meal-planner/internal/usecase/eater"
)

type CatalogUseCases interface {
	CreateProduct(context.Context, usecasecatalog.CreateProductCommand) (catalogdom.Product, error)
	ListProducts(context.Context, string) ([]catalogdom.Product, error)
	GetProduct(context.Context, catalogdom.ProductID) (catalogdom.Product, error)
	UpdateProduct(context.Context, catalogdom.ProductID, usecasecatalog.UpdateProductCommand) (catalogdom.Product, error)
	DeleteProduct(context.Context, catalogdom.ProductID) error
	ListRoleTags(context.Context) ([]catalogdom.RoleTag, error)
}

type EaterUseCases interface {
	CreateEater(context.Context, usecaseeater.CreateEaterCommand) (eaterdom.Eater, error)
	ListEaters(context.Context) ([]eaterdom.Eater, error)
	GetEater(context.Context, eaterdom.ID) (eaterdom.Eater, error)
	UpdateEater(context.Context, eaterdom.ID, usecaseeater.UpdateEaterCommand) (eaterdom.Eater, error)
	ListGoals(context.Context, eaterdom.ID) ([]eaterdom.MacroGoal, error)
	CreateGoal(context.Context, eaterdom.ID, usecaseeater.MacroGoalCommand) (eaterdom.MacroGoal, error)
	ListFixedMeals(context.Context, eaterdom.ID) ([]eaterdom.FixedMeal, error)
	CreateFixedMeal(context.Context, eaterdom.ID, usecaseeater.CreateFixedMealCommand) (eaterdom.FixedMeal, error)
	ListExclusions(context.Context, eaterdom.ID) ([]eaterdom.Exclusion, error)
	CreateExclusion(context.Context, eaterdom.ID, usecaseeater.CreateExclusionCommand) (eaterdom.Exclusion, error)
	DeleteExclusion(context.Context, eaterdom.ExclusionID) error
}

type api struct {
	catalog CatalogUseCases
	eater   EaterUseCases
}

var _ openapi.ServerInterface = (*api)(nil)

func newAPI(catalog CatalogUseCases, eater EaterUseCases) *api {
	return &api{catalog: catalog, eater: eater}
}

func notImplemented(w http.ResponseWriter) {
	respondJSON(w, http.StatusNotImplemented, openapi.Error{
		Error:   "not_implemented",
		Message: "endpoint is not implemented",
	})
}

func (a *api) GetMacroAnalytics(w http.ResponseWriter, _ *http.Request, _ openapi.GetMacroAnalyticsParams) {
	notImplemented(w)
}

func (a *api) GetNutritionQualityAnalytics(w http.ResponseWriter, _ *http.Request, _ openapi.GetNutritionQualityAnalyticsParams) {
	notImplemented(w)
}

func (a *api) GetTopRecipes(w http.ResponseWriter, _ *http.Request, _ openapi.GetTopRecipesParams) {
	notImplemented(w)
}
func (a *api) CreateFeedback(w http.ResponseWriter, _ *http.Request)   { notImplemented(w) }
func (a *api) LogMeal(w http.ResponseWriter, _ *http.Request, _ int64) { notImplemented(w) }
func (a *api) ListPlans(w http.ResponseWriter, _ *http.Request)        { notImplemented(w) }
func (a *api) GeneratePlan(w http.ResponseWriter, _ *http.Request)     { notImplemented(w) }
func (a *api) ArchivePlan(w http.ResponseWriter, _ *http.Request, _ openapi.PlanId) {
	notImplemented(w)
}
func (a *api) GetPlan(w http.ResponseWriter, _ *http.Request, _ openapi.PlanId) { notImplemented(w) }
func (a *api) ListPlanMeals(w http.ResponseWriter, _ *http.Request, _ openapi.PlanId, _ openapi.ListPlanMealsParams) {
	notImplemented(w)
}

func (a *api) GetShoppingList(w http.ResponseWriter, _ *http.Request, _ openapi.PlanId) {
	notImplemented(w)
}

func (a *api) RegenerateShoppingList(w http.ResponseWriter, _ *http.Request, _ openapi.PlanId) {
	notImplemented(w)
}
func (a *api) ApplySwap(w http.ResponseWriter, _ *http.Request, _ openapi.PlanId) { notImplemented(w) }
func (a *api) PreviewSwap(w http.ResponseWriter, _ *http.Request, _ openapi.PlanId) {
	notImplemented(w)
}

func (a *api) ListRecipes(w http.ResponseWriter, _ *http.Request, _ openapi.ListRecipesParams) {
	notImplemented(w)
}
func (a *api) CreateRecipe(w http.ResponseWriter, _ *http.Request) { notImplemented(w) }
func (a *api) GetRecipe(w http.ResponseWriter, _ *http.Request, _ openapi.RecipeId) {
	notImplemented(w)
}

func (a *api) PromoteRecipe(w http.ResponseWriter, _ *http.Request, _ openapi.RecipeId) {
	notImplemented(w)
}

func (a *api) UpdateShoppingListItem(w http.ResponseWriter, _ *http.Request, _ int64) {
	notImplemented(w)
}
