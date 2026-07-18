package httpapi

import (
	"context"

	"github.com/ZetoOfficial/meal-planner/internal/api/openapi"
	catalogdom "github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
)

func (a *api) ListProducts(ctx context.Context, request openapi.ListProductsRequestObject) (openapi.ListProductsResponseObject, error) {
	role := ""
	if request.Params.Role != nil {
		role = *request.Params.Role
	}
	products, err := a.catalog.ListProducts(ctx, role)
	if err != nil {
		return nil, err
	}
	return openapi.ListProducts200JSONResponse(productsResponse(products)), nil
}

func (a *api) CreateProduct(ctx context.Context, request openapi.CreateProductRequestObject) (openapi.CreateProductResponseObject, error) {
	product, err := a.catalog.CreateProduct(ctx, productCommand(*request.Body))
	if err != nil {
		return nil, err
	}
	return openapi.CreateProduct201JSONResponse{
		ProductOKJSONResponse: openapi.ProductOKJSONResponse(productResponse(product)),
	}, nil
}

func (a *api) GetProduct(ctx context.Context, request openapi.GetProductRequestObject) (openapi.GetProductResponseObject, error) {
	product, err := a.catalog.GetProduct(ctx, catalogdom.ProductID(request.ProductId))
	if err != nil {
		return nil, err
	}
	return openapi.GetProduct200JSONResponse{
		ProductOKJSONResponse: openapi.ProductOKJSONResponse(productResponse(product)),
	}, nil
}

func (a *api) UpdateProduct(ctx context.Context, request openapi.UpdateProductRequestObject) (openapi.UpdateProductResponseObject, error) {
	product, err := a.catalog.UpdateProduct(ctx, catalogdom.ProductID(request.ProductId), productCommand(*request.Body))
	if err != nil {
		return nil, err
	}
	return openapi.UpdateProduct200JSONResponse{
		ProductOKJSONResponse: openapi.ProductOKJSONResponse(productResponse(product)),
	}, nil
}

func (a *api) DeleteProduct(ctx context.Context, request openapi.DeleteProductRequestObject) (openapi.DeleteProductResponseObject, error) {
	if err := a.catalog.DeleteProduct(ctx, catalogdom.ProductID(request.ProductId)); err != nil {
		return nil, err
	}
	return openapi.DeleteProduct204Response{}, nil
}

func (a *api) ListRoleTags(ctx context.Context, _ openapi.ListRoleTagsRequestObject) (openapi.ListRoleTagsResponseObject, error) {
	tags, err := a.catalog.ListRoleTags(ctx)
	if err != nil {
		return nil, err
	}
	return openapi.ListRoleTags200JSONResponse(roleTagsResponse(tags)), nil
}
