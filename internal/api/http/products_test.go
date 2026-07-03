package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"

	api "github.com/ZetoOfficial/meal-planner/internal/api/openapi"
	catalogdom "github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
	apperr "github.com/ZetoOfficial/meal-planner/internal/errors"
	cataloguc "github.com/ZetoOfficial/meal-planner/internal/usecase/catalog"
)

func TestProductCRUDHandlers(t *testing.T) {
	repo := newMemoryProductRepo()
	handler := New(Deps{
		APIToken: "secret",
		Catalog:  cataloguc.NewService(repo),
	})

	created := requestJSON[api.Product](t, handler, http.MethodPost, "/products", productRequestBody(), http.StatusCreated)
	if created.Id == 0 {
		t.Fatal("created product id is zero")
	}
	if created.Roles == nil || len(*created.Roles) != 2 {
		t.Fatalf("created roles = %#v, want 2 roles", created.Roles)
	}
	if created.Allergens == nil || len(*created.Allergens) != 2 {
		t.Fatalf("created allergens = %#v, want 2 allergens", created.Allergens)
	}
	if created.Nutrition == nil || created.Nutrition.FiberPer100g == nil || *created.Nutrition.FiberPer100g != 2.5 {
		t.Fatalf("fiber not round-tripped: %#v", created.Nutrition)
	}

	listed := requestJSON[[]api.Product](t, handler, http.MethodGet, "/products?role=protein:lean:breast", nil, http.StatusOK)
	if len(listed) != 1 || listed[0].Id != created.Id {
		t.Fatalf("listed products = %#v, want created product", listed)
	}

	got := requestJSON[api.Product](t, handler, http.MethodGet, "/products/1", nil, http.StatusOK)
	if got.Id != created.Id {
		t.Fatalf("got id = %d, want %d", got.Id, created.Id)
	}

	updateBody := productRequestBody()
	updateBody["name"] = "updated breast"
	updateBody["roles"] = []string{"protein:lean:thigh"}
	updated := requestJSON[api.Product](t, handler, http.MethodPut, "/products/1", updateBody, http.StatusOK)
	if updated.Name != "updated breast" {
		t.Fatalf("updated name = %q", updated.Name)
	}
	if updated.Roles == nil || len(*updated.Roles) != 1 || (*updated.Roles)[0] != "protein:lean:thigh" {
		t.Fatalf("updated roles = %#v", updated.Roles)
	}

	requestNoBody(t, handler, http.MethodDelete, "/products/1", http.StatusNoContent)
	requestNoBody(t, handler, http.MethodGet, "/products/1", http.StatusNotFound)
}

func TestCreateProductValidationError(t *testing.T) {
	repo := newMemoryProductRepo()
	handler := New(Deps{APIToken: "secret", Catalog: cataloguc.NewService(repo)})

	body := productRequestBody()
	body["brand"] = ""
	body["roles"] = []string{}
	body["fiber_per_100g"] = -1

	var response api.Error
	rec := doRequest(t, handler, http.MethodPost, "/products", body)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Error != string(apperr.CodeValidation) {
		t.Fatalf("error = %q, want validation", response.Error)
	}
}

func productRequestBody() map[string]any {
	return map[string]any{
		"brand":            "Test Brand",
		"name":             "chicken breast",
		"protein_per_100g": 23.5,
		"fat_per_100g":     1.2,
		"carb_per_100g":    0,
		"kcal_per_100g":    110,
		"price_per_100g":   55.5,
		"default_unit":     "g",
		"purchasable":      true,
		"roles":            []string{"protein:lean:breast", "protein:lean"},
		"allergens":        []string{"casein", "nuts"},
		"nutrition": map[string]any{
			"fiber_per_100g": 2.5,
		},
		"data_quality": map[string]any{
			"nutrition_data_quality":   "label_exact",
			"nutrition_coverage_score": 0.5,
			"source_kind":              "pack_label",
			"source_note":              "label",
		},
		"classification": map[string]any{
			"food_group":            "white_meat",
			"processing_level":      "minimally_processed",
			"processing_confidence": 0.9,
			"is_red_meat":           false,
		},
	}
}

func requestJSON[T any](t *testing.T, handler http.Handler, method, path string, body any, status int) T {
	t.Helper()
	rec := doRequest(t, handler, method, path, body)
	if rec.Code != status {
		t.Fatalf("%s %s status = %d, want %d; body=%s", method, path, rec.Code, status, rec.Body.String())
	}
	var out T
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return out
}

func requestNoBody(t *testing.T, handler http.Handler, method, path string, status int) {
	t.Helper()
	rec := doRequest(t, handler, method, path, nil)
	if rec.Code != status {
		t.Fatalf("%s %s status = %d, want %d; body=%s", method, path, rec.Code, status, rec.Body.String())
	}
}

func doRequest(t *testing.T, handler http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var payload bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&payload).Encode(body); err != nil {
			t.Fatalf("encode request: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, &payload)
	req.Header.Set("X-API-Token", "secret")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

type memoryProductRepo struct {
	nextID  int64
	records map[catalogdom.ProductID]catalogdom.Product
}

func newMemoryProductRepo() *memoryProductRepo {
	return &memoryProductRepo{nextID: 1, records: map[catalogdom.ProductID]catalogdom.Product{}}
}

func (r *memoryProductRepo) List(_ context.Context, filter cataloguc.ProductFilter) ([]catalogdom.Product, error) {
	out := make([]catalogdom.Product, 0, len(r.records))
	for _, product := range r.records {
		if filter.Role != nil && *filter.Role != "" && !contains(product.Roles, *filter.Role) {
			continue
		}
		out = append(out, product)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out, nil
}

func (r *memoryProductRepo) ListUsable(ctx context.Context) ([]catalogdom.Product, error) {
	return r.List(ctx, cataloguc.ProductFilter{})
}

func (r *memoryProductRepo) Get(_ context.Context, id catalogdom.ProductID) (catalogdom.Product, error) {
	product, ok := r.records[id]
	if !ok {
		return catalogdom.Product{}, apperr.NotFound("product")
	}
	return product, nil
}

func (r *memoryProductRepo) Create(_ context.Context, p catalogdom.Product) (catalogdom.Product, error) {
	p.ID = catalogdom.ProductID(r.nextID)
	r.nextID++
	r.records[p.ID] = p
	return p, nil
}

func (r *memoryProductRepo) Update(_ context.Context, p catalogdom.Product) (catalogdom.Product, error) {
	if _, ok := r.records[p.ID]; !ok {
		return catalogdom.Product{}, apperr.NotFound("product")
	}
	r.records[p.ID] = p
	return p, nil
}

func (r *memoryProductRepo) Delete(_ context.Context, id catalogdom.ProductID) error {
	if _, ok := r.records[id]; !ok {
		return apperr.NotFound("product")
	}
	delete(r.records, id)
	return nil
}

func (r *memoryProductRepo) ListRoleTags(context.Context) ([]catalogdom.RoleTag, error) {
	return nil, nil
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
