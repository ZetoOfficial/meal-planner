package reality

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"go.uber.org/mock/gomock"

	eaterdom "github.com/ZetoOfficial/meal-planner/internal/domain/eater"
	feedbackdom "github.com/ZetoOfficial/meal-planner/internal/domain/feedback"
	"github.com/ZetoOfficial/meal-planner/internal/domain/nutrition"
	planningdom "github.com/ZetoOfficial/meal-planner/internal/domain/planning"
	recipedom "github.com/ZetoOfficial/meal-planner/internal/domain/recipe"
	apperr "github.com/ZetoOfficial/meal-planner/internal/errors"
	"github.com/ZetoOfficial/meal-planner/internal/usecase/reality/mocks"
)

func TestLogMealStoresEachStatus(t *testing.T) {
	actual := nutrition.Macros{ProteinG: 25, FatG: 12, CarbG: 35, Kcal: 380}
	cases := map[string]planningdom.MealLog{
		"eaten as planned": {Status: planningdom.MealLogStatusEatenAsPlanned, LoggedBy: 1},
		"eaten modified":   {Status: planningdom.MealLogStatusEatenModified, LoggedBy: 1, Actual: &actual},
		"skipped":          {Status: planningdom.MealLogStatusSkipped, LoggedBy: 1},
	}

	for name, log := range cases {
		t.Run(name, func(t *testing.T) {
			deps := newRealityDeps(t)
			var stored planningdom.MealLog
			deps.meals.EXPECT().
				LogMeal(gomock.Any(), planningdom.MealID(21), gomock.Any()).
				DoAndReturn(func(_ context.Context, _ planningdom.MealID, l planningdom.MealLog) (planningdom.Meal, error) {
					stored = l
					return planningdom.Meal{ID: 21, PlanID: 7, LogStatus: l.Status, Actual: l.Actual}, nil
				})
			deps.meals.EXPECT().
				Get(gomock.Any(), planningdom.PlanID(7)).
				Return(planningdom.Plan{ID: 7, StartDate: analyticsDay(0), Days: 1, CookDate: analyticsDay(0)}, nil)
			deps.products.EXPECT().List(gomock.Any(), "").Return(nil, nil)
			deps.recipes.EXPECT().List(gomock.Any(), nil).Return(nil, nil)
			deps.meals.EXPECT().
				SaveQualitySnapshots(gomock.Any(), planningdom.PlanID(7), gomock.Any()).
				Return(nil)

			meal, err := deps.service.LogMeal(t.Context(), 21, log)
			if err != nil {
				t.Fatalf("LogMeal() error = %v", err)
			}
			if meal.LogStatus != log.Status {
				t.Fatalf("stored status = %q, want %q", meal.LogStatus, log.Status)
			}
			if stored.At.IsZero() {
				t.Fatal("log timestamp is zero, want the service to stamp it")
			}
		})
	}
}

// Recomputing the actual-basis snapshot belongs to the event that changes the
// data, not to the read that reports it: a GET must not write.
func TestLogMealRefreshesTheActualSnapshotAndAnalyticsDoesNot(t *testing.T) {
	deps := newRealityDeps(t)
	plan := planningdom.Plan{
		ID: 7, StartDate: analyticsDay(0), Days: 1, CookDate: analyticsDay(0),
		EaterDays: []planningdom.EaterDay{{EaterID: 1, Date: analyticsDay(0), MacroGoalID: 1}},
	}
	deps.meals.EXPECT().
		LogMeal(gomock.Any(), planningdom.MealID(21), gomock.Any()).
		Return(planningdom.Meal{ID: 21, PlanID: 7, EaterID: 1, Date: analyticsDay(0)}, nil)
	deps.meals.EXPECT().Get(gomock.Any(), planningdom.PlanID(7)).Return(plan, nil)
	deps.products.EXPECT().List(gomock.Any(), "").Return(nil, nil)
	deps.recipes.EXPECT().List(gomock.Any(), nil).Return(nil, nil)
	deps.eaters.EXPECT().Get(gomock.Any(), eaterdom.ID(1)).Return(eaterdom.Eater{ID: 1}, nil)
	deps.eaters.EXPECT().ListFixedMeals(gomock.Any(), eaterdom.ID(1)).Return(nil, nil)
	deps.meals.EXPECT().SaveQualitySnapshots(gomock.Any(), planningdom.PlanID(7), gomock.Any()).Return(nil)

	if _, err := deps.service.LogMeal(t.Context(), 21, planningdom.MealLog{
		Status: planningdom.MealLogStatusEatenAsPlanned, LoggedBy: 1,
	}); err != nil {
		t.Fatalf("LogMeal() error = %v", err)
	}

	// No repository write is expected here: gomock fails the test if one happens.
	if _, err := deps.service.QualityAnalytics(t.Context(), analyticsPeriod(), nutrition.SummaryBasisActual); err != nil {
		t.Fatalf("QualityAnalytics() error = %v", err)
	}
}

func TestLogMealRejectsActualMacrosOutsideAModifiedMeal(t *testing.T) {
	deps := newRealityDeps(t)
	actual := nutrition.Macros{ProteinG: 25}

	_, err := deps.service.LogMeal(t.Context(), 21, planningdom.MealLog{
		Status: planningdom.MealLogStatusEatenAsPlanned, LoggedBy: 1, Actual: &actual,
	})

	if !isValidation(err) {
		t.Fatalf("LogMeal() error = %v, want a validation error", err)
	}
}

func TestLogMealRejectsAModifiedMealWithoutActualMacros(t *testing.T) {
	deps := newRealityDeps(t)

	_, err := deps.service.LogMeal(t.Context(), 21, planningdom.MealLog{
		Status: planningdom.MealLogStatusEatenModified, LoggedBy: 1,
	})

	if !isValidation(err) {
		t.Fatalf("LogMeal() error = %v, want a validation error", err)
	}
}

func TestCreateFeedbackNudgesRecipeWeightWithoutBanning(t *testing.T) {
	deps := newRealityDeps(t)
	rating := -2
	var stored feedbackdom.Event
	deps.feedback.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, event feedbackdom.Event) (feedbackdom.Event, error) {
			stored = event
			event.ID = 3
			return event, nil
		})
	deps.feedback.EXPECT().AdjustRecipeWeight(gomock.Any(), recipedom.ID(5), -0.2).Return(nil)

	event, err := deps.service.CreateFeedback(t.Context(), feedbackdom.Event{
		EaterID: 1, TargetType: feedbackdom.TargetRecipeTemplate, TargetID: 5, Rating: &rating,
	})
	if err != nil {
		t.Fatalf("CreateFeedback() error = %v", err)
	}
	if event.ID != 3 {
		t.Fatalf("event = %#v, want the stored feedback", event)
	}
	if stored.WeightDelta == nil || *stored.WeightDelta != -0.2 {
		t.Fatalf("weight delta = %v, want the rating converted to a bounded nudge", stored.WeightDelta)
	}
}

func TestCreateFeedbackOnAProductRecordsWithoutMovingRecipeWeight(t *testing.T) {
	deps := newRealityDeps(t)
	rating := 1
	deps.feedback.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, event feedbackdom.Event) (feedbackdom.Event, error) {
			event.ID = 4
			return event, nil
		})

	if _, err := deps.service.CreateFeedback(t.Context(), feedbackdom.Event{
		EaterID: 1, TargetType: feedbackdom.TargetProduct, TargetID: 9, Rating: &rating,
	}); err != nil {
		t.Fatalf("CreateFeedback() error = %v", err)
	}
}

func TestCreateFeedbackRejectsInvalidTargets(t *testing.T) {
	deps := newRealityDeps(t)

	_, err := deps.service.CreateFeedback(t.Context(), feedbackdom.Event{
		EaterID: 1, TargetType: "sandwich", TargetID: 9,
	})

	if !isValidation(err) {
		t.Fatalf("CreateFeedback() error = %v, want a validation error", err)
	}
}

func TestMacroAnalyticsAveragesPlannedAndActual(t *testing.T) {
	deps := newRealityDeps(t)
	actual := nutrition.Macros{ProteinG: 20, FatG: 10, CarbG: 30, Kcal: 300}
	deps.meals.EXPECT().
		ListEaterMeals(gomock.Any(), eaterdom.ID(1), gomock.Any(), gomock.Any()).
		Return([]planningdom.Meal{
			{
				Date: analyticsDay(0), LogStatus: planningdom.MealLogStatusEatenAsPlanned,
				Planned: nutrition.Macros{ProteinG: 30, FatG: 10, CarbG: 40, Kcal: 400},
			},
			{
				Date: analyticsDay(1), LogStatus: planningdom.MealLogStatusEatenModified,
				Planned: nutrition.Macros{ProteinG: 30, FatG: 10, CarbG: 40, Kcal: 400},
				Actual:  &actual,
			},
		}, nil)

	result, err := deps.service.MacroAnalytics(t.Context(), analyticsPeriod())
	if err != nil {
		t.Fatalf("MacroAnalytics() error = %v", err)
	}

	if result.Days != 2 {
		t.Fatalf("days = %d, want both dates counted", result.Days)
	}
	if result.AvgPlanned.Kcal != 400 {
		t.Fatalf("planned average = %#v, want the solver numbers", result.AvgPlanned)
	}
	// Day one has no actual, so FR-6 says it counts as planned.
	if result.AvgActual.Kcal != 350 {
		t.Fatalf("actual average = %#v, want planned to stand in where nothing was logged", result.AvgActual)
	}
}

func TestMacroAnalyticsSkipsSkippedMeals(t *testing.T) {
	deps := newRealityDeps(t)
	deps.meals.EXPECT().
		ListEaterMeals(gomock.Any(), eaterdom.ID(1), gomock.Any(), gomock.Any()).
		Return([]planningdom.Meal{{
			Date: analyticsDay(0), LogStatus: planningdom.MealLogStatusSkipped,
			Planned: nutrition.Macros{ProteinG: 30, FatG: 10, CarbG: 40, Kcal: 400},
		}}, nil)

	result, err := deps.service.MacroAnalytics(t.Context(), analyticsPeriod())
	if err != nil {
		t.Fatalf("MacroAnalytics() error = %v", err)
	}
	if result.AvgActual.Kcal != 0 || result.HasActual {
		t.Fatalf("actual average = %#v, want a skipped meal to contribute nothing", result.AvgActual)
	}
}

// Silently dropping a day that does not fit the accumulator would report an
// average over fewer days than it claims.
func TestMacroAnalyticsReportsAnArithmeticOverflowInsteadOfSkippingADay(t *testing.T) {
	deps := newRealityDeps(t)
	deps.meals.EXPECT().
		ListEaterMeals(gomock.Any(), eaterdom.ID(1), gomock.Any(), gomock.Any()).
		Return([]planningdom.Meal{
			{ID: 1, EaterID: 1, Date: analyticsDay(0), Planned: nutrition.Macros{Kcal: math.MaxInt}},
			{ID: 2, EaterID: 1, Date: analyticsDay(1), Planned: nutrition.Macros{Kcal: math.MaxInt}},
		}, nil)

	if _, err := deps.service.MacroAnalytics(t.Context(), analyticsPeriod()); err == nil {
		t.Fatal("MacroAnalytics() = nil, want the overflow reported")
	}
}

func TestMacroAnalyticsHandlesAnEmptyPeriod(t *testing.T) {
	deps := newRealityDeps(t)
	deps.meals.EXPECT().
		ListEaterMeals(gomock.Any(), eaterdom.ID(1), gomock.Any(), gomock.Any()).
		Return(nil, nil)

	result, err := deps.service.MacroAnalytics(t.Context(), analyticsPeriod())
	if err != nil {
		t.Fatalf("MacroAnalytics() error = %v", err)
	}
	if result.Days != 0 {
		t.Fatalf("days = %d, want an empty period to report nothing", result.Days)
	}
}

func TestMacroAnalyticsValidatesTheQuery(t *testing.T) {
	cases := map[string]Period{
		"no eater":      {From: analyticsDay(0), To: analyticsDay(1)},
		"no dates":      {EaterID: 1},
		"reversed span": {EaterID: 1, From: analyticsDay(1), To: analyticsDay(0)},
	}
	for name, period := range cases {
		t.Run(name, func(t *testing.T) {
			deps := newRealityDeps(t)
			if _, err := deps.service.MacroAnalytics(t.Context(), period); !isValidation(err) {
				t.Fatalf("MacroAnalytics() error = %v, want a validation error for %s", err, name)
			}
		})
	}
}

func TestQualityAnalyticsReportsAveragesAndCoverage(t *testing.T) {
	deps := newRealityDeps(t)
	lowCoverage, err := nutrition.CoverageForInput(false, 1)
	if err != nil {
		t.Fatalf("CoverageForInput() error = %v", err)
	}
	fullCoverage, err := nutrition.CoverageForInput(true, 1)
	if err != nil {
		t.Fatalf("CoverageForInput() error = %v", err)
	}
	deps.analytics.days = []nutrition.DaySummary{
		{Totals: nutrition.DailyTotals{FiberG: 30}, Coverage: fullCoverage},
		{Totals: nutrition.DailyTotals{FiberG: 10}, Coverage: lowCoverage},
	}
	deps.analytics.rolling = []nutrition.RollingSummary{{WindowDays: 7, RedMeatG: 120}}

	result, err := deps.service.QualityAnalytics(t.Context(), analyticsPeriod(), nutrition.SummaryBasisPlanned)
	if err != nil {
		t.Fatalf("QualityAnalytics() error = %v", err)
	}

	if result.Averages.FiberG != 20 {
		t.Fatalf("fibre average = %v, want the mean of the stored days", result.Averages.FiberG)
	}
	if result.Coverage != 0.5 {
		t.Fatalf("coverage = %v, want the low-confidence day to pull the average down", result.Coverage)
	}
	if result.Rolling.RedMeatG != 120 {
		t.Fatalf("rolling = %#v, want the weekly exposure reported", result.Rolling)
	}
}

func TestQualityAnalyticsReportsStoredRuleVerdicts(t *testing.T) {
	deps := newRealityDeps(t)
	fullCoverage, err := nutrition.CoverageForInput(true, 1)
	if err != nil {
		t.Fatalf("CoverageForInput() error = %v", err)
	}
	deps.analytics.days = []nutrition.DaySummary{{Totals: nutrition.DailyTotals{FiberG: 30}, Coverage: fullCoverage}}
	deps.analytics.evaluations = []RuleEvaluation{
		{
			Scope:  RuleScopeDay,
			Date:   analyticsDay(0),
			Result: nutrition.RuleEvaluationResult{RuleCode: "fiber", Status: nutrition.StatusPass},
		},
		{
			Scope:       RuleScopeRollingWindow,
			WindowStart: analyticsDay(0),
			WindowEnd:   analyticsDay(6),
			Result:      nutrition.RuleEvaluationResult{RuleCode: "red_meat", Status: nutrition.StatusWarn},
		},
	}

	result, err := deps.service.QualityAnalytics(t.Context(), analyticsPeriod(), nutrition.SummaryBasisPlanned)
	if err != nil {
		t.Fatalf("QualityAnalytics() error = %v", err)
	}
	if len(result.Evaluations) != 2 {
		t.Fatalf("evaluations = %#v, want the stored day and rolling verdicts", result.Evaluations)
	}
	if result.Evaluations[1].Scope != RuleScopeRollingWindow {
		t.Fatalf("second verdict = %#v, want the rolling window scope preserved", result.Evaluations[1])
	}
}

func TestQualityAnalyticsRejectsAnUnknownBasis(t *testing.T) {
	deps := newRealityDeps(t)

	if _, err := deps.service.QualityAnalytics(t.Context(), analyticsPeriod(), "guessed"); !isValidation(err) {
		t.Fatalf("QualityAnalytics() error = %v, want a validation error", err)
	}
}

func TestTopRecipesDefaultsTheLimit(t *testing.T) {
	deps := newRealityDeps(t)
	deps.analytics.top = []TopRecipe{{RecipeID: 5, Name: "Roast", CookedCount: 3}}

	recipes, err := deps.service.TopRecipes(t.Context(), 0)
	if err != nil {
		t.Fatalf("TopRecipes() error = %v", err)
	}
	if len(recipes) != 1 || recipes[0].CookedCount != 3 {
		t.Fatalf("recipes = %#v, want the ranked list", recipes)
	}
	if deps.analytics.lastLimit != 10 {
		t.Fatalf("limit = %d, want the documented default", deps.analytics.lastLimit)
	}
}

type realityDeps struct {
	service   *Service
	meals     *mocks.MockMealRepository
	feedback  *mocks.MockFeedbackRepository
	analytics *fakeAnalytics
	products  *mocks.MockProductRepository
	recipes   *mocks.MockRecipeRepository
	eaters    *mocks.MockEaterRepository
}

func newRealityDeps(t *testing.T) *realityDeps {
	t.Helper()
	controller := gomock.NewController(t)
	deps := &realityDeps{
		meals:     mocks.NewMockMealRepository(controller),
		feedback:  mocks.NewMockFeedbackRepository(controller),
		analytics: &fakeAnalytics{},
		products:  mocks.NewMockProductRepository(controller),
		recipes:   mocks.NewMockRecipeRepository(controller),
		eaters:    mocks.NewMockEaterRepository(controller),
	}
	transactions := mocks.NewMockTxRunner(controller)
	transactions.EXPECT().
		WithTx(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }).
		AnyTimes()
	clock := mocks.NewMockClock(controller)
	clock.EXPECT().Now().Return(analyticsDay(0)).AnyTimes()

	deps.service = NewService(
		deps.meals, deps.feedback, deps.analytics,
		deps.products, deps.recipes, deps.eaters, transactions, clock,
	)
	return deps
}

func analyticsDay(offset int) time.Time {
	return time.Date(2026, time.August, 10+offset, 0, 0, 0, 0, time.UTC)
}

func analyticsPeriod() Period {
	return Period{EaterID: 1, From: analyticsDay(0), To: analyticsDay(6)}
}

func isValidation(err error) bool {
	var appError *apperr.Error
	return errors.As(err, &appError) && appError.Code == apperr.CodeValidation
}

// fakeAnalytics stands in for the analytics reader: its ListTopRecipes returns
// a package-local type, which a generated mock cannot express without importing
// the package under test.
type fakeAnalytics struct {
	days        []nutrition.DaySummary
	rolling     []nutrition.RollingSummary
	evaluations []RuleEvaluation
	top         []TopRecipe
	lastLimit   int
	basis       nutrition.SummaryBasis
}

func (a *fakeAnalytics) ListDaySummaries(
	_ context.Context, _ eaterdom.ID, basis nutrition.SummaryBasis, _, _ time.Time,
) ([]nutrition.DaySummary, error) {
	a.basis = basis
	return a.days, nil
}

func (a *fakeAnalytics) ListRollingSummaries(
	_ context.Context, _ eaterdom.ID, _ nutrition.SummaryBasis, _, _ time.Time,
) ([]nutrition.RollingSummary, error) {
	return a.rolling, nil
}

func (a *fakeAnalytics) ListRuleEvaluations(
	_ context.Context, _ eaterdom.ID, _ nutrition.SummaryBasis, _, _ time.Time,
) ([]RuleEvaluation, error) {
	return a.evaluations, nil
}

func (a *fakeAnalytics) ListTopRecipes(_ context.Context, limit int) ([]TopRecipe, error) {
	a.lastLimit = limit
	return a.top, nil
}
