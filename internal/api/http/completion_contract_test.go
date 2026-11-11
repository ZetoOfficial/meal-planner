package httpapi_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"go.uber.org/mock/gomock"

	"github.com/ZetoOfficial/meal-planner/api/sdk/go/model"
	httpapi "github.com/ZetoOfficial/meal-planner/internal/api/http"
	"github.com/ZetoOfficial/meal-planner/internal/api/http/mocks"
	"github.com/ZetoOfficial/meal-planner/internal/apperr"
	eaterdom "github.com/ZetoOfficial/meal-planner/internal/domain/eater"
	"github.com/ZetoOfficial/meal-planner/internal/domain/nutrition"
	planningdom "github.com/ZetoOfficial/meal-planner/internal/domain/planning"
	usecaseplanning "github.com/ZetoOfficial/meal-planner/internal/usecase/planning"
	usecasereality "github.com/ZetoOfficial/meal-planner/internal/usecase/reality"
)

func TestDeleteEaterHTTPContract(t *testing.T) {
	eaters := mocks.NewMockEaterUseCases(gomock.NewController(t))
	eaters.EXPECT().DeleteEater(gomock.Any(), eaterdom.ID(3)).Return(nil)
	eaters.EXPECT().DeleteEater(gomock.Any(), eaterdom.ID(4)).Return(apperr.Conflict("eater is still in use"))
	handler := httpapi.New(httpapi.Deps{APIToken: "secret", Eater: eaters})

	if response := doRequest(t, handler, http.MethodDelete, "/eaters/3", ""); response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, body = %s; want 204", response.Code, response.Body.String())
	}
	if response := doRequest(t, handler, http.MethodDelete, "/eaters/4", ""); response.Code != http.StatusConflict {
		t.Fatalf("status = %d, body = %s; want 409 for an eater with history", response.Code, response.Body.String())
	}
}

func TestFixedMealUpdateAndDeleteHTTPContract(t *testing.T) {
	eaters := mocks.NewMockEaterUseCases(gomock.NewController(t))
	eaters.EXPECT().UpdateFixedMeal(gomock.Any(), eaterdom.FixedMealID(4), gomock.Any()).
		Return(eaterdom.FixedMeal{
			ID: 4, EaterID: 2, MealType: eaterdom.MealTypeBreakfast, Label: "two toasts",
			Macros: nutrition.Macros{ProteinG: 20, FatG: 10, CarbG: 60, Kcal: 420}, Active: true,
		}, nil)
	eaters.EXPECT().DeleteFixedMeal(gomock.Any(), eaterdom.FixedMealID(4)).Return(nil)
	eaters.EXPECT().DeleteFixedMeal(gomock.Any(), eaterdom.FixedMealID(5)).Return(apperr.NotFound("fixed meal"))
	handler := httpapi.New(httpapi.Deps{APIToken: "secret", Eater: eaters})

	body := `{"meal_type":"breakfast","label":"two toasts","macros":{"protein_g":20,"fat_g":10,"carb_g":60,"kcal":420}}`
	response := doRequest(t, handler, http.MethodPut, "/fixed-meals/4", body)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s; want 200", response.Code, response.Body.String())
	}
	var meal model.FixedMeal
	if err := json.Unmarshal(response.Body.Bytes(), &meal); err != nil {
		t.Fatalf("decode fixed meal: %v", err)
	}
	if meal.Id != 4 || meal.Label != "two toasts" || meal.Macros.Kcal != 420 {
		t.Fatalf("fixed meal = %#v, want the updated row", meal)
	}

	if response := doRequest(t, handler, http.MethodDelete, "/fixed-meals/4", ""); response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, body = %s; want 204", response.Code, response.Body.String())
	}
	if response := doRequest(t, handler, http.MethodDelete, "/fixed-meals/5", ""); response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, body = %s; want 404", response.Code, response.Body.String())
	}
}

func TestProductFrequencyHTTPContract(t *testing.T) {
	reality := mocks.NewMockRealityUseCases(gomock.NewController(t))
	from := time.Date(2026, time.August, 10, 0, 0, 0, 0, time.UTC)
	reality.EXPECT().
		ProductFrequency(gomock.Any(), usecasereality.Period{EaterID: 1, From: from, To: from.AddDate(0, 0, 6)}, 5).
		Return([]usecasereality.ProductFrequency{
			{ProductID: 3, Brand: "Acme", Name: "Rice", MealsCount: 4, TotalQtyG: 320},
			{ProductID: 9, Name: "Lentils", MealsCount: 1, TotalQtyG: 60},
		}, nil)
	handler := httpapi.New(httpapi.Deps{APIToken: "secret", Reality: reality})

	response := doRequest(t, handler, http.MethodGet,
		"/analytics/product-frequency?eaterId=1&from=2026-08-10&to=2026-08-16&limit=5", "")

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s; want 200", response.Code, response.Body.String())
	}
	var rows []model.ProductFrequency
	if err := json.Unmarshal(response.Body.Bytes(), &rows); err != nil {
		t.Fatalf("decode product frequency: %v", err)
	}
	if len(rows) != 2 || rows[0].ProductId != 3 || rows[0].MealsCount != 4 || rows[0].TotalQtyG != 320 {
		t.Fatalf("rows = %#v, want the ranked products", rows)
	}
	if rows[0].Brand == nil || *rows[0].Brand != "Acme" || rows[1].Brand != nil {
		t.Fatalf("brands = %v / %v, want a brand only where one is known", rows[0].Brand, rows[1].Brand)
	}
}

func TestQualityAnalyticsReportsTheFixedMealBurden(t *testing.T) {
	reality := mocks.NewMockRealityUseCases(gomock.NewController(t))
	sodium := 450.0
	reality.EXPECT().
		QualityAnalytics(gomock.Any(), gomock.Any(), nutrition.SummaryBasisActual).
		Return(usecasereality.QualityAnalytics{
			Days:       2,
			FixedMeals: usecasereality.FixedMealBurden{SodiumMg: &sodium},
		}, nil)
	handler := httpapi.New(httpapi.Deps{APIToken: "secret", Reality: reality})

	response := doRequest(t, handler, http.MethodGet,
		"/analytics/nutrition-quality?eaterId=1&from=2026-08-10&to=2026-08-16", "")

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s; want 200", response.Code, response.Body.String())
	}
	var analytics model.NutritionQualityAnalytics
	if err := json.Unmarshal(response.Body.Bytes(), &analytics); err != nil {
		t.Fatalf("decode analytics: %v", err)
	}
	burden := analytics.FixedMealQualityBurden
	if burden == nil || burden.SodiumMg == nil || *burden.SodiumMg != 450 {
		t.Fatalf("burden = %#v, want the fixed-meal sodium share", burden)
	}
	if burden.ProcessedMeatG != nil || strings.Contains(response.Body.String(), `"processed_meat_g":0`) {
		t.Fatalf("burden = %#v, want an unstated nutrient left out, not reported as zero", burden)
	}
}

func TestGetPlanPublishesThePrepGraphByPlanRecipeID(t *testing.T) {
	planning := mocks.NewMockPlanningUseCases(gomock.NewController(t))
	start := time.Date(2026, time.August, 10, 0, 0, 0, 0, time.UTC)
	note := "assembled fresh from plan recipe 1"
	planning.EXPECT().GetPlan(gomock.Any(), planningdom.PlanID(7)).Return(usecaseplanning.PlanResult{
		Plan: planningdom.Plan{
			ID: 7, StartDate: start, Days: 1, CookDate: start, Status: planningdom.PlanStatusDraft,
			Recipes: []planningdom.PlanRecipe{{ID: 31, TemplateID: 1, ScaleFactor: 1}, {ID: 32, TemplateID: 2, ScaleFactor: 1}},
			PrepDependencies: []planningdom.PrepDependency{
				{DependentRecipeIndex: 1, SourceRecipeIndex: 0, Note: &note},
			},
		},
	}, nil)
	handler := httpapi.New(httpapi.Deps{APIToken: "secret", Planning: planning})

	response := doRequest(t, handler, http.MethodGet, "/plans/7", "")

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s; want 200", response.Code, response.Body.String())
	}
	var plan model.Plan
	if err := json.Unmarshal(response.Body.Bytes(), &plan); err != nil {
		t.Fatalf("decode plan: %v", err)
	}
	if len(plan.PrepDependencies) != 1 {
		t.Fatalf("prep dependencies = %#v, want the stored edge", plan.PrepDependencies)
	}
	edge := plan.PrepDependencies[0]
	if edge.DependentRecipeId != 32 || edge.SourceRecipeId != 31 || edge.Note == nil || *edge.Note != note {
		t.Fatalf("edge = %#v, want plan recipe 32 depending on 31", edge)
	}
}
