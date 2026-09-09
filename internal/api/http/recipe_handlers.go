package httpapi

import (
	"context"

	"github.com/ZetoOfficial/meal-planner/api/sdk/go/api"
	recipedom "github.com/ZetoOfficial/meal-planner/internal/domain/recipe"
)

func (s *server) ListRecipes(ctx context.Context, request api.ListRecipesRequestObject) (api.ListRecipesResponseObject, error) {
	var status *recipedom.Status
	if request.Params.Status != nil {
		value := recipedom.Status(*request.Params.Status)
		status = &value
	}
	recipes, err := s.recipe.ListRecipes(ctx, status)
	if err != nil {
		return nil, err
	}
	return api.ListRecipes200JSONResponse(recipesResponse(recipes)), nil
}

func (s *server) CreateRecipe(ctx context.Context, request api.CreateRecipeRequestObject) (api.CreateRecipeResponseObject, error) {
	recipe, err := s.recipe.CreateRecipe(ctx, recipeCommand(*request.Body))
	if err != nil {
		return nil, err
	}
	return api.CreateRecipe201JSONResponse{
		RecipeOKJSONResponse: api.RecipeOKJSONResponse(recipeResponse(recipe)),
	}, nil
}

func (s *server) GetRecipe(ctx context.Context, request api.GetRecipeRequestObject) (api.GetRecipeResponseObject, error) {
	recipe, err := s.recipe.GetRecipe(ctx, recipedom.ID(request.RecipeId))
	if err != nil {
		return nil, err
	}
	return api.GetRecipe200JSONResponse{
		RecipeOKJSONResponse: api.RecipeOKJSONResponse(recipeResponse(recipe)),
	}, nil
}

func (s *server) PromoteRecipe(ctx context.Context, request api.PromoteRecipeRequestObject) (api.PromoteRecipeResponseObject, error) {
	recipe, err := s.recipe.PromoteRecipe(ctx, recipedom.ID(request.RecipeId))
	if err != nil {
		return nil, err
	}
	return api.PromoteRecipe200JSONResponse{
		RecipeOKJSONResponse: api.RecipeOKJSONResponse(recipeResponse(recipe)),
	}, nil
}
