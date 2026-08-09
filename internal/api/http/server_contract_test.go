package httpapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.uber.org/mock/gomock"

	httpapi "github.com/ZetoOfficial/meal-planner/internal/api/http"
	"github.com/ZetoOfficial/meal-planner/internal/api/http/mocks"
	"github.com/ZetoOfficial/meal-planner/internal/api/openapi"
	catalogdom "github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
	"github.com/ZetoOfficial/meal-planner/internal/domain/meal"
	recipedom "github.com/ZetoOfficial/meal-planner/internal/domain/recipe"
	apperr "github.com/ZetoOfficial/meal-planner/internal/errors"
	usecasecatalog "github.com/ZetoOfficial/meal-planner/internal/usecase/catalog"
	usecaserecipe "github.com/ZetoOfficial/meal-planner/internal/usecase/recipe"
)

func TestCreateProductHTTPContract(t *testing.T) {
	ctrl := gomock.NewController(t)
	catalog := mocks.NewMockCatalogUseCases(ctrl)
	command := usecasecatalog.CreateProductCommand{
		Brand:          "Acme",
		Name:           "Chicken breast",
		ProteinPer100G: 23,
		FatPer100G:     2,
		CarbPer100G:    0,
		KcalPer100G:    110,
		DefaultUnit:    "g",
		Purchasable:    true,
		Roles:          []string{"protein:lean"},
	}
	catalog.EXPECT().
		CreateProduct(gomock.Any(), command).
		Return(catalogdom.Product{
			ID:             42,
			Brand:          command.Brand,
			Name:           command.Name,
			ProteinPer100G: command.ProteinPer100G,
			FatPer100G:     command.FatPer100G,
			CarbPer100G:    command.CarbPer100G,
			KcalPer100G:    command.KcalPer100G,
			Purchasable:    command.Purchasable,
			Roles:          []catalogdom.RoleTag{{Code: "protein:lean", Kind: catalogdom.RoleKindProtein, Label: "Lean protein"}},
		}, nil)

	handler := httpapi.New(httpapi.Deps{APIToken: "secret", Catalog: catalog})
	req := httptest.NewRequest(http.MethodPost, "/products", strings.NewReader(`{
		"brand":"Acme",
		"name":"Chicken breast",
		"protein_per_100g":23,
		"fat_per_100g":2,
		"carb_per_100g":0,
		"kcal_per_100g":110,
		"roles":["protein:lean"]
	}`))
	req.Header.Set("X-API-Token", "secret")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /products status = %d, want %d: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	var product openapi.Product
	if err := json.NewDecoder(rec.Body).Decode(&product); err != nil {
		t.Fatalf("decode product: %v", err)
	}
	if product.Id != 42 || product.Name != command.Name {
		t.Fatalf("product = %#v", product)
	}
}

func TestApplicationErrorHTTPContract(t *testing.T) {
	ctrl := gomock.NewController(t)
	catalog := mocks.NewMockCatalogUseCases(ctrl)
	catalog.EXPECT().
		GetProduct(gomock.Any(), catalogdom.ProductID(42)).
		Return(catalogdom.Product{}, apperr.NotFound("product"))

	handler := httpapi.New(httpapi.Deps{APIToken: "secret", Catalog: catalog})
	req := httptest.NewRequest(http.MethodGet, "/products/42", http.NoBody)
	req.Header.Set("X-API-Token", "secret")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET /products/42 status = %d, want %d: %s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
	var response openapi.Error
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if response.Error != string(apperr.CodeNotFound) {
		t.Fatalf("error = %q, want %q", response.Error, apperr.CodeNotFound)
	}
}

func TestMalformedRequestHTTPContract(t *testing.T) {
	handler := httpapi.New(httpapi.Deps{APIToken: "secret"})
	req := httptest.NewRequest(http.MethodPost, "/eaters/1/fixed-meals", strings.NewReader(`{
		"meal_type":"breakfast",
		"label":"Toast",
		"macros":{"protein_g":0,"fat_g":0,"carb_g":0}
	}`))
	req.Header.Set("X-API-Token", "secret")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("POST /eaters/1/fixed-meals status = %d, want %d: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	var response openapi.Error
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if response.Error != string(apperr.CodeValidation) {
		t.Fatalf("error = %q, want %q", response.Error, apperr.CodeValidation)
	}
}

func TestCreateRecipeHTTPContract(t *testing.T) {
	ctrl := gomock.NewController(t)
	recipes := mocks.NewMockRecipeUseCases(ctrl)
	expectedOutput := 720.0
	preparedFactor := 0.75
	command := usecaserecipe.CreateRecipeCommand{
		Name:             "Roasted chicken",
		ComponentKind:    recipedom.ComponentKindProtein,
		AllowedMealTypes: []meal.Type{meal.TypeLunch, meal.TypeDinner},
		BaseServings:     4,
		ExpectedOutputG:  &expectedOutput,
		Storage:          recipedom.StorageKeeps3dReheat,
		Slots: []usecaserecipe.SlotCommand{{
			RoleTagCode:       "protein:lean",
			Label:             "Protein",
			QtyG:              960,
			PreparedQtyFactor: &preparedFactor,
			Position:          0,
		}},
	}
	recipes.EXPECT().
		CreateRecipe(gomock.Any(), command).
		Return(recipedom.Template{
			ID:               42,
			Name:             command.Name,
			Status:           recipedom.StatusDraft,
			Source:           recipedom.SourceManual,
			ComponentKind:    command.ComponentKind,
			AllowedMealTypes: command.AllowedMealTypes,
			BaseServings:     command.BaseServings,
			ExpectedOutputG:  command.ExpectedOutputG,
			Storage:          recipedom.StorageKeeps3dReheat,
			Weight:           1,
			Equipment:        []string{},
			Slots: []recipedom.Slot{{
				ID:                9,
				RoleTagCode:       "protein:lean",
				Label:             "Protein",
				QtyG:              960,
				PreparedQtyFactor: &preparedFactor,
				Position:          0,
			}},
		}, nil)

	handler := httpapi.New(httpapi.Deps{APIToken: "secret", Recipe: recipes})
	req := httptest.NewRequest(http.MethodPost, "/recipes", strings.NewReader(`{
		"name":"Roasted chicken",
		"component_kind":"protein",
		"allowed_meal_types":["lunch","dinner"],
		"base_servings":4,
		"expected_output_g":720,
		"storage":"keeps_3d_reheat",
		"slots":[{
			"role_tag":"protein:lean",
			"label":"Protein",
			"qty_g":960,
			"prepared_qty_factor":0.75,
			"position":0
		}]
	}`))
	req.Header.Set("X-API-Token", "secret")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /recipes status = %d, want %d: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	var response openapi.RecipeTemplate
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode recipe: %v", err)
	}
	if response.Id != 42 ||
		response.Status != openapi.Draft ||
		response.ComponentKind != openapi.ComponentKindProtein ||
		response.BaseServings != 4 ||
		len(response.AllowedMealTypes) != 2 ||
		response.ExpectedOutputG == nil ||
		*response.ExpectedOutputG != 720 ||
		len(response.Slots) != 1 ||
		response.Slots[0].RoleTag != "protein:lean" ||
		response.Slots[0].QtyG != 960 ||
		response.Slots[0].PreparedQtyFactor == nil ||
		*response.Slots[0].PreparedQtyFactor != 0.75 {
		t.Fatalf("recipe = %#v", response)
	}
	if response.Equipment == nil || len(*response.Equipment) != 0 {
		t.Fatalf("equipment = %#v, want empty array", response.Equipment)
	}
}

func TestPromoteRecipeConflictHTTPContract(t *testing.T) {
	ctrl := gomock.NewController(t)
	recipes := mocks.NewMockRecipeUseCases(ctrl)
	recipes.EXPECT().
		PromoteRecipe(gomock.Any(), recipedom.ID(7)).
		Return(recipedom.Template{}, apperr.Conflict("only draft recipes can be promoted"))

	handler := httpapi.New(httpapi.Deps{APIToken: "secret", Recipe: recipes})
	req := httptest.NewRequest(http.MethodPost, "/recipes/7/promote", http.NoBody)
	req.Header.Set("X-API-Token", "secret")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("POST /recipes/7/promote status = %d, want %d: %s", rec.Code, http.StatusConflict, rec.Body.String())
	}
}
