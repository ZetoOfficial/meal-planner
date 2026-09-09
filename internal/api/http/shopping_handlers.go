package httpapi

import (
	"context"

	"github.com/ZetoOfficial/meal-planner/api/sdk/go/api"
	planningdom "github.com/ZetoOfficial/meal-planner/internal/domain/planning"
	shoppingdom "github.com/ZetoOfficial/meal-planner/internal/domain/shopping"
)

func (s *server) GetShoppingList(ctx context.Context, request api.GetShoppingListRequestObject) (api.GetShoppingListResponseObject, error) {
	list, err := s.shopping.GetList(ctx, planningdom.PlanID(request.PlanId))
	if err != nil {
		return nil, err
	}
	return api.GetShoppingList200JSONResponse{
		ShoppingOKJSONResponse: api.ShoppingOKJSONResponse(shoppingListResponse(list)),
	}, nil
}

func (s *server) RegenerateShoppingList(ctx context.Context, request api.RegenerateShoppingListRequestObject) (api.RegenerateShoppingListResponseObject, error) {
	list, err := s.shopping.Regenerate(ctx, planningdom.PlanID(request.PlanId))
	if err != nil {
		return nil, err
	}
	return api.RegenerateShoppingList200JSONResponse{
		ShoppingOKJSONResponse: api.ShoppingOKJSONResponse(shoppingListResponse(list)),
	}, nil
}

func (s *server) UpdateShoppingListItem(ctx context.Context, request api.UpdateShoppingListItemRequestObject) (api.UpdateShoppingListItemResponseObject, error) {
	itemID := shoppingdom.ItemID(request.ItemId)
	if err := s.shopping.SetChecked(ctx, itemID, request.Body.Checked); err != nil {
		return nil, err
	}
	item, err := s.shopping.GetItem(ctx, itemID)
	if err != nil {
		return nil, err
	}
	return api.UpdateShoppingListItem200JSONResponse(shoppingItemResponse(item)), nil
}
