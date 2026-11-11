package planning

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/ZetoOfficial/meal-planner/internal/apperr"
	catalogdom "github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
	eaterdom "github.com/ZetoOfficial/meal-planner/internal/domain/eater"
	mealdom "github.com/ZetoOfficial/meal-planner/internal/domain/meal"
	planningdom "github.com/ZetoOfficial/meal-planner/internal/domain/planning"
	recipedom "github.com/ZetoOfficial/meal-planner/internal/domain/recipe"
	"github.com/ZetoOfficial/meal-planner/internal/solver"
	"github.com/ZetoOfficial/meal-planner/internal/swap"
)

// expectRichSwapCatalogue serves a library with two interchangeable lunch
// templates and the products that fill them.
func (d *deps) expectRichSwapCatalogue() {
	protein := []catalogdom.RoleTag{{Code: "protein:lean", Kind: catalogdom.RoleKindProtein}}
	grain := []catalogdom.RoleTag{{Code: "carb:grain", Kind: catalogdom.RoleKindCarb}}
	d.products.EXPECT().List(gomock.Any(), "").Return([]catalogdom.Product{
		{ID: 1, Name: "Chicken", ProteinPer100G: 23, FatPer100G: 2, KcalPer100G: 110, Roles: protein},
		{ID: 2, Name: "Turkey", ProteinPer100G: 25, FatPer100G: 1, KcalPer100G: 110, Roles: protein},
		{ID: 3, Name: "Rice", ProteinPer100G: 8, CarbPer100G: 76, KcalPer100G: 350, Roles: grain},
	}, nil).AnyTimes()
	lunch := []mealdom.Type{mealdom.TypeLunch}
	d.recipes.EXPECT().List(gomock.Any(), nil).Return([]recipedom.Template{
		{
			ID: 1, Name: "Baked chicken", Status: recipedom.StatusPermanent, BaseServings: 1, AllowedMealTypes: lunch, Storage: recipedom.StorageKeeps3d,
			Slots: []recipedom.Slot{{ID: 1, RoleTagCode: "protein:lean", QtyG: 100}},
		},
		{
			ID: 2, Name: "Rice", Status: recipedom.StatusPermanent, BaseServings: 2, AllowedMealTypes: lunch, Storage: recipedom.StorageKeeps3d,
			Slots: []recipedom.Slot{{ID: 5, RoleTagCode: "carb:grain", QtyG: 160}},
		},
	}, nil).AnyTimes()
	d.eaters.EXPECT().Get(gomock.Any(), eaterdom.ID(1)).Return(eaterdom.Eater{ID: 1}, nil).AnyTimes()
	d.eaters.EXPECT().ListFixedMeals(gomock.Any(), eaterdom.ID(1)).Return(nil, nil).AnyTimes()
	d.eaters.EXPECT().ListGoals(gomock.Any(), eaterdom.ID(1)).Return(validGoals(), nil).AnyTimes()
	d.plans.EXPECT().
		ListEaterDaySummaries(gomock.Any(), eaterdom.ID(1), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, nil).
		AnyTimes()
}

func TestApplySwapNeedsConfirmationAndWritesNothingWithoutIt(t *testing.T) {
	deps := newDeps(t)
	deps.plans.EXPECT().Get(gomock.Any(), planningdom.PlanID(7)).Return(swappablePlan(), nil)
	deps.expectRichSwapCatalogue()

	_, err := deps.service.ApplySwap(t.Context(), 7,
		SwapCommand{Level: planningdom.SwapFill, SlotFillID: 11, NewProductID: 2}, false)

	var unconfirmed *UnconfirmedSwapError
	if !errors.As(err, &unconfirmed) {
		t.Fatalf("ApplySwap() error = %v, want the preview returned for confirmation", err)
	}
	if unconfirmed.Preview.Level != swap.LevelProteinChange {
		t.Fatalf("preview level = %q, want protein_change", unconfirmed.Preview.Level)
	}
}

func TestApplyFillSwapStoresTheNewMealMacrosAndHeadline(t *testing.T) {
	deps := newDeps(t)
	deps.plans.EXPECT().Get(gomock.Any(), planningdom.PlanID(7)).Return(swappablePlan(), nil).Times(2)
	deps.expectRichSwapCatalogue()
	deps.plans.EXPECT().SwapSlotFill(gomock.Any(), planningdom.PlanID(7), planningdom.SlotFillID(11), catalogdom.ProductID(2)).Return(nil)
	var stored planningdom.Plan
	deps.plans.EXPECT().UpdateMealMacros(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, plan planningdom.Plan) error {
			stored = plan
			return nil
		})
	deps.plans.EXPECT().SaveQualitySnapshots(gomock.Any(), planningdom.PlanID(7), gomock.Any()).Return(nil)
	var score *float64
	deps.plans.EXPECT().UpdatePlanQuality(gomock.Any(), planningdom.PlanID(7), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ planningdom.PlanID, value *float64, _ int) error {
			score = value
			return nil
		})

	_, err := deps.service.ApplySwap(t.Context(), 7,
		SwapCommand{Level: planningdom.SwapFill, SlotFillID: 11, NewProductID: 2}, true)
	if err != nil {
		t.Fatalf("ApplySwap() error = %v", err)
	}

	if got := stored.Meals[0].Planned.ProteinG; got != 25 {
		t.Fatalf("stored planned protein = %v, want the meal to follow the turkey fill", got)
	}
	if score == nil {
		t.Fatal("plan quality score was not refreshed after the swap")
	}
}

func TestApplyRecipeSwapRewritesTheBatchInPlace(t *testing.T) {
	deps := newDeps(t)
	deps.plans.EXPECT().Get(gomock.Any(), planningdom.PlanID(7)).Return(swappablePlan(), nil).Times(2)
	deps.expectRichSwapCatalogue()
	var replaced planningdom.Plan
	deps.plans.EXPECT().ReplacePlanRecipe(gomock.Any(), planningdom.PlanID(7), gomock.Any(), 0).
		DoAndReturn(func(_ context.Context, _ planningdom.PlanID, plan planningdom.Plan, _ int) error {
			replaced = plan
			return nil
		})
	deps.plans.EXPECT().SaveQualitySnapshots(gomock.Any(), planningdom.PlanID(7), gomock.Any()).Return(nil)
	deps.plans.EXPECT().UpdatePlanQuality(gomock.Any(), planningdom.PlanID(7), gomock.Any(), gomock.Any()).Return(nil)

	_, err := deps.service.ApplySwap(t.Context(), 7,
		SwapCommand{Level: planningdom.SwapRecipe, PlanRecipeID: 3, NewTemplateID: 2}, true)
	if err != nil {
		t.Fatalf("ApplySwap() error = %v", err)
	}

	recipe := replaced.Recipes[0]
	// One serving of chicken becomes one serving of a two-serving rice base.
	if recipe.ID != 3 || recipe.TemplateID != 2 || recipe.ScaleFactor != 0.5 {
		t.Fatalf("recipe = %#v, want the same batch row cooking half a base of rice", recipe)
	}
	if len(recipe.Fills) != 1 || recipe.Fills[0].ProductID != 3 || recipe.Fills[0].PlannedInputQtyG != 80 {
		t.Fatalf("fills = %#v, want 80 g of rice", recipe.Fills)
	}
}

func TestPreviewRecipeSwapReportsAnUnknownTemplateAsAValidationError(t *testing.T) {
	deps := newDeps(t)
	deps.plans.EXPECT().Get(gomock.Any(), planningdom.PlanID(7)).Return(swappablePlan(), nil)
	deps.expectRichSwapCatalogue()

	_, err := deps.service.PreviewSwap(t.Context(), 7,
		SwapCommand{Level: planningdom.SwapRecipe, PlanRecipeID: 3, NewTemplateID: 99})

	if !isValidation(err) {
		t.Fatalf("PreviewSwap() error = %v, want a validation error", err)
	}
}

func TestPreviewSwapRejectsAnUnknownLevel(t *testing.T) {
	deps := newDeps(t)
	deps.plans.EXPECT().Get(gomock.Any(), planningdom.PlanID(7)).Return(swappablePlan(), nil)
	deps.expectRichSwapCatalogue()

	if _, err := deps.service.PreviewSwap(t.Context(), 7, SwapCommand{Level: "week"}); !isValidation(err) {
		t.Fatalf("PreviewSwap() error = %v, want a validation error", err)
	}
}

// rotatedPlan is what the solver returns for the rotation: rice instead of
// chicken for the same eater-day.
func rotatedPlan() solver.Result {
	result := solvedResult()
	result.Plan = planningdom.Plan{
		StartDate: planStartDate(), Days: 1, CookDate: planStartDate(),
		EaterDays: []planningdom.EaterDay{{
			EaterID: 1, Date: planStartDate(), MacroGoalID: 1,
			GeneratedMealTypes: []mealdom.Type{mealdom.TypeLunch},
		}},
		Recipes: []planningdom.PlanRecipe{{
			TemplateID: 2, ScaleFactor: 0.5,
			Fills: []planningdom.SlotFill{{SlotID: 5, ProductID: 3, PlannedInputQtyG: 80}},
		}},
		Meals: []planningdom.Meal{{
			EaterID: 1, Date: planStartDate(), Type: mealdom.TypeLunch,
			Components: []planningdom.ComponentAllocation{{PlanRecipeIndex: 0, PortionFactor: 1}},
		}},
	}
	return result
}

func (d *deps) expectRotationInputs() {
	d.expectRichSwapCatalogue()
	d.recipes.EXPECT().ListPermanentCandidates(gomock.Any()).Return([]recipedom.Template{{ID: 1}, {ID: 2}}, nil).AnyTimes()
	d.plans.EXPECT().ListTemplateLastPlanned(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
}

func TestPreviewRotationSearchesTheRestOfTheLibraryFirst(t *testing.T) {
	deps := newDeps(t)
	deps.plans.EXPECT().Get(gomock.Any(), planningdom.PlanID(7)).Return(swappablePlan(), nil)
	deps.expectRotationInputs()
	var offered []recipedom.Template
	deps.planner.EXPECT().BuildPlan(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, input solver.Input) (solver.Result, error) {
			offered = input.Recipes
			return rotatedPlan(), nil
		})

	preview, err := deps.service.PreviewSwap(t.Context(), 7, SwapCommand{Level: planningdom.SwapRotation})
	if err != nil {
		t.Fatalf("PreviewSwap() error = %v", err)
	}

	if len(offered) != 1 || offered[0].ID != 2 {
		t.Fatalf("offered templates = %#v, want the current plan's template 1 held back", offered)
	}
	if preview.Level != swap.LevelFullRotation || len(preview.AffectedMeals) != 1 {
		t.Fatalf("preview = %#v, want the whole plan as the blast radius", preview)
	}
	if len(preview.ShoppingDelta.Added) != 1 || len(preview.ShoppingDelta.Removed) != 1 {
		t.Fatalf("shopping delta = %#v, want rice in and chicken out", preview.ShoppingDelta)
	}
}

func TestPreviewRotationFallsBackToTheWholeLibrary(t *testing.T) {
	deps := newDeps(t)
	deps.plans.EXPECT().Get(gomock.Any(), planningdom.PlanID(7)).Return(swappablePlan(), nil)
	deps.expectRotationInputs()
	calls := 0
	deps.planner.EXPECT().BuildPlan(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, input solver.Input) (solver.Result, error) {
			calls++
			if len(input.Recipes) == 1 {
				return solver.Result{Failure: &solver.Failure{Code: solver.FailureMacroInfeasible, EaterID: 1}}, nil
			}
			return rotatedPlan(), nil
		}).Times(2)

	if _, err := deps.service.PreviewSwap(t.Context(), 7, SwapCommand{Level: planningdom.SwapRotation}); err != nil {
		t.Fatalf("PreviewSwap() error = %v, want the full library tried when the rest cannot feed the plan", err)
	}
	if calls != 2 {
		t.Fatalf("solver calls = %d, want the restricted search followed by the full one", calls)
	}
}

func TestPreviewRotationExplainsWhenNothingElseFits(t *testing.T) {
	deps := newDeps(t)
	deps.plans.EXPECT().Get(gomock.Any(), planningdom.PlanID(7)).Return(swappablePlan(), nil)
	deps.expectRotationInputs()
	deps.planner.EXPECT().BuildPlan(gomock.Any(), gomock.Any()).
		Return(solver.Result{Failure: &solver.Failure{Code: solver.FailureMacroInfeasible, EaterID: 1}}, nil).
		Times(2)

	_, err := deps.service.PreviewSwap(t.Context(), 7, SwapCommand{Level: planningdom.SwapRotation})

	var appErr *apperr.Error
	if !errors.As(err, &appErr) || appErr.Code != apperr.CodeValidation || len(appErr.Details) == 0 {
		t.Fatalf("PreviewSwap() error = %v, want a validation error carrying the solver diagnostics", err)
	}
}

func TestRotationIsRefusedOnceAMealIsLogged(t *testing.T) {
	deps := newDeps(t)
	plan := swappablePlan()
	plan.Meals[0].LogStatus = planningdom.MealLogStatusEatenAsPlanned
	deps.plans.EXPECT().Get(gomock.Any(), planningdom.PlanID(7)).Return(plan, nil)
	deps.expectRotationInputs()

	_, err := deps.service.PreviewSwap(t.Context(), 7, SwapCommand{Level: planningdom.SwapRotation})

	var appErr *apperr.Error
	if !errors.As(err, &appErr) || appErr.Code != apperr.CodeConflict {
		t.Fatalf("PreviewSwap() error = %v, want a conflict: eaten food cannot be rotated away", err)
	}
}

func TestApplyRotationStoresTheNewPlanAndArchivesTheOldOne(t *testing.T) {
	deps := newDeps(t)
	deps.plans.EXPECT().Get(gomock.Any(), planningdom.PlanID(7)).Return(swappablePlan(), nil)
	deps.expectRotationInputs()
	solverDepth := -1
	deps.planner.EXPECT().BuildPlan(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ solver.Input) (solver.Result, error) {
			solverDepth = deps.txDepth
			return rotatedPlan(), nil
		})
	deps.plans.EXPECT().Save(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, plan planningdom.Plan) (planningdom.Plan, error) {
			plan.ID = 8
			return plan, nil
		})
	deps.plans.EXPECT().SaveQualitySnapshots(gomock.Any(), planningdom.PlanID(8), gomock.Any()).Return(nil)
	deps.plans.EXPECT().Archive(gomock.Any(), planningdom.PlanID(7)).Return(nil)

	result, err := deps.service.ApplySwap(t.Context(), 7, SwapCommand{Level: planningdom.SwapRotation}, true)
	if err != nil {
		t.Fatalf("ApplySwap() error = %v", err)
	}

	if result.Plan.ID != 8 || result.Plan.Recipes[0].TemplateID != 2 {
		t.Fatalf("result = %#v, want the newly stored rotation", result.Plan)
	}
	if solverDepth != 0 {
		t.Fatalf("solver ran at transaction depth %d, want the search outside any transaction", solverDepth)
	}
	if !deps.committed {
		t.Fatal("the rotation transaction did not commit")
	}
}

func TestApplyRotationWithoutConfirmationStoresNothing(t *testing.T) {
	deps := newDeps(t)
	deps.plans.EXPECT().Get(gomock.Any(), planningdom.PlanID(7)).Return(swappablePlan(), nil)
	deps.expectRotationInputs()
	deps.planner.EXPECT().BuildPlan(gomock.Any(), gomock.Any()).Return(rotatedPlan(), nil)

	_, err := deps.service.ApplySwap(t.Context(), 7, SwapCommand{Level: planningdom.SwapRotation}, false)

	var unconfirmed *UnconfirmedSwapError
	if !errors.As(err, &unconfirmed) || unconfirmed.Preview.Level != swap.LevelFullRotation {
		t.Fatalf("ApplySwap() error = %v, want the rotation preview returned for confirmation", err)
	}
}

func TestRotationIsRefusedWhenTheSolverRebuildsTheSamePlan(t *testing.T) {
	deps := newDeps(t)
	deps.plans.EXPECT().Get(gomock.Any(), planningdom.PlanID(7)).Return(swappablePlan(), nil)
	deps.expectRotationInputs()
	same := solvedResult()
	same.Plan = swappablePlan()
	deps.planner.EXPECT().BuildPlan(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, input solver.Input) (solver.Result, error) {
			if len(input.Recipes) == 1 {
				return solver.Result{Failure: &solver.Failure{Code: solver.FailureMacroInfeasible, EaterID: 1}}, nil
			}
			return same, nil
		}).Times(2)

	_, err := deps.service.PreviewSwap(t.Context(), 7, SwapCommand{Level: planningdom.SwapRotation})

	var appErr *apperr.Error
	if !errors.As(err, &appErr) || appErr.Code != apperr.CodeValidation {
		t.Fatalf("PreviewSwap() error = %v, want a validation error: the same batches are not a new rotation", err)
	}
}

func TestSwapIsRefusedOnAnArchivedPlan(t *testing.T) {
	deps := newDeps(t)
	plan := swappablePlan()
	plan.Status = planningdom.PlanStatusArchived
	deps.plans.EXPECT().Get(gomock.Any(), planningdom.PlanID(7)).Return(plan, nil)

	_, err := deps.service.PreviewSwap(t.Context(), 7,
		SwapCommand{Level: planningdom.SwapFill, SlotFillID: 11, NewProductID: 2})

	var appErr *apperr.Error
	if !errors.As(err, &appErr) || appErr.Code != apperr.CodeConflict {
		t.Fatalf("PreviewSwap() error = %v, want a conflict: an archived plan is history", err)
	}
}

func TestRecipeSwapIsRefusedOnceAMealOfTheBatchIsLogged(t *testing.T) {
	deps := newDeps(t)
	plan := swappablePlan()
	plan.Meals[0].LogStatus = planningdom.MealLogStatusEatenAsPlanned
	deps.plans.EXPECT().Get(gomock.Any(), planningdom.PlanID(7)).Return(plan, nil)
	deps.expectRichSwapCatalogue()

	_, err := deps.service.PreviewSwap(t.Context(), 7,
		SwapCommand{Level: planningdom.SwapRecipe, PlanRecipeID: 3, NewTemplateID: 2})

	var appErr *apperr.Error
	if !errors.As(err, &appErr) || appErr.Code != apperr.CodeConflict {
		t.Fatalf("PreviewSwap() error = %v, want a conflict: an eaten batch keeps its recipe", err)
	}
}
