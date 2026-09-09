//go:build integration

package httpapi_test

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ZetoOfficial/meal-planner/api/sdk/go/model"
	httpapi "github.com/ZetoOfficial/meal-planner/internal/api/http"
	"github.com/ZetoOfficial/meal-planner/internal/clock"
	catalogdom "github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
	eaterdom "github.com/ZetoOfficial/meal-planner/internal/domain/eater"
	recipedom "github.com/ZetoOfficial/meal-planner/internal/domain/recipe"
	postgresdb "github.com/ZetoOfficial/meal-planner/internal/postgres"
	analyticsrepo "github.com/ZetoOfficial/meal-planner/internal/postgres/analytics"
	catalogrepo "github.com/ZetoOfficial/meal-planner/internal/postgres/catalog"
	eaterrepo "github.com/ZetoOfficial/meal-planner/internal/postgres/eater"
	feedbackrepo "github.com/ZetoOfficial/meal-planner/internal/postgres/feedback"
	planningrepo "github.com/ZetoOfficial/meal-planner/internal/postgres/planning"
	reciperepo "github.com/ZetoOfficial/meal-planner/internal/postgres/recipe"
	shoppingrepo "github.com/ZetoOfficial/meal-planner/internal/postgres/shopping"
	"github.com/ZetoOfficial/meal-planner/internal/solver"
	testpostgres "github.com/ZetoOfficial/meal-planner/internal/testkit/postgres"
	usecasecatalog "github.com/ZetoOfficial/meal-planner/internal/usecase/catalog"
	usecaseeater "github.com/ZetoOfficial/meal-planner/internal/usecase/eater"
	usecaseplanning "github.com/ZetoOfficial/meal-planner/internal/usecase/planning"
	usecasereality "github.com/ZetoOfficial/meal-planner/internal/usecase/reality"
	usecaserecipe "github.com/ZetoOfficial/meal-planner/internal/usecase/recipe"
	usecaseshopping "github.com/ZetoOfficial/meal-planner/internal/usecase/shopping"
)

const apiToken = "smoke-token"

// TestOwnerWorkflowSmoke walks the whole MVP path over HTTP against a migrated,
// seeded database: read the catalogue, generate a plan without an LLM, shop from
// it, log what was eaten and read the analytics back.
func TestOwnerWorkflowSmoke(t *testing.T) {
	handler, database := newSmokeAPI(t)
	ctx := context.Background()

	products := decodeList[model.Product](t, call(t, handler, http.MethodGet, "/products", ""))
	if len(products) < 9 {
		t.Fatalf("seeded products = %d, want the demo catalogue", len(products))
	}
	roleTags := decodeList[model.RoleTag](t, call(t, handler, http.MethodGet, "/role-tags", ""))
	if len(roleTags) == 0 {
		t.Fatal("no role tags, want the seeded taxonomy")
	}
	recipes := decodeList[model.RecipeTemplate](t, call(t, handler, http.MethodGet, "/recipes?status=permanent", ""))
	if len(recipes) < 4 {
		t.Fatalf("permanent recipes = %d, want a reusable library", len(recipes))
	}

	eaterID := seededEaterID(t, ctx, database, "Pasha")
	start := time.Now().UTC().Truncate(24 * time.Hour).Format(time.DateOnly)
	generate := `{"start_date":"` + start + `","days":2,"novelty_level":0,"targets":[` +
		`{"eater_id":` + itoa(eaterID) + `,"generated_meal_types":["lunch","dinner"]}]}`

	response := call(t, handler, http.MethodPost, "/plans", generate)
	if response.Code != http.StatusCreated {
		t.Fatalf("generate plan status = %d, body = %s; want 201", response.Code, response.Body.String())
	}
	plan := decode[model.Plan](t, response)
	if plan.Id == 0 || len(plan.Meals) == 0 || len(plan.Recipes) == 0 {
		t.Fatalf("generated plan = %#v, want a persisted plan with meals and batches", plan)
	}
	if plan.QualitySummary == nil {
		t.Fatal("generated plan has no quality summary, want the evaluation that ranked it")
	}

	planPath := "/plans/" + itoa(plan.Id)
	reloaded := decode[model.Plan](t, call(t, handler, http.MethodGet, planPath, ""))
	if len(reloaded.Meals) != len(plan.Meals) {
		t.Fatalf("reloaded plan = %d meals, want the %d it was saved with", len(reloaded.Meals), len(plan.Meals))
	}

	adjusted := lockPlannedQuantity(t, handler, planPath, reloaded)
	swapped := swapOneFill(t, handler, planPath, adjusted, products)
	if !swapped.ShoppingListStale {
		t.Fatal("plan after swap is not marked shopping_list_stale, want the list flagged for regeneration")
	}

	list := decode[model.ShoppingList](t, call(t, handler, http.MethodGet, planPath+"/shopping-list", ""))
	if len(list.Items) == 0 {
		t.Fatal("shopping list is empty, want the plan's products")
	}
	checked := call(t, handler, http.MethodPatch, "/shopping-list-items/"+itoa(list.Items[0].Id), `{"checked":true}`)
	if checked.Code != http.StatusOK {
		t.Fatalf("check item status = %d, body = %s; want 200", checked.Code, checked.Body.String())
	}
	regenerated := decode[model.ShoppingList](t, call(t, handler, http.MethodPost, planPath+"/shopping-list", ""))
	if !itemChecked(regenerated, list.Items[0].ProductId) {
		t.Fatal("regeneration lost the check of an unchanged row")
	}

	meals := decodeList[model.Meal](t, call(t, handler, http.MethodGet, planPath+"/meals", ""))
	if len(meals) == 0 {
		t.Fatal("plan has no meals to log")
	}
	logBody := `{"log_status":"eaten_modified","logged_by":` + itoa(eaterID) +
		`,"actual":{"protein_g":40,"fat_g":20,"carb_g":50,"kcal":560}}`
	logged := call(t, handler, http.MethodPost, "/meals/"+itoa(meals[0].Id)+"/log", logBody)
	if logged.Code != http.StatusOK {
		t.Fatalf("log meal status = %d, body = %s; want 200", logged.Code, logged.Body.String())
	}
	loggedMeal := decode[model.Meal](t, logged)
	if loggedMeal.Actual == nil || loggedMeal.Actual.Kcal != 560 {
		t.Fatalf("logged meal = %#v, want the actual macros stored", loggedMeal)
	}

	feedback := call(t, handler, http.MethodPost, "/feedback",
		`{"eater_id":`+itoa(eaterID)+`,"target_type":"recipe_template","target_id":`+
			itoa(plan.Recipes[0].TemplateId)+`,"rating":1,"comment":"worked well"}`)
	if feedback.Code != http.StatusCreated {
		t.Fatalf("feedback status = %d, body = %s; want 201", feedback.Code, feedback.Body.String())
	}

	period := "?eaterId=" + itoa(eaterID) + "&from=" + start + "&to=" + start
	macros := decode[model.MacroSummary](t, call(t, handler, http.MethodGet, "/analytics/macros"+period, ""))
	if macros.AvgPlanned.Kcal == 0 {
		t.Fatalf("macro analytics = %#v, want the planned average", macros)
	}
	if macros.AvgActual == nil {
		t.Fatal("macro analytics has no actual average, want the logged day counted")
	}

	quality := call(t, handler, http.MethodGet, "/analytics/nutrition-quality"+period+"&basis=actual", "")
	if quality.Code != http.StatusOK {
		t.Fatalf("quality analytics status = %d, body = %s; want 200", quality.Code, quality.Body.String())
	}
	top := decodeList[model.TopRecipe](t, call(t, handler, http.MethodGet, "/analytics/top-recipes?limit=5", ""))
	if len(top) == 0 {
		t.Fatal("top recipes is empty, want the seeded library ranked")
	}

	archived := call(t, handler, http.MethodDelete, planPath, "")
	if archived.Code != http.StatusNoContent {
		t.Fatalf("archive status = %d, body = %s; want 204", archived.Code, archived.Body.String())
	}
}

// lockPlannedQuantity exercises FR-3/FR-5 over HTTP: preview the locked
// quantity, then apply it with confirm=true and check the plan really moved.
func lockPlannedQuantity(t *testing.T, handler http.Handler, planPath string, plan model.Plan) model.Plan {
	t.Helper()
	recipe, fill := firstFill(t, plan)
	locked := float64(fill.PlannedInputQtyG) * 1.2
	body := `{"slot_fill_id":` + itoa(*fill.Id) + `,"planned_input_qty_g":` +
		strconv.FormatFloat(locked, 'f', 1, 64) + `,"confirm":true}`

	preview := decode[model.QuantityAdjustmentPreview](t,
		call(t, handler, http.MethodPost, planPath+"/quantity/preview", body))
	if preview.Summary == "" {
		t.Fatalf("quantity preview = %#v, want a readable summary", preview)
	}

	response := call(t, handler, http.MethodPost, planPath+"/quantity", body)
	if response.Code != http.StatusOK {
		t.Fatalf("apply quantity status = %d, body = %s; want 200", response.Code, response.Body.String())
	}
	applied := decode[model.Plan](t, response)
	stored := fillByID(t, applied, *fill.Id)
	if math.Abs(float64(stored.PlannedInputQtyG)-locked) > 0.05 {
		t.Fatalf("stored quantity = %v, want the locked %v g", stored.PlannedInputQtyG, locked)
	}
	if !applied.ShoppingListStale {
		t.Fatalf("plan recipe %d adjusted but the shopping list is not stale", recipe.Id)
	}
	return applied
}

// swapOneFill exercises FR-4: preview the blast radius of a fill swap and then
// apply it with confirm=true.
func swapOneFill(t *testing.T, handler http.Handler, planPath string, plan model.Plan, products []model.Product) model.Plan {
	t.Helper()
	fillID, newProductID := swapCandidate(t, plan, products)
	body := `{"level":"fill","slot_fill_id":` + itoa(fillID) +
		`,"new_product_id":` + itoa(newProductID) + `,"confirm":true}`

	preview := decode[model.SwapPreview](t,
		call(t, handler, http.MethodPost, planPath+"/swap/preview", body))
	if preview.Summary == "" {
		t.Fatalf("swap preview = %#v, want a readable blast radius", preview)
	}

	response := call(t, handler, http.MethodPost, planPath+"/swap", body)
	if response.Code != http.StatusOK {
		t.Fatalf("apply swap status = %d, body = %s; want 200", response.Code, response.Body.String())
	}
	applied := decode[model.Plan](t, response)
	if fillByID(t, applied, fillID).ProductId != newProductID {
		t.Fatalf("swapped fill still points at the old product, want %d", newProductID)
	}
	return applied
}

// swapCandidate finds a fill whose product shares a role tag with another
// purchasable product, so the swap stays inside the slot's role.
func swapCandidate(t *testing.T, plan model.Plan, products []model.Product) (int64, int64) {
	t.Helper()
	byID := make(map[int64]model.Product, len(products))
	for _, product := range products {
		byID[product.Id] = product
	}
	for _, recipe := range plan.Recipes {
		for _, fill := range recipe.Fills {
			if fill.Id == nil {
				continue
			}
			current, known := byID[fill.ProductId]
			if !known || current.Roles == nil {
				continue
			}
			for _, candidate := range products {
				if candidate.Id == fill.ProductId || candidate.Roles == nil {
					continue
				}
				if shareRole(*current.Roles, *candidate.Roles) {
					return *fill.Id, candidate.Id
				}
			}
		}
	}
	t.Fatal("no fill in the plan has an alternative product with the same role")
	return 0, 0
}

func shareRole(left, right []string) bool {
	for _, l := range left {
		for _, r := range right {
			if l == r {
				return true
			}
		}
	}
	return false
}

func firstFill(t *testing.T, plan model.Plan) (model.PlanRecipe, model.SlotFill) {
	t.Helper()
	for _, recipe := range plan.Recipes {
		for _, fill := range recipe.Fills {
			if fill.Id != nil {
				return recipe, fill
			}
		}
	}
	t.Fatal("plan has no persisted slot fill")
	return model.PlanRecipe{}, model.SlotFill{}
}

func fillByID(t *testing.T, plan model.Plan, id int64) model.SlotFill {
	t.Helper()
	for _, recipe := range plan.Recipes {
		for _, fill := range recipe.Fills {
			if fill.Id != nil && *fill.Id == id {
				return fill
			}
		}
	}
	t.Fatalf("plan has no slot fill %d", id)
	return model.SlotFill{}
}

func newSmokeAPI(t *testing.T) (http.Handler, testpostgres.Database) {
	t.Helper()
	database := testpostgres.RequireDatabase(t)
	testpostgres.ApplySeed(t, database)

	store := postgresdb.NewStore(database.Pool)
	transactions := postgresdb.NewTxManager(database.Pool)
	eaters := eaterrepo.New(store)
	products := catalogrepo.New(store)
	recipes := reciperepo.New(store)
	plans := planningrepo.New(store)

	// No generator: the smoke path must work reuse-only, without an LLM.
	handler := httpapi.New(httpapi.Deps{
		APIToken: apiToken,
		Catalog:  usecasecatalog.NewService(products, transactions),
		Eater:    usecaseeater.NewService(eaters, transactions, clock.System{}),
		Recipe:   usecaserecipe.NewService(recipes, transactions, clock.System{}),
		Planning: usecaseplanning.NewService(
			plans, eaters, products, recipes, solver.New(), nil, transactions,
			usecaseplanning.Options{AvailableEquipment: []string{"stove", "oven"}, RollingWindowDays: 7},
		),
		Shopping: usecaseshopping.NewService(shoppingrepo.New(store), plans, products, transactions),
		Reality: usecasereality.NewService(
			plans, feedbackrepo.New(store), analyticsrepo.New(store),
			smokeReference{products: products, recipes: recipes, eaters: eaters},
			transactions, clock.System{},
		),
	})
	return handler, database
}

type smokeReference struct {
	products *catalogrepo.Repository
	recipes  *reciperepo.Repository
	eaters   *eaterrepo.Repository
}

func (r smokeReference) ListProducts(ctx context.Context) ([]catalogdom.Product, error) {
	return r.products.List(ctx, "")
}

func (r smokeReference) ListRecipeTemplates(ctx context.Context) ([]recipedom.Template, error) {
	return r.recipes.List(ctx, nil)
}

func (r smokeReference) GetEater(ctx context.Context, id eaterdom.ID) (eaterdom.Eater, error) {
	return r.eaters.Get(ctx, id)
}

func (r smokeReference) ListFixedMeals(ctx context.Context, id eaterdom.ID) ([]eaterdom.FixedMeal, error) {
	return r.eaters.ListFixedMeals(ctx, id)
}

func seededEaterID(t *testing.T, ctx context.Context, database testpostgres.Database, name string) int64 {
	t.Helper()
	var id int64
	if err := database.Pool.QueryRow(ctx, "SELECT id FROM eater WHERE name = $1", name).Scan(&id); err != nil {
		t.Fatalf("find seeded eater %s: %v", name, err)
	}
	return id
}

func call(t *testing.T, handler http.Handler, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	request.Header.Set("X-API-Token", apiToken)
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func decode[T any](t *testing.T, response *httptest.ResponseRecorder) T {
	t.Helper()
	if response.Code < 200 || response.Code >= 300 {
		t.Fatalf("unexpected status %d: %s", response.Code, response.Body.String())
	}
	var value T
	if err := json.Unmarshal(response.Body.Bytes(), &value); err != nil {
		t.Fatalf("decode response: %v (body %s)", err, response.Body.String())
	}
	return value
}

func decodeList[T any](t *testing.T, response *httptest.ResponseRecorder) []T {
	t.Helper()
	return decode[[]T](t, response)
}

func itemChecked(list model.ShoppingList, productID int64) bool {
	for _, item := range list.Items {
		if item.ProductId == productID {
			return item.Checked
		}
	}
	return false
}

func itoa(value int64) string {
	return strconv.FormatInt(value, 10)
}
