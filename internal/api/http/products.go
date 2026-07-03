package httpapi

import (
	"context"
	"net/http"

	api "github.com/ZetoOfficial/meal-planner/internal/api/openapi"
	catalogdom "github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
	cataloguc "github.com/ZetoOfficial/meal-planner/internal/usecase/catalog"
)

func (a *API) ListProducts(ctx context.Context, request api.ListProductsRequestObject) (api.ListProductsResponseObject, error) {
	if a.deps.Catalog == nil {
		return api.ListProducts501JSONResponse{NotImplementedJSONResponse: notImplemented()}, nil
	}
	filter := cataloguc.ProductFilter{Role: request.Params.Role}
	products, err := a.deps.Catalog.ListProducts(ctx, filter)
	if err != nil {
		return nil, err
	}
	out := make([]api.Product, 0, len(products))
	for _, product := range products {
		out = append(out, productToAPI(product))
	}
	return api.ListProducts200JSONResponse(out), nil
}

func (a *API) CreateProduct(ctx context.Context, request api.CreateProductRequestObject) (api.CreateProductResponseObject, error) {
	if a.deps.Catalog == nil {
		return api.CreateProduct501JSONResponse{NotImplementedJSONResponse: notImplemented()}, nil
	}
	if request.Body == nil {
		body := validationBody("request body is required")
		return api.CreateProduct422JSONResponse{ValidationErrorJSONResponse: api.ValidationErrorJSONResponse(body)}, nil
	}
	product, err := a.deps.Catalog.CreateProduct(ctx, productInputToDomain(*request.Body))
	if err != nil {
		return createProductError(err), nil
	}
	return api.CreateProduct201JSONResponse{ProductOKJSONResponse: api.ProductOKJSONResponse(productToAPI(product))}, nil
}

func (a *API) GetProduct(ctx context.Context, request api.GetProductRequestObject) (api.GetProductResponseObject, error) {
	if a.deps.Catalog == nil {
		return api.GetProduct501JSONResponse{NotImplementedJSONResponse: notImplemented()}, nil
	}
	product, err := a.deps.Catalog.GetProduct(ctx, catalogdom.ProductID(request.ProductId))
	if err != nil {
		return getProductError(err), nil
	}
	return api.GetProduct200JSONResponse{ProductOKJSONResponse: api.ProductOKJSONResponse(productToAPI(product))}, nil
}

func (a *API) UpdateProduct(ctx context.Context, request api.UpdateProductRequestObject) (api.UpdateProductResponseObject, error) {
	if a.deps.Catalog == nil {
		return api.UpdateProduct501JSONResponse{NotImplementedJSONResponse: notImplemented()}, nil
	}
	if request.Body == nil {
		body := validationBody("request body is required")
		return api.UpdateProduct422JSONResponse{ValidationErrorJSONResponse: api.ValidationErrorJSONResponse(body)}, nil
	}
	product, err := a.deps.Catalog.UpdateProduct(ctx, catalogdom.ProductID(request.ProductId), productInputToDomain(*request.Body))
	if err != nil {
		return updateProductError(err), nil
	}
	return api.UpdateProduct200JSONResponse{ProductOKJSONResponse: api.ProductOKJSONResponse(productToAPI(product))}, nil
}

func (a *API) DeleteProduct(ctx context.Context, request api.DeleteProductRequestObject) (api.DeleteProductResponseObject, error) {
	if a.deps.Catalog == nil {
		return api.DeleteProduct501JSONResponse{NotImplementedJSONResponse: notImplemented()}, nil
	}
	if err := a.deps.Catalog.DeleteProduct(ctx, catalogdom.ProductID(request.ProductId)); err != nil {
		return deleteProductError(err), nil
	}
	return api.DeleteProduct204Response{}, nil
}

func (a *API) ListRoleTags(ctx context.Context, request api.ListRoleTagsRequestObject) (api.ListRoleTagsResponseObject, error) {
	if a.deps.Catalog == nil {
		return api.ListRoleTags501JSONResponse{NotImplementedJSONResponse: notImplemented()}, nil
	}
	roles, err := a.deps.Catalog.ListRoleTags(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]api.RoleTag, 0, len(roles))
	for _, role := range roles {
		out = append(out, api.RoleTag{
			Id:       role.ID,
			Code:     role.Code,
			Kind:     api.RoleKind(role.Kind),
			Label:    role.Label,
			ParentId: role.ParentID,
		})
	}
	return api.ListRoleTags200JSONResponse(out), nil
}

func createProductError(err error) api.CreateProductResponseObject {
	return api.CreateProduct422JSONResponse{ValidationErrorJSONResponse: api.ValidationErrorJSONResponse(errorBody(err))}
}

func getProductError(err error) api.GetProductResponseObject {
	return api.GetProduct404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse(errorBody(err))}
}

func updateProductError(err error) api.UpdateProductResponseObject {
	status, body := apiError(err)
	switch status {
	case http.StatusNotFound:
		return api.UpdateProduct404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse(body)}
	default:
		return api.UpdateProduct422JSONResponse{ValidationErrorJSONResponse: api.ValidationErrorJSONResponse(body)}
	}
}

func deleteProductError(err error) api.DeleteProductResponseObject {
	status, body := apiError(err)
	switch status {
	case http.StatusNotFound:
		return api.DeleteProduct404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse(body)}
	default:
		return api.DeleteProduct422JSONResponse{ValidationErrorJSONResponse: api.ValidationErrorJSONResponse(body)}
	}
}

func errorBody(err error) api.Error {
	_, body := apiError(err)
	return body
}

func apiError(err error) (int, api.Error) {
	status, body := mapError(err)
	if response, ok := body.(api.Error); ok {
		return status, response
	}
	return http.StatusInternalServerError, api.Error{
		Error:   "internal_error",
		Message: "internal server error",
	}
}

func validationBody(message string) api.Error {
	return api.Error{
		Error:   "validation_failed",
		Message: message,
	}
}

func productInputToDomain(input api.ProductInput) catalogdom.Product {
	product := catalogdom.Product{
		Brand:          input.Brand,
		Name:           input.Name,
		ProteinPer100g: float64(input.ProteinPer100g),
		FatPer100g:     float64(input.FatPer100g),
		CarbPer100g:    float64(input.CarbPer100g),
		KcalPer100g:    float64(input.KcalPer100g),
		PricePer100g:   float32PtrToFloat64Ptr(input.PricePer100g),
		DefaultUnit:    stringValue(input.DefaultUnit, "g"),
		Purchasable:    boolValue(input.Purchasable, true),
		Roles:          append([]string(nil), input.Roles...),
	}
	if input.Allergens != nil {
		product.Allergens = append([]string(nil), (*input.Allergens)...)
	}
	product.Nutrition = nutritionInputToDomain(input.Nutrition)
	product.DataQuality = dataQualityInputToDomain(input.DataQuality)
	product.Classification = classificationInputToDomain(input.Classification)
	return product
}

func productToAPI(product catalogdom.Product) api.Product {
	out := api.Product{
		Id:             int64(product.ID),
		Brand:          product.Brand,
		Name:           product.Name,
		ProteinPer100g: float32(product.ProteinPer100g),
		FatPer100g:     float32(product.FatPer100g),
		CarbPer100g:    float32(product.CarbPer100g),
		KcalPer100g:    float32(product.KcalPer100g),
		PricePer100g:   float64PtrToFloat32Ptr(product.PricePer100g),
		DefaultUnit:    &product.DefaultUnit,
		Purchasable:    &product.Purchasable,
		Nutrition:      nutritionToAPI(product.Nutrition),
		DataQuality:    dataQualityToAPI(product.DataQuality),
		Classification: classificationToAPI(product.Classification),
	}
	allergens := append([]string(nil), product.Allergens...)
	roles := append([]string(nil), product.Roles...)
	out.Allergens = &allergens
	out.Roles = &roles
	return out
}

func nutritionInputToDomain(input *api.NutritionFacts) *catalogdom.NutritionFacts {
	if input == nil {
		return nil
	}
	return &catalogdom.NutritionFacts{
		SaturatedFatPer100g:               float32PtrToFloat64Ptr(input.SaturatedFatPer100g),
		TransFatPer100g:                   float32PtrToFloat64Ptr(input.TransFatPer100g),
		IndustrialTransFatPer100g:         float32PtrToFloat64Ptr(input.IndustrialTransFatPer100g),
		FiberPer100g:                      float32PtrToFloat64Ptr(input.FiberPer100g),
		SodiumMgPer100g:                   float32PtrToFloat64Ptr(input.SodiumMgPer100g),
		SaltGPer100g:                      float32PtrToFloat64Ptr(input.SaltGPer100g),
		TotalSugarsPer100g:                float32PtrToFloat64Ptr(input.TotalSugarsPer100g),
		AddedSugarsPer100g:                float32PtrToFloat64Ptr(input.AddedSugarsPer100g),
		FreeSugarsPer100g:                 float32PtrToFloat64Ptr(input.FreeSugarsPer100g),
		ContainsPartiallyHydrogenatedOils: input.ContainsPartiallyHydrogenatedOils,
	}
}

func nutritionToAPI(n *catalogdom.NutritionFacts) *api.NutritionFacts {
	if n == nil {
		return nil
	}
	return &api.NutritionFacts{
		SaturatedFatPer100g:               float64PtrToFloat32Ptr(n.SaturatedFatPer100g),
		TransFatPer100g:                   float64PtrToFloat32Ptr(n.TransFatPer100g),
		IndustrialTransFatPer100g:         float64PtrToFloat32Ptr(n.IndustrialTransFatPer100g),
		FiberPer100g:                      float64PtrToFloat32Ptr(n.FiberPer100g),
		SodiumMgPer100g:                   float64PtrToFloat32Ptr(n.SodiumMgPer100g),
		SaltGPer100g:                      float64PtrToFloat32Ptr(n.SaltGPer100g),
		TotalSugarsPer100g:                float64PtrToFloat32Ptr(n.TotalSugarsPer100g),
		AddedSugarsPer100g:                float64PtrToFloat32Ptr(n.AddedSugarsPer100g),
		FreeSugarsPer100g:                 float64PtrToFloat32Ptr(n.FreeSugarsPer100g),
		ContainsPartiallyHydrogenatedOils: n.ContainsPartiallyHydrogenatedOils,
	}
}

func dataQualityInputToDomain(input *api.NutritionDataQuality) *catalogdom.NutritionDataQualityMetadata {
	if input == nil {
		return nil
	}
	var quality *catalogdom.NutritionDataQuality
	if input.NutritionDataQuality != nil {
		value := catalogdom.NutritionDataQuality(*input.NutritionDataQuality)
		quality = &value
	}
	var sourceKind *catalogdom.SourceKind
	if input.SourceKind != nil {
		value := catalogdom.SourceKind(*input.SourceKind)
		sourceKind = &value
	}
	return &catalogdom.NutritionDataQualityMetadata{
		NutritionDataQuality:   quality,
		NutritionCoverageScore: float32PtrToFloat64Ptr(input.NutritionCoverageScore),
		SourceKind:             sourceKind,
		SourceNote:             input.SourceNote,
		LastVerifiedAt:         input.LastVerifiedAt,
	}
}

func dataQualityToAPI(input *catalogdom.NutritionDataQualityMetadata) *api.NutritionDataQuality {
	if input == nil {
		return nil
	}
	var quality *api.NutritionDataQualityNutritionDataQuality
	if input.NutritionDataQuality != nil {
		value := api.NutritionDataQualityNutritionDataQuality(*input.NutritionDataQuality)
		quality = &value
	}
	var sourceKind *api.NutritionDataQualitySourceKind
	if input.SourceKind != nil {
		value := api.NutritionDataQualitySourceKind(*input.SourceKind)
		sourceKind = &value
	}
	return &api.NutritionDataQuality{
		NutritionDataQuality:   quality,
		NutritionCoverageScore: float64PtrToFloat32Ptr(input.NutritionCoverageScore),
		SourceKind:             sourceKind,
		SourceNote:             input.SourceNote,
		LastVerifiedAt:         input.LastVerifiedAt,
	}
}

func classificationInputToDomain(input *api.FoodClassification) *catalogdom.FoodClassification {
	if input == nil {
		return nil
	}
	var foodGroup *catalogdom.FoodGroup
	if input.FoodGroup != nil {
		value := catalogdom.FoodGroup(*input.FoodGroup)
		foodGroup = &value
	}
	var processingLevel *catalogdom.ProcessingLevel
	if input.ProcessingLevel != nil {
		value := catalogdom.ProcessingLevel(*input.ProcessingLevel)
		processingLevel = &value
	}
	return &catalogdom.FoodClassification{
		FoodGroup:            foodGroup,
		ProcessingLevel:      processingLevel,
		ProcessingConfidence: float32PtrToFloat64Ptr(input.ProcessingConfidence),
		CountsToFruitVeg:     input.CountsToFruitVeg,
		IsLegume:             input.IsLegume,
		IsWholeGrain:         input.IsWholeGrain,
		WholeGrainRatio:      float32PtrToFloat64Ptr(input.WholeGrainRatio),
		IsRedMeat:            input.IsRedMeat,
		IsProcessedMeat:      input.IsProcessedMeat,
		ClassificationNote:   input.ClassificationNote,
	}
}

func classificationToAPI(input *catalogdom.FoodClassification) *api.FoodClassification {
	if input == nil {
		return nil
	}
	var foodGroup *api.FoodClassificationFoodGroup
	if input.FoodGroup != nil {
		value := api.FoodClassificationFoodGroup(*input.FoodGroup)
		foodGroup = &value
	}
	var processingLevel *api.FoodClassificationProcessingLevel
	if input.ProcessingLevel != nil {
		value := api.FoodClassificationProcessingLevel(*input.ProcessingLevel)
		processingLevel = &value
	}
	return &api.FoodClassification{
		FoodGroup:            foodGroup,
		ProcessingLevel:      processingLevel,
		ProcessingConfidence: float64PtrToFloat32Ptr(input.ProcessingConfidence),
		CountsToFruitVeg:     input.CountsToFruitVeg,
		IsLegume:             input.IsLegume,
		IsWholeGrain:         input.IsWholeGrain,
		WholeGrainRatio:      float64PtrToFloat32Ptr(input.WholeGrainRatio),
		IsRedMeat:            input.IsRedMeat,
		IsProcessedMeat:      input.IsProcessedMeat,
		ClassificationNote:   input.ClassificationNote,
	}
}

func float32PtrToFloat64Ptr(value *float32) *float64 {
	if value == nil {
		return nil
	}
	out := float64(*value)
	return &out
}

func float64PtrToFloat32Ptr(value *float64) *float32 {
	if value == nil {
		return nil
	}
	out := float32(*value)
	return &out
}

func stringValue(value *string, fallback string) string {
	if value == nil || *value == "" {
		return fallback
	}
	return *value
}

func boolValue(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}
