package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/ZetoOfficial/meal-planner/internal/api/openapi"
	catalogdom "github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
	apperr "github.com/ZetoOfficial/meal-planner/internal/errors"
	cataloguc "github.com/ZetoOfficial/meal-planner/internal/usecase/catalog"
)

func (a *API) listProducts(w http.ResponseWriter, r *http.Request, params openapi.ListProductsParams) {
	if a.deps.Catalog == nil {
		a.notImplemented(w)
		return
	}
	filter := cataloguc.ProductFilter{Role: params.Role}
	products, err := a.deps.Catalog.ListProducts(r.Context(), filter)
	if err != nil {
		writeError(w, err)
		return
	}
	out := make([]openapi.Product, 0, len(products))
	for _, product := range products {
		out = append(out, productToAPI(product))
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *API) createProduct(w http.ResponseWriter, r *http.Request) {
	if a.deps.Catalog == nil {
		a.notImplemented(w)
		return
	}
	input, ok := decodeProductInput(w, r)
	if !ok {
		return
	}
	product, err := a.deps.Catalog.CreateProduct(r.Context(), productInputToDomain(input))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, productToAPI(product))
}

func (a *API) getProduct(w http.ResponseWriter, r *http.Request, productID openapi.ProductId) {
	if a.deps.Catalog == nil {
		a.notImplemented(w)
		return
	}
	product, err := a.deps.Catalog.GetProduct(r.Context(), catalogdom.ProductID(productID))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, productToAPI(product))
}

func (a *API) updateProduct(w http.ResponseWriter, r *http.Request, productID openapi.ProductId) {
	if a.deps.Catalog == nil {
		a.notImplemented(w)
		return
	}
	input, ok := decodeProductInput(w, r)
	if !ok {
		return
	}
	product, err := a.deps.Catalog.UpdateProduct(r.Context(), catalogdom.ProductID(productID), productInputToDomain(input))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, productToAPI(product))
}

func (a *API) deleteProduct(w http.ResponseWriter, r *http.Request, productID openapi.ProductId) {
	if a.deps.Catalog == nil {
		a.notImplemented(w)
		return
	}
	if err := a.deps.Catalog.DeleteProduct(r.Context(), catalogdom.ProductID(productID)); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) listRoleTags(w http.ResponseWriter, r *http.Request) {
	if a.deps.Catalog == nil {
		a.notImplemented(w)
		return
	}
	roles, err := a.deps.Catalog.ListRoleTags(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	out := make([]openapi.RoleTag, 0, len(roles))
	for _, role := range roles {
		out = append(out, openapi.RoleTag{
			Id:       role.ID,
			Code:     role.Code,
			Kind:     openapi.RoleKind(role.Kind),
			Label:    role.Label,
			ParentId: role.ParentID,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

func decodeProductInput(w http.ResponseWriter, r *http.Request) (openapi.ProductInput, bool) {
	defer r.Body.Close()
	var input openapi.ProductInput
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeError(w, apperr.Validation("invalid product request body", err.Error()))
		return openapi.ProductInput{}, false
	}
	return input, true
}

func productInputToDomain(input openapi.ProductInput) catalogdom.Product {
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

func productToAPI(product catalogdom.Product) openapi.Product {
	out := openapi.Product{
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

func nutritionInputToDomain(input *openapi.NutritionFacts) *catalogdom.NutritionFacts {
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

func nutritionToAPI(n *catalogdom.NutritionFacts) *openapi.NutritionFacts {
	if n == nil {
		return nil
	}
	return &openapi.NutritionFacts{
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

func dataQualityInputToDomain(input *openapi.NutritionDataQuality) *catalogdom.NutritionDataQualityMetadata {
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

func dataQualityToAPI(input *catalogdom.NutritionDataQualityMetadata) *openapi.NutritionDataQuality {
	if input == nil {
		return nil
	}
	var quality *openapi.NutritionDataQualityNutritionDataQuality
	if input.NutritionDataQuality != nil {
		value := openapi.NutritionDataQualityNutritionDataQuality(*input.NutritionDataQuality)
		quality = &value
	}
	var sourceKind *openapi.NutritionDataQualitySourceKind
	if input.SourceKind != nil {
		value := openapi.NutritionDataQualitySourceKind(*input.SourceKind)
		sourceKind = &value
	}
	return &openapi.NutritionDataQuality{
		NutritionDataQuality:   quality,
		NutritionCoverageScore: float64PtrToFloat32Ptr(input.NutritionCoverageScore),
		SourceKind:             sourceKind,
		SourceNote:             input.SourceNote,
		LastVerifiedAt:         input.LastVerifiedAt,
	}
}

func classificationInputToDomain(input *openapi.FoodClassification) *catalogdom.FoodClassification {
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

func classificationToAPI(input *catalogdom.FoodClassification) *openapi.FoodClassification {
	if input == nil {
		return nil
	}
	var foodGroup *openapi.FoodClassificationFoodGroup
	if input.FoodGroup != nil {
		value := openapi.FoodClassificationFoodGroup(*input.FoodGroup)
		foodGroup = &value
	}
	var processingLevel *openapi.FoodClassificationProcessingLevel
	if input.ProcessingLevel != nil {
		value := openapi.FoodClassificationProcessingLevel(*input.ProcessingLevel)
		processingLevel = &value
	}
	return &openapi.FoodClassification{
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
