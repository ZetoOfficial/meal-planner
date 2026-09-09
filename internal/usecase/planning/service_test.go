package planning

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.uber.org/mock/gomock"

	catalogdom "github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
	eaterdom "github.com/ZetoOfficial/meal-planner/internal/domain/eater"
	mealdom "github.com/ZetoOfficial/meal-planner/internal/domain/meal"
	"github.com/ZetoOfficial/meal-planner/internal/domain/nutrition"
	planningdom "github.com/ZetoOfficial/meal-planner/internal/domain/planning"
	apperr "github.com/ZetoOfficial/meal-planner/internal/errors"
	"github.com/ZetoOfficial/meal-planner/internal/quality"
	"github.com/ZetoOfficial/meal-planner/internal/solver"
	"github.com/ZetoOfficial/meal-planner/internal/usecase/planning/mocks"
)

func planStartDate() time.Time { return time.Date(2026, time.August, 10, 0, 0, 0, 0, time.UTC) }

func TestGeneratePlanPersistsPlanAndQualityWithoutLLM(t *testing.T) {
	deps := newDeps(t)
	deps.expectInputs()
	deps.expectSolvedPlan()

	var savedPlan planningdom.Plan
	deps.plans.EXPECT().
		Save(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, plan planningdom.Plan) (planningdom.Plan, error) {
			savedPlan = plan
			plan.ID = 7
			return plan, nil
		})
	var savedSnapshots []planningdom.QualitySnapshot
	deps.plans.EXPECT().
		SaveQualitySnapshots(gomock.Any(), planningdom.PlanID(7), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ planningdom.PlanID, snapshots []planningdom.QualitySnapshot) error {
			savedSnapshots = snapshots
			return nil
		})

	result, err := deps.service.GeneratePlan(t.Context(), validCommand())
	if err != nil {
		t.Fatalf("GeneratePlan() error = %v", err)
	}

	if result.Plan.ID != 7 || result.Plan.Status != planningdom.PlanStatusDraft {
		t.Fatalf("generated plan = %#v, want a persisted draft", result.Plan)
	}
	if savedPlan.Novelty != 2 {
		t.Fatalf("stored novelty = %d, want the requested level passed through", savedPlan.Novelty)
	}
	if savedPlan.QualityScore == nil || *savedPlan.QualityScore != 80 {
		t.Fatalf("stored quality score = %v, want the mean evaluation score", savedPlan.QualityScore)
	}
	if savedPlan.QualityWarningsCount != 1 {
		t.Fatalf("stored warnings = %d, want the soft failure counted", savedPlan.QualityWarningsCount)
	}
	if len(savedSnapshots) != 1 || len(savedSnapshots[0].DayRules) != 1 {
		t.Fatalf("stored snapshots = %#v, want one eater-day with its day rules", savedSnapshots)
	}
	if savedSnapshots[0].DayRules[0].RuleCode != quality.RuleFiber {
		t.Fatalf("stored day rule = %#v, want the fiber verdict from the ranking", savedSnapshots[0].DayRules[0])
	}
	if len(savedSnapshots[0].RollingRules) != 1 || savedSnapshots[0].RollingRules[0].RuleCode != quality.RuleRedMeat {
		t.Fatalf("stored rolling rules = %#v, want the weekly verdict", savedSnapshots[0].RollingRules)
	}
}

func TestGeneratePlanReportsInfeasibilityWithDiagnostics(t *testing.T) {
	deps := newDeps(t)
	deps.expectInputs()
	deps.planner.EXPECT().
		BuildPlan(gomock.Any(), gomock.Any()).
		Return(solver.Result{Failure: &solver.Failure{
			Code:       solver.FailureMacroInfeasible,
			EaterID:    1,
			Date:       planStartDate(),
			Exhaustive: true,
		}}, nil)

	_, err := deps.service.GeneratePlan(t.Context(), validCommand())

	var infeasible *InfeasibleError
	if !errors.As(err, &infeasible) {
		t.Fatalf("GeneratePlan() error = %v, want an infeasible plan error", err)
	}
	if !infeasible.Failure.Exhaustive || len(infeasible.Failure.Diagnostics()) == 0 {
		t.Fatalf("failure = %#v, want exhaustive diagnostics", infeasible.Failure)
	}
}

func TestGeneratePlanRejectsInvalidCommandBeforeTouchingTheDatabase(t *testing.T) {
	cases := map[string]GeneratePlanCommand{
		"missing start date": {Targets: validTargets()},
		"negative days":      {StartDate: planStartDate(), Days: -1, Targets: validTargets()},
		"novelty above four": {StartDate: planStartDate(), Novelty: 4, Targets: validTargets()},
		"no targets":         {StartDate: planStartDate()},
		"repeated eater": {StartDate: planStartDate(), Targets: []EaterTarget{
			{EaterID: 1, GeneratedMealTypes: []mealdom.Type{mealdom.TypeLunch}},
			{EaterID: 1, GeneratedMealTypes: []mealdom.Type{mealdom.TypeDinner}},
		}},
		"repeated meal type": {StartDate: planStartDate(), Targets: []EaterTarget{
			{EaterID: 1, GeneratedMealTypes: []mealdom.Type{mealdom.TypeLunch, mealdom.TypeLunch}},
		}},
		"unknown meal type": {StartDate: planStartDate(), Targets: []EaterTarget{
			{EaterID: 1, GeneratedMealTypes: []mealdom.Type{"brunch"}},
		}},
	}

	for name, command := range cases {
		t.Run(name, func(t *testing.T) {
			deps := newDeps(t)

			_, err := deps.service.GeneratePlan(t.Context(), command)

			if !isValidation(err) {
				t.Fatalf("GeneratePlan() error = %v, want a validation error", err)
			}
		})
	}
}

func TestGeneratePlanDefaultsDaysAndCookDate(t *testing.T) {
	deps := newDeps(t)
	deps.expectInputs()

	var input solver.Input
	deps.planner.EXPECT().
		BuildPlan(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, in solver.Input) (solver.Result, error) {
			input = in
			return solver.Result{}, errors.New("stop after input")
		})

	command := validCommand()
	command.Days = 0
	command.CookDate = nil
	_, _ = deps.service.GeneratePlan(t.Context(), command)

	if input.Days != 3 {
		t.Fatalf("solver days = %d, want the documented default of 3", input.Days)
	}
	if !input.CookDate.Equal(planStartDate()) {
		t.Fatalf("cook date = %s, want it defaulted to the start date", input.CookDate)
	}
	if len(input.Eaters) != 1 || len(input.Eaters[0].Days) != 3 {
		t.Fatalf("solver eaters = %#v, want one eater with three dated targets", input.Eaters)
	}
}

func TestGeneratePlanFailsWhenNoGoalIsEffective(t *testing.T) {
	deps := newDeps(t)
	deps.products.EXPECT().List(gomock.Any(), "").Return(nil, nil)
	deps.recipes.EXPECT().ListPermanentCandidates(gomock.Any()).Return(nil, nil)
	deps.eaters.EXPECT().Get(gomock.Any(), eaterdom.ID(1)).Return(eaterdom.Eater{ID: 1}, nil)
	deps.eaters.EXPECT().ListGoals(gomock.Any(), eaterdom.ID(1)).Return([]eaterdom.MacroGoal{{
		ID:            1,
		EaterID:       1,
		EffectiveFrom: planStartDate().AddDate(0, 0, 30),
		Range:         goalRange(),
	}}, nil)
	deps.eaters.EXPECT().ListFixedMeals(gomock.Any(), eaterdom.ID(1)).Return(nil, nil)

	_, err := deps.service.GeneratePlan(t.Context(), validCommand())

	if !isValidation(err) {
		t.Fatalf("GeneratePlan() error = %v, want a validation error about the missing goal", err)
	}
}

func TestGeneratePlanPropagatesSnapshotFailureSoTheTransactionRollsBack(t *testing.T) {
	deps := newDeps(t)
	deps.expectInputs()
	deps.expectSolvedPlan()
	deps.plans.EXPECT().
		Save(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, plan planningdom.Plan) (planningdom.Plan, error) {
			plan.ID = 7
			return plan, nil
		})
	storageErr := errors.New("summary write failed")
	deps.plans.EXPECT().SaveQualitySnapshots(gomock.Any(), gomock.Any(), gomock.Any()).Return(storageErr)

	_, err := deps.service.GeneratePlan(t.Context(), validCommand())

	if !errors.Is(err, storageErr) {
		t.Fatalf("GeneratePlan() error = %v, want the snapshot failure to abort the transaction", err)
	}
	if deps.committed {
		t.Fatal("transaction committed, want the failed snapshot write to roll it back")
	}
}

func TestGeneratePlanPassesOnlyActiveFixedMealsToTheSolver(t *testing.T) {
	deps := newDeps(t)
	deps.products.EXPECT().List(gomock.Any(), "").Return(nil, nil)
	deps.recipes.EXPECT().ListPermanentCandidates(gomock.Any()).Return(nil, nil)
	deps.eaters.EXPECT().Get(gomock.Any(), eaterdom.ID(1)).Return(eaterdom.Eater{ID: 1}, nil)
	deps.eaters.EXPECT().ListGoals(gomock.Any(), eaterdom.ID(1)).Return(validGoals(), nil)
	deps.eaters.EXPECT().ListFixedMeals(gomock.Any(), eaterdom.ID(1)).Return([]eaterdom.FixedMeal{
		{ID: 1, EaterID: 1, MealType: mealdom.TypeBreakfast, Active: true},
		{ID: 2, EaterID: 1, MealType: mealdom.TypeSnack, Active: false},
	}, nil)

	var input solver.Input
	deps.planner.EXPECT().
		BuildPlan(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, in solver.Input) (solver.Result, error) {
			input = in
			return solver.Result{}, errors.New("stop after input")
		})

	_, _ = deps.service.GeneratePlan(t.Context(), validCommand())

	if len(input.Eaters[0].FixedMeals) != 1 || input.Eaters[0].FixedMeals[0].ID != 1 {
		t.Fatalf("fixed meals = %#v, want only the active one", input.Eaters[0].FixedMeals)
	}
}

type deps struct {
	service   *Service
	plans     *mocks.MockPlanRepository
	eaters    *mocks.MockEaterRepository
	products  *mocks.MockProductRepository
	recipes   *mocks.MockRecipeRepository
	planner   *mocks.MockSolver
	committed bool
}

func newDeps(t *testing.T) *deps {
	t.Helper()
	controller := gomock.NewController(t)
	result := &deps{
		plans:    mocks.NewMockPlanRepository(controller),
		eaters:   mocks.NewMockEaterRepository(controller),
		products: mocks.NewMockProductRepository(controller),
		recipes:  mocks.NewMockRecipeRepository(controller),
		planner:  mocks.NewMockSolver(controller),
	}
	transactions := mocks.NewMockTxRunner(controller)
	transactions.EXPECT().
		WithTx(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			err := fn(ctx)
			result.committed = err == nil
			return err
		}).
		AnyTimes()

	result.service = NewService(
		result.plans, result.eaters, result.products, result.recipes, result.planner, transactions,
		Options{AvailableEquipment: []string{"oven"}, RollingWindowDays: 7},
	)
	return result
}

func (d *deps) expectInputs() {
	d.products.EXPECT().List(gomock.Any(), "").Return([]catalogdom.Product{{ID: 1}}, nil)
	d.recipes.EXPECT().ListPermanentCandidates(gomock.Any()).Return(nil, nil)
	d.eaters.EXPECT().Get(gomock.Any(), eaterdom.ID(1)).Return(eaterdom.Eater{ID: 1}, nil)
	d.eaters.EXPECT().ListGoals(gomock.Any(), eaterdom.ID(1)).Return(validGoals(), nil)
	d.eaters.EXPECT().ListFixedMeals(gomock.Any(), eaterdom.ID(1)).Return(nil, nil)
}

func (d *deps) expectSolvedPlan() {
	d.planner.EXPECT().
		BuildPlan(gomock.Any(), gomock.Any()).
		Return(solver.Result{
			Plan: planningdom.Plan{StartDate: planStartDate(), Days: 1, CookDate: planStartDate(), Novelty: 2},
			Quality: []solver.EaterDayQuality{{
				EaterID: 1,
				Date:    planStartDate(),
				Day:     nutrition.DaySummary{Basis: nutrition.SummaryBasisPlanned},
				Rolling: nutrition.RollingSummary{Basis: nutrition.SummaryBasisPlanned, WindowDays: 7},
				Evaluation: quality.Evaluation{
					Score:    80,
					Coverage: 0.9,
					Outcomes: []quality.RuleOutcome{
						{Code: quality.RuleFiber, Status: nutrition.StatusSoftFail, Message: "fiber below target"},
						{Code: quality.RuleRedMeat, Status: nutrition.StatusPass, Message: "red meat within the weekly cap"},
					},
				},
			}},
		}, nil)
}

func isValidation(err error) bool {
	var appError *apperr.Error
	return errors.As(err, &appError) && appError.Code == apperr.CodeValidation
}

func validCommand() GeneratePlanCommand {
	cookDate := planStartDate()
	return GeneratePlanCommand{
		StartDate: planStartDate(),
		Days:      1,
		CookDate:  &cookDate,
		Novelty:   2,
		Targets:   validTargets(),
	}
}

func validTargets() []EaterTarget {
	return []EaterTarget{{EaterID: 1, GeneratedMealTypes: []mealdom.Type{mealdom.TypeLunch}}}
}

func validGoals() []eaterdom.MacroGoal {
	return []eaterdom.MacroGoal{{ID: 1, EaterID: 1, EffectiveFrom: planStartDate().AddDate(0, 0, -1), Range: goalRange()}}
}

func goalRange() eaterdom.MacroRange {
	return eaterdom.MacroRange{
		ProteinMinG: 30, ProteinMaxG: 200,
		FatMinG: 10, FatMaxG: 90,
		CarbMinG: 40, CarbMaxG: 300,
		KcalMin: 400, KcalMax: 2500,
	}
}
