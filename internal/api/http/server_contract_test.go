package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go.uber.org/mock/gomock"

	"github.com/ZetoOfficial/meal-planner/api/sdk/go/model"
	httpapi "github.com/ZetoOfficial/meal-planner/internal/api/http"
	"github.com/ZetoOfficial/meal-planner/internal/api/http/mocks"
	catalogdom "github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
	"github.com/ZetoOfficial/meal-planner/internal/domain/meal"
	"github.com/ZetoOfficial/meal-planner/internal/domain/nutrition"
	planningdom "github.com/ZetoOfficial/meal-planner/internal/domain/planning"
	recipedom "github.com/ZetoOfficial/meal-planner/internal/domain/recipe"
	shoppingdom "github.com/ZetoOfficial/meal-planner/internal/domain/shopping"
	apperr "github.com/ZetoOfficial/meal-planner/internal/errors"
	"github.com/ZetoOfficial/meal-planner/internal/solver"
	"github.com/ZetoOfficial/meal-planner/internal/swap"
	usecasecatalog "github.com/ZetoOfficial/meal-planner/internal/usecase/catalog"
	usecaseplanning "github.com/ZetoOfficial/meal-planner/internal/usecase/planning"
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
	var product model.Product
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
	var response model.Error
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
	var response model.Error
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
	var response model.RecipeTemplate
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode recipe: %v", err)
	}
	if response.Id != 42 ||
		response.Status != model.Draft ||
		response.ComponentKind != model.ComponentKindProtein ||
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

func TestGeneratePlanHTTPContract(t *testing.T) {
	ctrl := gomock.NewController(t)
	planning := mocks.NewMockPlanningUseCases(ctrl)
	start := time.Date(2026, time.August, 10, 0, 0, 0, 0, time.UTC)

	var command usecaseplanning.GeneratePlanCommand
	planning.EXPECT().
		GeneratePlan(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, cmd usecaseplanning.GeneratePlanCommand) (usecaseplanning.PlanResult, error) {
			command = cmd
			score := 82.5
			servingG := 200.0
			return usecaseplanning.PlanResult{
				Plan: planningdom.Plan{
					ID:                   7,
					StartDate:            start,
					Days:                 1,
					CookDate:             start,
					Novelty:              2,
					Status:               planningdom.PlanStatusDraft,
					QualityScore:         &score,
					QualityWarningsCount: 1,
					EaterDays: []planningdom.EaterDay{{
						EaterID: 1, Date: start, MacroGoalID: 3,
						GeneratedMealTypes: []meal.Type{meal.TypeLunch},
					}},
					Recipes: []planningdom.PlanRecipe{{
						ID: 11, TemplateID: 5, ScaleFactor: 2,
						Fills: []planningdom.SlotFill{{SlotID: 9, ProductID: 4, PlannedInputQtyG: 200}},
					}},
					Meals: []planningdom.Meal{{
						ID: 21, EaterID: 1, Date: start, Type: meal.TypeLunch,
						Planned:    nutrition.Macros{ProteinG: 30, FatG: 10, CarbG: 40, Kcal: 400},
						Components: []planningdom.ComponentAllocation{{PlanRecipeIndex: 0, PortionFactor: 0.5, PlannedServingG: &servingG}},
					}},
				},
				Snapshots: []planningdom.QualitySnapshot{{
					EaterID: 1,
					Date:    start,
					Basis:   nutrition.SummaryBasisPlanned,
					Rolling: nutrition.RollingSummary{WindowDays: 7, RedMeatG: 120},
					DayRules: []nutrition.RuleEvaluationResult{{
						RuleCode: "fiber_min_g", Status: nutrition.StatusSoftFail,
						Unit: "g/day", Coverage: 1, Message: "fiber below target",
					}},
					RollingRules: []nutrition.RuleEvaluationResult{{
						RuleCode: "red_meat_max_weekly_g", Status: nutrition.StatusPass,
						Unit: "g/week", Coverage: 1, Message: "red meat within the weekly cap",
					}},
				}},
			}, nil
		})

	handler := httpapi.New(httpapi.Deps{APIToken: "secret", Planning: planning})
	body := `{"start_date":"2026-08-10","days":1,"novelty_level":2,"targets":[{"eater_id":1,"generated_meal_types":["lunch"]}]}`
	response := doRequest(t, handler, http.MethodPost, "/plans", body)

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s; want 201", response.Code, response.Body.String())
	}
	if command.Novelty != 2 || command.Days != 1 || len(command.Targets) != 1 {
		t.Fatalf("mapped command = %#v, want the request passed through", command)
	}

	var plan model.Plan
	if err := json.Unmarshal(response.Body.Bytes(), &plan); err != nil {
		t.Fatalf("decode plan: %v", err)
	}
	if plan.Id != 7 || plan.NoveltyLevel != 2 || plan.Status != model.PlanStatusDraft {
		t.Fatalf("plan = %#v, want the persisted draft", plan)
	}
	if len(plan.Recipes) != 1 || plan.Recipes[0].Id != 11 {
		t.Fatalf("plan recipes = %#v, want the stored plan recipe id", plan.Recipes)
	}
	if len(plan.Meals) != 1 || len(plan.Meals[0].Components) != 1 || plan.Meals[0].Components[0].PlanRecipeId != 11 {
		t.Fatalf("plan meals = %#v, want allocations addressed by plan recipe id", plan.Meals)
	}
	if plan.QualitySummary == nil {
		t.Fatal("plan has no quality summary, want the ranking evaluation exposed")
	}
	if plan.QualitySummary.QualityScore != 82.5 || plan.QualitySummary.WarningsCount != 1 {
		t.Fatalf("quality summary = %#v, want the stored score and warning count", plan.QualitySummary)
	}
	if len(plan.QualitySummary.RuleEvaluations) != 2 {
		t.Fatalf("rule evaluations = %#v, want the day and rolling verdicts", plan.QualitySummary.RuleEvaluations)
	}
}

func TestGeneratePlanReturnsSolverDiagnosticsAs422(t *testing.T) {
	ctrl := gomock.NewController(t)
	planning := mocks.NewMockPlanningUseCases(ctrl)
	start := time.Date(2026, time.August, 10, 0, 0, 0, 0, time.UTC)
	planning.EXPECT().
		GeneratePlan(gomock.Any(), gomock.Any()).
		Return(usecaseplanning.PlanResult{}, &usecaseplanning.InfeasibleError{Failure: solver.Failure{
			Code:       solver.FailureMacroInfeasible,
			EaterID:    1,
			Date:       start,
			Exhaustive: true,
			Rejections: []solver.HardRejection{{
				Code: solver.RejectionAllergen, EaterID: 1, ProductID: 4, Detail: "milk",
			}},
			MacroConstraints: []solver.MacroConstraint{{
				Code: solver.MacroProtein, TargetMin: 500, TargetMax: 600, ReachableMin: 0, ReachableMax: 60,
			}},
		}})

	handler := httpapi.New(httpapi.Deps{APIToken: "secret", Planning: planning})
	body := `{"start_date":"2026-08-10","targets":[{"eater_id":1,"generated_meal_types":["lunch"]}]}`
	response := doRequest(t, handler, http.MethodPost, "/plans", body)

	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, body = %s; want 422", response.Code, response.Body.String())
	}

	var solverError model.SolverError
	if err := json.Unmarshal(response.Body.Bytes(), &solverError); err != nil {
		t.Fatalf("decode solver error: %v", err)
	}
	if solverError.Code != model.MacroInfeasible || !solverError.Exhaustive || solverError.Message == "" {
		t.Fatalf("solver error = %#v, want an exhaustive macro infeasibility with a message", solverError)
	}
	if !hasConstraint(solverError.Diagnostics, model.HardExclusion) {
		t.Fatalf("diagnostics = %#v, want the allergen conflict reported", solverError.Diagnostics)
	}
	protein := findConstraint(solverError.Diagnostics, model.ProteinG)
	if protein == nil || protein.TargetMin == nil || *protein.TargetMin != 500 {
		t.Fatalf("diagnostics = %#v, want the unreachable protein bound", solverError.Diagnostics)
	}
	if protein.ReachableMax == nil || *protein.ReachableMax != 60 {
		t.Fatalf("protein diagnostic = %#v, want the reachable ceiling", protein)
	}
}

func TestArchivePlanHTTPContract(t *testing.T) {
	ctrl := gomock.NewController(t)
	planning := mocks.NewMockPlanningUseCases(ctrl)
	planning.EXPECT().ArchivePlan(gomock.Any(), planningdom.PlanID(7)).Return(nil)

	handler := httpapi.New(httpapi.Deps{APIToken: "secret", Planning: planning})
	response := doRequest(t, handler, http.MethodDelete, "/plans/7", "")

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, body = %s; want 204", response.Code, response.Body.String())
	}
}

func TestGetPlanMapsNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	planning := mocks.NewMockPlanningUseCases(ctrl)
	planning.EXPECT().
		GetPlan(gomock.Any(), planningdom.PlanID(7)).
		Return(usecaseplanning.PlanResult{}, apperr.NotFound("plan"))

	handler := httpapi.New(httpapi.Deps{APIToken: "secret", Planning: planning})
	response := doRequest(t, handler, http.MethodGet, "/plans/7", "")

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, body = %s; want 404", response.Code, response.Body.String())
	}
}

func hasConstraint(diagnostics []model.SolverDiagnostic, constraint model.SolverDiagnosticConstraint) bool {
	return findConstraint(diagnostics, constraint) != nil
}

func findConstraint(
	diagnostics []model.SolverDiagnostic,
	constraint model.SolverDiagnosticConstraint,
) *model.SolverDiagnostic {
	for i := range diagnostics {
		if diagnostics[i].Constraint == constraint {
			return &diagnostics[i]
		}
	}
	return nil
}

func doRequest(t *testing.T, handler http.Handler, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	request := httptest.NewRequest(method, target, reader)
	request.Header.Set("X-API-Token", "secret")
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func TestPreviewSwapHTTPContract(t *testing.T) {
	ctrl := gomock.NewController(t)
	planning := mocks.NewMockPlanningUseCases(ctrl)
	start := time.Date(2026, time.August, 10, 0, 0, 0, 0, time.UTC)

	var command usecaseplanning.SwapCommand
	planning.EXPECT().
		PreviewSwap(gomock.Any(), planningdom.PlanID(7), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ planningdom.PlanID, cmd usecaseplanning.SwapCommand) (swap.Preview, error) {
			command = cmd
			return swap.Preview{
				Level:         swap.LevelProteinChange,
				Summary:       "swap Salmon for Tuna",
				AffectedMeals: []planningdom.MealID{21},
				MacroDiff: []swap.EaterDayMacroDelta{{
					EaterID: 1, Date: start,
					Delta: swap.MacroDelta{ProteinG: 10, Kcal: 40},
				}},
				InvalidatedRecipes: []swap.InvalidatedRecipe{{PlanRecipeID: 12, Reason: "uses the roast"}},
				ShoppingDelta: swap.ShoppingDelta{
					Added:   []catalogdom.ProductID{3},
					Removed: []catalogdom.ProductID{1},
				},
				QualityDiff: swap.QualityDelta{
					FiberG:        -12,
					CoverageScore: -0.25,
					ChangedRules: []swap.RuleChange{{
						EaterID: 1, Date: start,
						Before: nutrition.RuleEvaluationResult{RuleCode: "fiber_min_g", Status: nutrition.StatusPass},
						After:  nutrition.RuleEvaluationResult{RuleCode: "fiber_min_g", Status: nutrition.StatusSoftFail, Coverage: 1},
					}},
				},
			}, nil
		})

	handler := httpapi.New(httpapi.Deps{APIToken: "secret", Planning: planning})
	body := `{"level":"fill","slot_fill_id":5,"new_product_id":3}`
	response := doRequest(t, handler, http.MethodPost, "/plans/7/swap/preview", body)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s; want 200", response.Code, response.Body.String())
	}
	if command.SlotFillID != 5 || command.NewProductID != 3 {
		t.Fatalf("mapped command = %#v, want the requested fill swap", command)
	}

	var preview model.SwapPreview
	if err := json.Unmarshal(response.Body.Bytes(), &preview); err != nil {
		t.Fatalf("decode preview: %v", err)
	}
	if preview.Level != "protein_change" || preview.Summary == "" {
		t.Fatalf("preview = %#v, want the blast radius level and summary", preview)
	}
	if preview.AffectedMeals == nil || len(*preview.AffectedMeals) != 1 {
		t.Fatalf("affected meals = %#v, want the meal eating from the batch", preview.AffectedMeals)
	}
	if preview.InvalidatedRecipes == nil || len(*preview.InvalidatedRecipes) != 1 {
		t.Fatalf("invalidated = %#v, want the dependent dish reported", preview.InvalidatedRecipes)
	}
	if preview.QualityDiff == nil || preview.QualityDiff.FiberG == nil || *preview.QualityDiff.FiberG != -12 {
		t.Fatalf("quality diff = %#v, want the fibre movement", preview.QualityDiff)
	}
	if preview.QualityDiff.ChangedRules == nil || len(*preview.QualityDiff.ChangedRules) != 1 {
		t.Fatalf("changed rules = %#v, want the moved verdict", preview.QualityDiff.ChangedRules)
	}
	if preview.ShoppingDelta == nil || preview.ShoppingDelta.Added == nil || len(*preview.ShoppingDelta.Added) != 1 {
		t.Fatalf("shopping delta = %#v, want the added product", preview.ShoppingDelta)
	}
	if preview.MacroDiff == nil || preview.MacroDiff.ByEater == nil || len(*preview.MacroDiff.ByEater) != 1 {
		t.Fatalf("macro diff = %#v, want the eater-day delta", preview.MacroDiff)
	}
}

func TestPreviewSwapRejectsUnsupportedLevels(t *testing.T) {
	handler := httpapi.New(httpapi.Deps{APIToken: "secret", Planning: mocks.NewMockPlanningUseCases(gomock.NewController(t))})

	response := doRequest(t, handler, http.MethodPost, "/plans/7/swap/preview", `{"level":"rotation"}`)

	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, body = %s; want 422 for an unsupported swap level", response.Code, response.Body.String())
	}
}

func TestApplySwapReturns409WithoutConfirmation(t *testing.T) {
	ctrl := gomock.NewController(t)
	planning := mocks.NewMockPlanningUseCases(ctrl)
	planning.EXPECT().
		ApplySwap(gomock.Any(), planningdom.PlanID(7), gomock.Any(), false).
		Return(usecaseplanning.PlanResult{}, &usecaseplanning.UnconfirmedSwapError{
			Preview: swap.Preview{Level: swap.LevelFillToFill, Summary: "confirm first"},
		})

	handler := httpapi.New(httpapi.Deps{APIToken: "secret", Planning: planning})
	body := `{"level":"fill","slot_fill_id":5,"new_product_id":3,"confirm":false}`
	response := doRequest(t, handler, http.MethodPost, "/plans/7/swap", body)

	if response.Code != http.StatusConflict {
		t.Fatalf("status = %d, body = %s; want 409", response.Code, response.Body.String())
	}
	var preview model.SwapPreview
	if err := json.Unmarshal(response.Body.Bytes(), &preview); err != nil {
		t.Fatalf("decode preview: %v", err)
	}
	if preview.Summary != "confirm first" {
		t.Fatalf("preview = %#v, want the pending blast radius returned", preview)
	}
}

func TestShoppingListHTTPContract(t *testing.T) {
	ctrl := gomock.NewController(t)
	shopping := mocks.NewMockShoppingUseCases(ctrl)
	cost := 4.0
	shopping.EXPECT().
		GetList(gomock.Any(), planningdom.PlanID(7)).
		Return(shoppingdom.List{
			ID: 3, PlanID: 7,
			Items: []shoppingdom.Item{{
				ID: 10, ProductID: 1, ProductName: "Chicken breast",
				TotalQtyG: 500, Unit: "g", EstCost: &cost, Checked: true,
			}},
		}, nil)

	handler := httpapi.New(httpapi.Deps{APIToken: "secret", Shopping: shopping})
	response := doRequest(t, handler, http.MethodGet, "/plans/7/shopping-list", "")

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s; want 200", response.Code, response.Body.String())
	}
	var list model.ShoppingList
	if err := json.Unmarshal(response.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if list.Id != 3 || len(list.Items) != 1 {
		t.Fatalf("list = %#v, want the stored list", list)
	}
	if list.Items[0].TotalQtyG != 500 || !list.Items[0].Checked {
		t.Fatalf("item = %#v, want the aggregated quantity and its check", list.Items[0])
	}
	if list.EstTotal == nil || *list.EstTotal != 4 {
		t.Fatalf("estimated total = %v, want the summed known costs", list.EstTotal)
	}
}

func TestUpdateShoppingListItemHTTPContract(t *testing.T) {
	ctrl := gomock.NewController(t)
	shopping := mocks.NewMockShoppingUseCases(ctrl)
	shopping.EXPECT().SetChecked(gomock.Any(), shoppingdom.ItemID(10), true).Return(nil)
	shopping.EXPECT().
		GetItem(gomock.Any(), shoppingdom.ItemID(10)).
		Return(shoppingdom.Item{ID: 10, ProductID: 1, TotalQtyG: 500, Unit: "g", Checked: true}, nil)

	handler := httpapi.New(httpapi.Deps{APIToken: "secret", Shopping: shopping})
	response := doRequest(t, handler, http.MethodPatch, "/shopping-list-items/10", `{"checked":true}`)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s; want 200", response.Code, response.Body.String())
	}
}
