package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/ZetoOfficial/meal-planner/internal/api/openapi"
	catalogdom "github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
	apperr "github.com/ZetoOfficial/meal-planner/internal/errors"
)

func (a *api) ListProducts(w http.ResponseWriter, r *http.Request, params openapi.ListProductsParams) {
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
	if err := a.catalog.DeleteProduct(r.Context(), catalogdom.ProductID(id)); err != nil {
		writeError(w, err)
		return
	}
	writeOpenAPIResponse(w, openapi.DeleteProduct204Response{}.VisitDeleteProductResponse)
}

func (a *api) ListRoleTags(w http.ResponseWriter, r *http.Request) {
	tags, err := a.catalog.ListRoleTags(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	result := make([]openapi.RoleTag, 0, len(tags))
	for i := range tags {
		result = append(result, roleTagResponse(tags[i]))
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
