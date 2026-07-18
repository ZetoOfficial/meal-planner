package httpapi

import (
	"context"

	"github.com/ZetoOfficial/meal-planner/internal/api/openapi"
	recipedom "github.com/ZetoOfficial/meal-planner/internal/domain/recipe"
)

func (a *api) ListRecipes(ctx context.Context, request openapi.ListRecipesRequestObject) (openapi.ListRecipesResponseObject, error) {
	var status *recipedom.Status
	if request.Params.Status != nil {
		value := recipedom.Status(*request.Params.Status)
		status = &value
	}
	recipes, err := a.recipe.ListRecipes(ctx, status)
	if err != nil {
		return nil, err
	}
	return openapi.ListRecipes200JSONResponse(recipesResponse(recipes)), nil
}

func (a *api) CreateRecipe(ctx context.Context, request openapi.CreateRecipeRequestObject) (openapi.CreateRecipeResponseObject, error) {
	recipe, err := a.recipe.CreateRecipe(ctx, recipeCommand(*request.Body))
	if err != nil {
		return nil, err
	}
	return openapi.CreateRecipe201JSONResponse{
		RecipeOKJSONResponse: openapi.RecipeOKJSONResponse(recipeResponse(recipe)),
	}, nil
}

func (a *api) GetRecipe(ctx context.Context, request openapi.GetRecipeRequestObject) (openapi.GetRecipeResponseObject, error) {
	recipe, err := a.recipe.GetRecipe(ctx, recipedom.ID(request.RecipeId))
	if err != nil {
		return nil, err
	}
	return openapi.GetRecipe200JSONResponse{
		RecipeOKJSONResponse: openapi.RecipeOKJSONResponse(recipeResponse(recipe)),
	}, nil
}

func (a *api) PromoteRecipe(ctx context.Context, request openapi.PromoteRecipeRequestObject) (openapi.PromoteRecipeResponseObject, error) {
	recipe, err := a.recipe.PromoteRecipe(ctx, recipedom.ID(request.RecipeId))
	if err != nil {
		return nil, err
	}
	return openapi.PromoteRecipe200JSONResponse{
		RecipeOKJSONResponse: openapi.RecipeOKJSONResponse(recipeResponse(recipe)),
	}, nil
}
