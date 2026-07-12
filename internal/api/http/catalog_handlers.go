package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/ZetoOfficial/meal-planner/internal/api/openapi"
	catalogdom "github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
	apperr "github.com/ZetoOfficial/meal-planner/internal/errors"
	usecasecatalog "github.com/ZetoOfficial/meal-planner/internal/usecase/catalog"
)

func (a *api) ListProducts(w http.ResponseWriter, r *http.Request, params openapi.ListProductsParams) {
	if a.catalog == nil {
		notImplemented(w)
		return
	}
	role := ""
	if params.Role != nil {
		role = *params.Role
	}
	products, err := a.catalog.ListProducts(r.Context(), role)
	if err != nil {
		writeError(w, err)
		return
	}
	result := make([]openapi.Product, 0, len(products))
	for i := range products {
		result = append(result, productResponse(products[i]))
	}
	writeOpenAPIResponse(w, openapi.ListProducts200JSONResponse(result).VisitListProductsResponse)
}

func (a *api) CreateProduct(w http.ResponseWriter, r *http.Request) {
	if a.catalog == nil {
		notImplemented(w)
		return
	}
	var input openapi.ProductInput
	if err := decodeJSON(r, &input, validateProductInput); err != nil {
		writeError(w, err)
		return
	}
	product, err := a.catalog.CreateProduct(r.Context(), productCommand(input))
	if err != nil {
		writeError(w, err)
		return
	}
	writeOpenAPIResponse(w, openapi.CreateProduct201JSONResponse{
		ProductOKJSONResponse: openapi.ProductOKJSONResponse(productResponse(product)),
	}.VisitCreateProductResponse)
}

func (a *api) GetProduct(w http.ResponseWriter, r *http.Request, id openapi.ProductId) {
	if a.catalog == nil {
		notImplemented(w)
		return
	}
	product, err := a.catalog.GetProduct(r.Context(), catalogdom.ProductID(id))
	if err != nil {
		writeError(w, err)
		return
	}
	writeOpenAPIResponse(w, openapi.GetProduct200JSONResponse{
		ProductOKJSONResponse: openapi.ProductOKJSONResponse(productResponse(product)),
	}.VisitGetProductResponse)
}

func (a *api) UpdateProduct(w http.ResponseWriter, r *http.Request, id openapi.ProductId) {
	if a.catalog == nil {
		notImplemented(w)
		return
	}
	var input openapi.ProductInput
	if err := decodeJSON(r, &input, validateProductInput); err != nil {
		writeError(w, err)
		return
	}
	product, err := a.catalog.UpdateProduct(r.Context(), catalogdom.ProductID(id), productCommand(input))
	if err != nil {
		writeError(w, err)
		return
	}
	writeOpenAPIResponse(w, openapi.UpdateProduct200JSONResponse{
		ProductOKJSONResponse: openapi.ProductOKJSONResponse(productResponse(product)),
	}.VisitUpdateProductResponse)
}

func (a *api) DeleteProduct(w http.ResponseWriter, r *http.Request, id openapi.ProductId) {
	if a.catalog == nil {
		notImplemented(w)
		return
	}
	if err := a.catalog.DeleteProduct(r.Context(), catalogdom.ProductID(id)); err != nil {
		writeError(w, err)
		return
	}
	writeOpenAPIResponse(w, openapi.DeleteProduct204Response{}.VisitDeleteProductResponse)
}

func (a *api) ListRoleTags(w http.ResponseWriter, r *http.Request) {
	if a.catalog == nil {
		notImplemented(w)
		return
	}
	tags, err := a.catalog.ListRoleTags(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	result := make([]openapi.RoleTag, 0, len(tags))
	for i := range tags {
		tag := &tags[i]
		result = append(result, openapi.RoleTag{Id: tag.ID, Code: tag.Code, Kind: openapi.RoleKind(tag.Kind), Label: tag.Label, ParentId: tag.ParentID})
	}
	writeOpenAPIResponse(w, openapi.ListRoleTags200JSONResponse(result).VisitListRoleTagsResponse)
}

type jsonObjectValidator func(map[string]json.RawMessage) error

func decodeJSON(r *http.Request, destination any, validate jsonObjectValidator) error {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return apperr.Validation("read JSON request body", err.Error())
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return apperr.Validation("invalid JSON request body", err.Error())
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return apperr.Validation("request body must contain one JSON object")
	}
	if validate == nil {
		return nil
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(body, &object); err != nil {
		return apperr.Validation("JSON request body must be an object", err.Error())
	}
	return validate(object)
}

func validateProductInput(object map[string]json.RawMessage) error {
	return requireJSONFields(object, "brand", "name", "protein_per_100g", "fat_per_100g", "carb_per_100g", "kcal_per_100g", "roles")
}

func requireMacroRange(object map[string]json.RawMessage) error {
	return requireJSONFields(object, "protein_min_g", "protein_max_g", "fat_min_g", "fat_max_g", "carb_min_g", "carb_max_g", "kcal_min", "kcal_max")
}

func requireMacros(object map[string]json.RawMessage) error {
	return requireJSONFields(object, "protein_g", "fat_g", "carb_g", "kcal")
}

func requireJSONFields(object map[string]json.RawMessage, fields ...string) error {
	for _, field := range fields {
		value, ok := object[field]
		if !ok || bytes.Equal(value, []byte("null")) {
			return apperr.Validation("required JSON field is missing", field)
		}
	}
	return nil
}

func productCommand(input openapi.ProductInput) usecasecatalog.CreateProductCommand {
	command := usecasecatalog.CreateProductCommand{
		Brand:          input.Brand,
		Name:           input.Name,
		ProteinPer100G: float64(input.ProteinPer100g),
		FatPer100G:     float64(input.FatPer100g),
		CarbPer100G:    float64(input.CarbPer100g),
		KcalPer100G:    float64(input.KcalPer100g),
		PricePer100G:   float64Pointer(input.PricePer100g),
		Purchasable:    true,
		Roles:          append([]string(nil), input.Roles...),
	}
	if input.DefaultUnit != nil {
		command.DefaultUnit = *input.DefaultUnit
	}
	if input.Purchasable != nil {
		command.Purchasable = *input.Purchasable
	}
	if input.Allergens != nil {
		command.Allergens = make([]catalogdom.Allergen, len(*input.Allergens))
		for i, allergen := range *input.Allergens {
			command.Allergens[i] = catalogdom.Allergen(allergen)
		}
	}
	command.Nutrition = nutritionFactsCommand(input.Nutrition)
	command.DataQuality = nutritionDataCommand(input.DataQuality)
	command.Classification = classificationCommand(input.Classification)
	return command
}

func nutritionFactsCommand(input *openapi.NutritionFacts) catalogdom.NutritionFacts {
	if input == nil {
		return catalogdom.NutritionFacts{}
	}
	return catalogdom.NutritionFacts{
		SaturatedFatPer100G:               float64Pointer(input.SaturatedFatPer100g),
		TransFatPer100G:                   float64Pointer(input.TransFatPer100g),
		IndustrialTransFatPer100G:         float64Pointer(input.IndustrialTransFatPer100g),
		FiberPer100G:                      float64Pointer(input.FiberPer100g),
		SodiumMgPer100G:                   float64Pointer(input.SodiumMgPer100g),
		SaltGPer100G:                      float64Pointer(input.SaltGPer100g),
		TotalSugarsPer100G:                float64Pointer(input.TotalSugarsPer100g),
		AddedSugarsPer100G:                float64Pointer(input.AddedSugarsPer100g),
		FreeSugarsPer100G:                 float64Pointer(input.FreeSugarsPer100g),
		ContainsPartiallyHydrogenatedOils: input.ContainsPartiallyHydrogenatedOils,
	}
}

func nutritionDataCommand(input *openapi.NutritionDataQuality) catalogdom.NutritionData {
	if input == nil {
		return catalogdom.NutritionData{}
	}
	result := catalogdom.NutritionData{SourceNote: input.SourceNote, LastVerifiedAt: input.LastVerifiedAt}
	if input.NutritionDataQuality != nil {
		result.Quality = catalogdom.NutritionDataQuality(*input.NutritionDataQuality)
	}
	if input.SourceKind != nil {
		value := catalogdom.SourceKind(*input.SourceKind)
		result.SourceKind = &value
	}
	return result
}

func classificationCommand(input *openapi.FoodClassification) catalogdom.FoodClassification {
	if input == nil {
		return catalogdom.FoodClassification{}
	}
	result := catalogdom.FoodClassification{
		ProcessingConfidence: float64Pointer(input.ProcessingConfidence),
		CountsToFruitVeg:     dereferenceBool(input.CountsToFruitVeg),
		IsLegume:             dereferenceBool(input.IsLegume),
		IsWholeGrain:         dereferenceBool(input.IsWholeGrain),
		WholeGrainRatio:      float64Pointer(input.WholeGrainRatio),
		IsRedMeat:            dereferenceBool(input.IsRedMeat),
		IsProcessedMeat:      dereferenceBool(input.IsProcessedMeat),
		Note:                 input.ClassificationNote,
	}
	if input.FoodGroup != nil {
		result.FoodGroup = catalogdom.FoodGroup(*input.FoodGroup)
	}
	if input.ProcessingLevel != nil {
		result.ProcessingLevel = catalogdom.ProcessingLevel(*input.ProcessingLevel)
	}
	return result
}

func productResponse(product catalogdom.Product) openapi.Product {
	roles := make([]string, len(product.Roles))
	for i := range product.Roles {
		roles[i] = product.Roles[i].Code
	}
	allergens := make([]string, len(product.Allergens))
	for i := range product.Allergens {
		allergens[i] = string(product.Allergens[i])
	}
	return openapi.Product{
		Id:             int64(product.ID),
		Brand:          product.Brand,
		Name:           product.Name,
		ProteinPer100g: float32(product.ProteinPer100G),
		FatPer100g:     float32(product.FatPer100G),
		CarbPer100g:    float32(product.CarbPer100G),
		KcalPer100g:    float32(product.KcalPer100G),
		PricePer100g:   float32Pointer(product.PricePer100G),
		DefaultUnit:    stringPointer(product.DefaultUnit),
		Purchasable:    boolPointer(product.Purchasable),
		Allergens:      &allergens,
		Roles:          &roles,
		Nutrition:      nutritionFactsResponse(product.Nutrition),
		DataQuality:    nutritionDataResponse(product.DataQuality),
		Classification: classificationResponse(product.Classification),
	}
}

func nutritionFactsResponse(value catalogdom.NutritionFacts) *openapi.NutritionFacts {
	return &openapi.NutritionFacts{
		SaturatedFatPer100g:               float32Pointer(value.SaturatedFatPer100G),
		TransFatPer100g:                   float32Pointer(value.TransFatPer100G),
		IndustrialTransFatPer100g:         float32Pointer(value.IndustrialTransFatPer100G),
		FiberPer100g:                      float32Pointer(value.FiberPer100G),
		SodiumMgPer100g:                   float32Pointer(value.SodiumMgPer100G),
		SaltGPer100g:                      float32Pointer(value.SaltGPer100G),
		TotalSugarsPer100g:                float32Pointer(value.TotalSugarsPer100G),
		AddedSugarsPer100g:                float32Pointer(value.AddedSugarsPer100G),
		FreeSugarsPer100g:                 float32Pointer(value.FreeSugarsPer100G),
		ContainsPartiallyHydrogenatedOils: value.ContainsPartiallyHydrogenatedOils,
	}
}

func nutritionDataResponse(value catalogdom.NutritionData) *openapi.NutritionDataQuality {
	result := &openapi.NutritionDataQuality{
		NutritionCoverageScore: float32Pointer(&value.Coverage),
		SourceNote:             value.SourceNote,
		LastVerifiedAt:         value.LastVerifiedAt,
	}
	quality := openapi.NutritionDataQualityNutritionDataQuality(value.Quality)
	result.NutritionDataQuality = &quality
	if value.SourceKind != nil {
		source := openapi.NutritionDataQualitySourceKind(*value.SourceKind)
		result.SourceKind = &source
	}
	return result
}

func classificationResponse(value catalogdom.FoodClassification) *openapi.FoodClassification {
	foodGroup := openapi.FoodClassificationFoodGroup(value.FoodGroup)
	processingLevel := openapi.FoodClassificationProcessingLevel(value.ProcessingLevel)
	return &openapi.FoodClassification{
		FoodGroup:            &foodGroup,
		ProcessingLevel:      &processingLevel,
		ProcessingConfidence: float32Pointer(value.ProcessingConfidence),
		CountsToFruitVeg:     boolPointer(value.CountsToFruitVeg),
		IsLegume:             boolPointer(value.IsLegume),
		IsWholeGrain:         boolPointer(value.IsWholeGrain),
		WholeGrainRatio:      float32Pointer(value.WholeGrainRatio),
		IsRedMeat:            boolPointer(value.IsRedMeat),
		IsProcessedMeat:      boolPointer(value.IsProcessedMeat),
		ClassificationNote:   value.Note,
	}
}

func float64Pointer(value *float32) *float64 {
	if value == nil {
		return nil
	}
	converted := float64(*value)
	return &converted
}

func float32Pointer(value *float64) *float32 {
	if value == nil {
		return nil
	}
	converted := float32(*value)
	return &converted
}

func boolPointer(value bool) *bool       { return &value }
func stringPointer(value string) *string { return &value }
func dereferenceBool(value *bool) bool   { return value != nil && *value }
