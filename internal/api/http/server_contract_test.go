package httpapi_test

import (
	"context"
	"encoding/json"
	"errors"
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
	feedbackdom "github.com/ZetoOfficial/meal-planner/internal/domain/feedback"
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
	usecasereality "github.com/ZetoOfficial/meal-planner/internal/usecase/reality"
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

func TestUpdateShoppingListItemDoesNotHideAFailedRead(t *testing.T) {
	ctrl := gomock.NewController(t)
	shopping := mocks.NewMockShoppingUseCases(ctrl)
	shopping.EXPECT().SetChecked(gomock.Any(), shoppingdom.ItemID(10), true).Return(nil)
	shopping.EXPECT().
		GetItem(gomock.Any(), shoppingdom.ItemID(10)).
		Return(shoppingdom.Item{}, errors.New("connection reset by peer"))

	handler := httpapi.New(httpapi.Deps{APIToken: "secret", Shopping: shopping})
	response := doRequest(t, handler, http.MethodPatch, "/shopping-list-items/10", `{"checked":true}`)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, body = %s; want 500, not a 404 that claims the item is gone",
			response.Code, response.Body.String())
	}
}

func TestLogMealReportsThePlanItBelongsTo(t *testing.T) {
	ctrl := gomock.NewController(t)
	reality := mocks.NewMockRealityUseCases(ctrl)
	reality.EXPECT().
		LogMeal(gomock.Any(), planningdom.MealID(21), gomock.Any()).
		Return(planningdom.Meal{
			ID:        21,
			PlanID:    7,
			EaterID:   1,
			Date:      time.Date(2026, time.August, 10, 0, 0, 0, 0, time.UTC),
			Type:      meal.TypeLunch,
			LogStatus: planningdom.MealLogStatusEatenAsPlanned,
		}, nil)

	handler := httpapi.New(httpapi.Deps{APIToken: "secret", Reality: reality})
	response := doRequest(t, handler, http.MethodPost, "/meals/21/log",
		`{"log_status":"eaten_as_planned","logged_by":1}`)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s; want 200", response.Code, response.Body.String())
	}
	var logged model.Meal
	if err := json.Unmarshal(response.Body.Bytes(), &logged); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if logged.PlanId != 7 {
		t.Fatalf("plan id = %d, want the plan the meal belongs to", logged.PlanId)
	}
}

func TestPreviewQuantityAdjustmentHTTPContract(t *testing.T) {
	ctrl := gomock.NewController(t)
	planning := mocks.NewMockPlanningUseCases(ctrl)
	start := time.Date(2026, time.August, 10, 0, 0, 0, 0, time.UTC)

	var command usecaseplanning.QuantityCommand
	planning.EXPECT().
		PreviewQuantityAdjustment(gomock.Any(), planningdom.PlanID(7), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ planningdom.PlanID, cmd usecaseplanning.QuantityCommand) (swap.QuantityPreview, error) {
			command = cmd
			return swap.QuantityPreview{
				Summary:       "lock 1200 g to 988 g",
				AffectedMeals: []planningdom.MealID{21},
				PortionChanges: []swap.PortionChange{{
					MealID: 21, EaterID: 1, Date: start, Before: 0.5, After: 0.6,
				}},
				MacroDiff: []swap.EaterDayMacroDelta{{
					EaterID: 1, Date: start, Delta: swap.MacroDelta{ProteinG: -2},
				}},
				ShoppingDiff: []swap.ShoppingQuantityChange{{ProductID: 1, BeforeG: 1200, AfterG: 988}},
				Warnings:     []string{"40% of the batch stays unallocated"},
			}, nil
		})

	handler := httpapi.New(httpapi.Deps{APIToken: "secret", Planning: planning})
	body := `{"slot_fill_id":5,"planned_input_qty_g":988}`
	response := doRequest(t, handler, http.MethodPost, "/plans/7/quantity/preview", body)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s; want 200", response.Code, response.Body.String())
	}
	if command.SlotFillID != 5 || command.PlannedInputQtyG != 988 {
		t.Fatalf("mapped command = %#v, want the locked quantity", command)
	}

	var preview model.QuantityAdjustmentPreview
	if err := json.Unmarshal(response.Body.Bytes(), &preview); err != nil {
		t.Fatalf("decode preview: %v", err)
	}
	if len(preview.PortionChanges) != 1 || preview.PortionChanges[0].After != 0.6 {
		t.Fatalf("portion changes = %#v, want the rebalanced share", preview.PortionChanges)
	}
	if len(preview.ShoppingDiff) != 1 || preview.ShoppingDiff[0].AfterG != 988 {
		t.Fatalf("shopping diff = %#v, want the purchased quantity", preview.ShoppingDiff)
	}
	if len(preview.Warnings) != 1 {
		t.Fatalf("warnings = %#v, want the leftover reported", preview.Warnings)
	}
}

func TestQuantityAdjustmentReportsInfeasibilityAs422(t *testing.T) {
	ctrl := gomock.NewController(t)
	planning := mocks.NewMockPlanningUseCases(ctrl)
	planning.EXPECT().
		PreviewQuantityAdjustment(gomock.Any(), planningdom.PlanID(7), gomock.Any()).
		Return(swap.QuantityPreview{}, &swap.InfeasibleQuantityError{
			EaterID: 1,
			Constraints: []solver.MacroConstraint{{
				Code: solver.MacroProtein, TargetMin: 130, TargetMax: 145, ReachableMax: 11,
			}},
		})

	handler := httpapi.New(httpapi.Deps{APIToken: "secret", Planning: planning})
	response := doRequest(t, handler, http.MethodPost, "/plans/7/quantity/preview", `{"slot_fill_id":5,"planned_input_qty_g":50}`)

	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, body = %s; want 422", response.Code, response.Body.String())
	}
	var solverError model.SolverError
	if err := json.Unmarshal(response.Body.Bytes(), &solverError); err != nil {
		t.Fatalf("decode solver error: %v", err)
	}
	if len(solverError.Diagnostics) != 1 || solverError.Diagnostics[0].Constraint != model.ProteinG {
		t.Fatalf("diagnostics = %#v, want the unsatisfied protein bound", solverError.Diagnostics)
	}
}

func TestApplyQuantityAdjustmentNeedsConfirmation(t *testing.T) {
	ctrl := gomock.NewController(t)
	planning := mocks.NewMockPlanningUseCases(ctrl)
	planning.EXPECT().
		ApplyQuantityAdjustment(gomock.Any(), planningdom.PlanID(7), gomock.Any(), false).
		Return(usecaseplanning.PlanResult{}, &usecaseplanning.UnconfirmedQuantityError{
			Preview: swap.QuantityPreview{Summary: "confirm first"},
		})

	handler := httpapi.New(httpapi.Deps{APIToken: "secret", Planning: planning})
	response := doRequest(t, handler, http.MethodPost, "/plans/7/quantity", `{"slot_fill_id":5,"planned_input_qty_g":988}`)

	if response.Code != http.StatusConflict {
		t.Fatalf("status = %d, body = %s; want 409", response.Code, response.Body.String())
	}
}

func TestLogMealHTTPContract(t *testing.T) {
	ctrl := gomock.NewController(t)
	reality := mocks.NewMockRealityUseCases(ctrl)
	actual := nutrition.Macros{ProteinG: 25, FatG: 12, CarbG: 35, Kcal: 380}

	var log planningdom.MealLog
	reality.EXPECT().
		LogMeal(gomock.Any(), planningdom.MealID(21), gomock.Any()).
		DoAndReturn(func(_ context.Context, id planningdom.MealID, l planningdom.MealLog) (planningdom.Meal, error) {
			log = l
			return planningdom.Meal{
				ID: id, EaterID: 1, Type: meal.TypeLunch,
				LogStatus: planningdom.MealLogStatusEatenModified,
				Planned:   nutrition.Macros{ProteinG: 30, FatG: 10, CarbG: 40, Kcal: 400},
				Actual:    &actual,
			}, nil
		})

	handler := httpapi.New(httpapi.Deps{APIToken: "secret", Reality: reality})
	body := `{"log_status":"eaten_modified","logged_by":1,"actual":{"protein_g":25,"fat_g":12,"carb_g":35,"kcal":380}}`
	response := doRequest(t, handler, http.MethodPost, "/meals/21/log", body)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s; want 200", response.Code, response.Body.String())
	}
	if log.Status != planningdom.MealLogStatusEatenModified || log.Actual == nil {
		t.Fatalf("mapped log = %#v, want the modified meal with its actual macros", log)
	}

	var logged model.Meal
	if err := json.Unmarshal(response.Body.Bytes(), &logged); err != nil {
		t.Fatalf("decode meal: %v", err)
	}
	if logged.Actual == nil || logged.Actual.Kcal != 380 {
		t.Fatalf("meal = %#v, want the actual macros echoed", logged)
	}
}

func TestCreateFeedbackHTTPContract(t *testing.T) {
	ctrl := gomock.NewController(t)
	reality := mocks.NewMockRealityUseCases(ctrl)
	rating := -1
	delta := -0.1
	reality.EXPECT().
		CreateFeedback(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, event feedbackdom.Event) (feedbackdom.Event, error) {
			event.ID = 3
			event.WeightDelta = &delta
			return event, nil
		})

	handler := httpapi.New(httpapi.Deps{APIToken: "secret", Reality: reality})
	body := `{"eater_id":1,"target_type":"recipe_template","target_id":5,"rating":-1,"comment":"too salty"}`
	response := doRequest(t, handler, http.MethodPost, "/feedback", body)

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s; want 201", response.Code, response.Body.String())
	}
	var event model.FeedbackEvent
	if err := json.Unmarshal(response.Body.Bytes(), &event); err != nil {
		t.Fatalf("decode feedback: %v", err)
	}
	if event.Rating == nil || *event.Rating != rating {
		t.Fatalf("event = %#v, want the rating stored", event)
	}
	if event.WeightDelta == nil || *event.WeightDelta != -0.1 {
		t.Fatalf("event = %#v, want the bounded weight nudge reported", event)
	}
	if event.Comment == nil || *event.Comment != "too salty" {
		t.Fatalf("event = %#v, want the comment stored", event)
	}
}

func TestMacroAnalyticsHTTPContract(t *testing.T) {
	ctrl := gomock.NewController(t)
	reality := mocks.NewMockRealityUseCases(ctrl)
	from := time.Date(2026, time.August, 10, 0, 0, 0, 0, time.UTC)
	reality.EXPECT().
		MacroAnalytics(gomock.Any(), gomock.Any()).
		Return(usecasereality.MacroAnalytics{
			Period:     usecasereality.Period{EaterID: 1, From: from, To: from.AddDate(0, 0, 6)},
			Days:       7,
			AvgPlanned: nutrition.Macros{ProteinG: 120, FatG: 60, CarbG: 200, Kcal: 1800},
			AvgActual:  nutrition.Macros{ProteinG: 110, FatG: 62, CarbG: 190, Kcal: 1750},
			HasActual:  true,
		}, nil)

	handler := httpapi.New(httpapi.Deps{APIToken: "secret", Reality: reality})
	response := doRequest(t, handler, http.MethodGet, "/analytics/macros?eaterId=1&from=2026-08-10&to=2026-08-16", "")

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s; want 200", response.Code, response.Body.String())
	}
	var summary model.MacroSummary
	if err := json.Unmarshal(response.Body.Bytes(), &summary); err != nil {
		t.Fatalf("decode summary: %v", err)
	}
	if summary.AvgPlanned.Kcal != 1800 || summary.AvgActual == nil || summary.AvgActual.Kcal != 1750 {
		t.Fatalf("summary = %#v, want both averages", summary)
	}
}

func TestNutritionQualityAnalyticsHTTPContract(t *testing.T) {
	ctrl := gomock.NewController(t)
	reality := mocks.NewMockRealityUseCases(ctrl)
	from := time.Date(2026, time.August, 10, 0, 0, 0, 0, time.UTC)
	reality.EXPECT().
		QualityAnalytics(gomock.Any(), gomock.Any(), nutrition.SummaryBasisActual).
		Return(usecasereality.QualityAnalytics{
			Period:   usecasereality.Period{EaterID: 1, From: from, To: from.AddDate(0, 0, 6)},
			Basis:    nutrition.SummaryBasisActual,
			Days:     7,
			Coverage: 0.4,
			Averages: nutrition.DailyTotals{FiberG: 18, GrainGEquiv: 200, WholeGrainGEquiv: 50},
			Rolling:  nutrition.RollingSummary{WindowDays: 7, RedMeatG: 420},
			Evaluations: []usecasereality.RuleEvaluation{{
				Scope: usecasereality.RuleScopeDay,
				Date:  from,
				Result: nutrition.RuleEvaluationResult{
					RuleCode: "fiber_min_g", Status: nutrition.StatusUnknown, Coverage: 0.4,
				},
			}},
		}, nil)

	handler := httpapi.New(httpapi.Deps{APIToken: "secret", Reality: reality})
	response := doRequest(t, handler, http.MethodGet, "/analytics/nutrition-quality?eaterId=1&from=2026-08-10&to=2026-08-16", "")

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s; want 200", response.Code, response.Body.String())
	}
	var payload struct {
		DataCoverageScore float32 `json:"data_coverage_score"`
		FiberG            float32 `json:"fiber_g"`
		WholeGrainRatio   float32 `json:"whole_grain_ratio"`
		RedMeatGRolling7d float32 `json:"red_meat_g_rolling_7d"`
		RuleEvaluations   []struct {
			RuleCode string `json:"rule_code"`
			Status   string `json:"status"`
		} `json:"rule_evaluations"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode analytics: %v", err)
	}
	if payload.DataCoverageScore != 0.4 {
		t.Fatalf("coverage = %v, want the low confidence reported rather than hidden", payload.DataCoverageScore)
	}
	if payload.WholeGrainRatio != 0.25 || payload.RedMeatGRolling7d != 420 {
		t.Fatalf("payload = %#v, want the derived ratio and weekly exposure", payload)
	}
	if len(payload.RuleEvaluations) != 1 || payload.RuleEvaluations[0].Status != "unknown" {
		t.Fatalf("rule evaluations = %#v, want the unknown verdict surfaced", payload.RuleEvaluations)
	}
}

func TestTopRecipesHTTPContract(t *testing.T) {
	ctrl := gomock.NewController(t)
	reality := mocks.NewMockRealityUseCases(ctrl)
	reality.EXPECT().
		TopRecipes(gomock.Any(), 3).
		Return([]usecasereality.TopRecipe{
			{RecipeID: 5, Name: "Roast chicken", CookedCount: 7},
			{RecipeID: 6, Name: "Lentil stew", CookedCount: 2},
		}, nil)

	handler := httpapi.New(httpapi.Deps{APIToken: "secret", Reality: reality})
	response := doRequest(t, handler, http.MethodGet, "/analytics/top-recipes?limit=3", "")

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s; want 200", response.Code, response.Body.String())
	}
	var recipes []model.TopRecipe
	if err := json.Unmarshal(response.Body.Bytes(), &recipes); err != nil {
		t.Fatalf("decode recipes: %v", err)
	}
	if len(recipes) != 2 || recipes[0].CookedCount < recipes[1].CookedCount {
		t.Fatalf("recipes = %#v, want the most cooked first", recipes)
	}
}
