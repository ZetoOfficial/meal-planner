package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	catalogdom "github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
	eaterdom "github.com/ZetoOfficial/meal-planner/internal/domain/eater"
	apperr "github.com/ZetoOfficial/meal-planner/internal/errors"
	usecasecatalog "github.com/ZetoOfficial/meal-planner/internal/usecase/catalog"
	usecaseeater "github.com/ZetoOfficial/meal-planner/internal/usecase/eater"
)

type catalogFake struct {
	created catalogdom.Product
	err     error
}

func (f *catalogFake) CreateProduct(_ context.Context, command usecasecatalog.CreateProductCommand) (catalogdom.Product, error) {
	if f.err != nil {
		return catalogdom.Product{}, f.err
	}
	f.created = catalogdom.Product{
		ID:             42,
		Brand:          command.Brand,
		Name:           command.Name,
		ProteinPer100G: command.ProteinPer100G,
		FatPer100G:     command.FatPer100G,
		CarbPer100G:    command.CarbPer100G,
		KcalPer100G:    command.KcalPer100G,
		Roles:          []catalogdom.RoleTag{{Code: command.Roles[0], Kind: catalogdom.RoleKindProtein, Label: "Lean protein"}},
	}
	return f.created, nil
}

func (f *catalogFake) ListProducts(_ context.Context, role string) ([]catalogdom.Product, error) {
	if f.err != nil {
		return nil, f.err
	}
	if role != "protein:lean" {
		return nil, apperr.Validation("unexpected role")
	}
	return []catalogdom.Product{f.created}, nil
}

func (f *catalogFake) GetProduct(context.Context, catalogdom.ProductID) (catalogdom.Product, error) {
	return f.created, f.err
}

func (f *catalogFake) UpdateProduct(context.Context, catalogdom.ProductID, usecasecatalog.UpdateProductCommand) (catalogdom.Product, error) {
	return f.created, f.err
}
func (f *catalogFake) DeleteProduct(context.Context, catalogdom.ProductID) error  { return f.err }
func (f *catalogFake) ListRoleTags(context.Context) ([]catalogdom.RoleTag, error) { return nil, f.err }

type eaterFake struct{}

func (eaterFake) CreateEater(context.Context, usecaseeater.CreateEaterCommand) (eaterdom.Eater, error) {
	return eaterdom.Eater{}, nil
}
func (eaterFake) ListEaters(context.Context) ([]eaterdom.Eater, error) { return nil, nil }
func (eaterFake) GetEater(context.Context, eaterdom.ID) (eaterdom.Eater, error) {
	return eaterdom.Eater{}, nil
}

func (eaterFake) UpdateEater(context.Context, eaterdom.ID, usecaseeater.UpdateEaterCommand) (eaterdom.Eater, error) {
	return eaterdom.Eater{}, nil
}

func (eaterFake) ListGoals(context.Context, eaterdom.ID) ([]eaterdom.MacroGoal, error) {
	return nil, nil
}

func (eaterFake) CreateGoal(context.Context, eaterdom.ID, usecaseeater.MacroGoalCommand) (eaterdom.MacroGoal, error) {
	return eaterdom.MacroGoal{}, nil
}

func (eaterFake) ListFixedMeals(context.Context, eaterdom.ID) ([]eaterdom.FixedMeal, error) {
	return nil, nil
}

func (eaterFake) CreateFixedMeal(context.Context, eaterdom.ID, usecaseeater.CreateFixedMealCommand) (eaterdom.FixedMeal, error) {
	return eaterdom.FixedMeal{}, nil
}

func (eaterFake) ListExclusions(context.Context, eaterdom.ID) ([]eaterdom.Exclusion, error) {
	return nil, nil
}

func (eaterFake) CreateExclusion(context.Context, eaterdom.ID, usecaseeater.CreateExclusionCommand) (eaterdom.Exclusion, error) {
	return eaterdom.Exclusion{}, nil
}
func (eaterFake) DeleteExclusion(context.Context, eaterdom.ExclusionID) error { return nil }

func TestProductCreateAndRoleFilteredList(t *testing.T) {
	catalog := &catalogFake{}
	handler := New(Deps{APIToken: "secret", Catalog: catalog})

	create := httptest.NewRequest(http.MethodPost, "/products", strings.NewReader(`{
  "brand":"Acme", "name":"Chicken breast", "protein_per_100g":23,
  "fat_per_100g":2, "carb_per_100g":0, "kcal_per_100g":110,
  "roles":["protein:lean"]
}`))
	create.Header.Set("X-API-Token", "secret")
	create.Header.Set("Content-Type", "application/json")
	created := httptest.NewRecorder()
	handler.ServeHTTP(created, create)

	if created.Code != http.StatusCreated {
		t.Fatalf("POST /products status = %d, want %d: %s", created.Code, http.StatusCreated, created.Body.String())
	}
	if contentType := created.Header().Get("Content-Type"); contentType != "application/json" {
		t.Fatalf("POST /products Content-Type = %q, want application/json", contentType)
	}
	if catalog.created.Brand != "Acme" || catalog.created.Name != "Chicken breast" {
		t.Fatalf("create command = %#v", catalog.created)
	}

	list := httptest.NewRequest(http.MethodGet, "/products?role=protein:lean", http.NoBody)
	list.Header.Set("X-API-Token", "secret")
	listed := httptest.NewRecorder()
	handler.ServeHTTP(listed, list)

	if listed.Code != http.StatusOK {
		t.Fatalf("GET /products status = %d, want %d: %s", listed.Code, http.StatusOK, listed.Body.String())
	}
	var products []struct {
		ID    int64    `json:"id"`
		Roles []string `json:"roles"`
	}
	if err := json.NewDecoder(listed.Body).Decode(&products); err != nil {
		t.Fatalf("decode product list: %v", err)
	}
	if len(products) != 1 || products[0].ID != 42 || len(products[0].Roles) != 1 || products[0].Roles[0] != "protein:lean" {
		t.Fatalf("products = %#v", products)
	}
}

func TestM2HandlerMapsApplicationErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{name: "not found", err: apperr.NotFound("product"), want: http.StatusNotFound},
		{name: "conflict", err: apperr.Conflict("duplicate product"), want: http.StatusConflict},
		{name: "validation", err: apperr.Validation("invalid product"), want: http.StatusUnprocessableEntity},
	}
	for i := range tests {
		test := &tests[i]
		t.Run(test.name, func(t *testing.T) {
			handler := New(Deps{APIToken: "secret", Catalog: &catalogFake{err: test.err}})
			req := httptest.NewRequest(http.MethodGet, "/products/42", http.NoBody)
			req.Header.Set("X-API-Token", "secret")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != test.want {
				t.Fatalf("status = %d, want %d: %s", rec.Code, test.want, rec.Body.String())
			}
			var body struct {
				Error string `json:"error"`
			}
			if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
				t.Fatalf("decode error: %v", err)
			}
			if body.Error == "" {
				t.Fatal("structured error code is empty")
			}
		})
	}
}

func TestM2HandlersRejectMissingRequiredJSONFields(t *testing.T) {
	handler := New(Deps{APIToken: "secret", Catalog: &catalogFake{}, Eater: eaterFake{}})
	tests := []struct {
		name string
		path string
		body string
	}{
		{
			name: "product macro",
			path: "/products",
			body: `{"brand":"Acme","name":"Chicken","fat_per_100g":2,"carb_per_100g":0,"kcal_per_100g":100,"roles":["protein:lean"]}`,
		},
		{
			name: "eater goal",
			path: "/eaters",
			body: `{"name":"Pasha"}`,
		},
		{
			name: "macro goal range",
			path: "/eaters/1/goals",
			body: `{"protein_min_g":0,"protein_max_g":1,"fat_min_g":0,"fat_max_g":1,"carb_min_g":0,"kcal_min":0,"kcal_max":1}`,
		},
		{
			name: "fixed meal nested macros",
			path: "/eaters/1/fixed-meals",
			body: `{"meal_type":"breakfast","label":"Toast","macros":{"protein_g":0,"fat_g":0,"carb_g":0}}`,
		},
	}

	for i := range tests {
		test := &tests[i]
		t.Run(test.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, test.path, strings.NewReader(test.body))
			req.Header.Set("X-API-Token", "secret")
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
			}
			var response struct {
				Error string `json:"error"`
			}
			if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if response.Error != string(apperr.CodeValidation) {
				t.Fatalf("error = %q, want %q", response.Error, apperr.CodeValidation)
			}
		})
	}
}
