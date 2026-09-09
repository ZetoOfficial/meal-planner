package httpapi

//go:generate go tool mockgen -source=$GOFILE -destination=mocks/usecases.go -package=mocks

import (
	"context"

	catalogdom "github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
	eaterdom "github.com/ZetoOfficial/meal-planner/internal/domain/eater"
	planningdom "github.com/ZetoOfficial/meal-planner/internal/domain/planning"
	recipedom "github.com/ZetoOfficial/meal-planner/internal/domain/recipe"
	usecasecatalog "github.com/ZetoOfficial/meal-planner/internal/usecase/catalog"
	usecaseeater "github.com/ZetoOfficial/meal-planner/internal/usecase/eater"
	usecaseplanning "github.com/ZetoOfficial/meal-planner/internal/usecase/planning"
	usecaserecipe "github.com/ZetoOfficial/meal-planner/internal/usecase/recipe"
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

type RecipeUseCases interface {
	CreateRecipe(context.Context, usecaserecipe.CreateRecipeCommand) (recipedom.Template, error)
	ListRecipes(context.Context, *recipedom.Status) ([]recipedom.Template, error)
	GetRecipe(context.Context, recipedom.ID) (recipedom.Template, error)
	PromoteRecipe(context.Context, recipedom.ID) (recipedom.Template, error)
}

type PlanningUseCases interface {
	GeneratePlan(context.Context, usecaseplanning.GeneratePlanCommand) (usecaseplanning.PlanResult, error)
	GetPlan(context.Context, planningdom.PlanID) (usecaseplanning.PlanResult, error)
	ListPlans(context.Context) ([]planningdom.Plan, error)
	ArchivePlan(context.Context, planningdom.PlanID) error
}

type server struct {
	catalog  CatalogUseCases
	eater    EaterUseCases
	recipe   RecipeUseCases
	planning PlanningUseCases
}

func newServer(catalog CatalogUseCases, eater EaterUseCases, recipe RecipeUseCases, planning PlanningUseCases) *server {
	return &server{catalog: catalog, eater: eater, recipe: recipe, planning: planning}
}
