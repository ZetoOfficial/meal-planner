package httpapi

//go:generate go tool mockgen -source=$GOFILE -destination=mocks/usecases.go -package=mocks

import (
	"context"

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

func newAPI(catalog CatalogUseCases, eater EaterUseCases) *api {
	return &api{catalog: catalog, eater: eater}
}
