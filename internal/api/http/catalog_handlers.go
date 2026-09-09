package httpapi

import (
	"context"

	"github.com/ZetoOfficial/meal-planner/api/sdk/go/api"
	catalogdom "github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
)

func (s *server) ListProducts(ctx context.Context, request api.ListProductsRequestObject) (api.ListProductsResponseObject, error) {
	role := ""
	if request.Params.Role != nil {
		role = *request.Params.Role
	}
	products, err := s.catalog.ListProducts(ctx, role)
	if err != nil {
		return nil, err
	}
	return api.ListProducts200JSONResponse(productsResponse(products)), nil
}

func (s *server) CreateProduct(ctx context.Context, request api.CreateProductRequestObject) (api.CreateProductResponseObject, error) {
	product, err := s.catalog.CreateProduct(ctx, productCommand(*request.Body))
	if err != nil {
		return nil, err
	}
	return api.CreateProduct201JSONResponse{
		ProductOKJSONResponse: api.ProductOKJSONResponse(productResponse(product)),
	}, nil
}

func (s *server) GetProduct(ctx context.Context, request api.GetProductRequestObject) (api.GetProductResponseObject, error) {
	product, err := s.catalog.GetProduct(ctx, catalogdom.ProductID(request.ProductId))
	if err != nil {
		return nil, err
	}
	return api.GetProduct200JSONResponse{
		ProductOKJSONResponse: api.ProductOKJSONResponse(productResponse(product)),
	}, nil
}

func (s *server) UpdateProduct(ctx context.Context, request api.UpdateProductRequestObject) (api.UpdateProductResponseObject, error) {
	product, err := s.catalog.UpdateProduct(ctx, catalogdom.ProductID(request.ProductId), productCommand(*request.Body))
	if err != nil {
		return nil, err
	}
	return api.UpdateProduct200JSONResponse{
		ProductOKJSONResponse: api.ProductOKJSONResponse(productResponse(product)),
	}, nil
}

func (s *server) DeleteProduct(ctx context.Context, request api.DeleteProductRequestObject) (api.DeleteProductResponseObject, error) {
	if err := s.catalog.DeleteProduct(ctx, catalogdom.ProductID(request.ProductId)); err != nil {
		return nil, err
	}
	return api.DeleteProduct204Response{}, nil
}

func (s *server) ListRoleTags(ctx context.Context, _ api.ListRoleTagsRequestObject) (api.ListRoleTagsResponseObject, error) {
	tags, err := s.catalog.ListRoleTags(ctx)
	if err != nil {
		return nil, err
	}
	return api.ListRoleTags200JSONResponse(roleTagsResponse(tags)), nil
}
