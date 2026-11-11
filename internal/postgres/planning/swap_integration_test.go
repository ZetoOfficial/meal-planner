//go:build integration

package planning_test

import (
	"context"
	"errors"
	"testing"

	"github.com/ZetoOfficial/meal-planner/internal/apperr"
	catalogdom "github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
	mealdom "github.com/ZetoOfficial/meal-planner/internal/domain/meal"
	"github.com/ZetoOfficial/meal-planner/internal/domain/nutrition"
	planningdom "github.com/ZetoOfficial/meal-planner/internal/domain/planning"
	recipedom "github.com/ZetoOfficial/meal-planner/internal/domain/recipe"
	postgresdb "github.com/ZetoOfficial/meal-planner/internal/postgres"
	analyticsrepo "github.com/ZetoOfficial/meal-planner/internal/postgres/analytics"
	catalogrepo "github.com/ZetoOfficial/meal-planner/internal/postgres/catalog"
	eaterrepo "github.com/ZetoOfficial/meal-planner/internal/postgres/eater"
	reciperepo "github.com/ZetoOfficial/meal-planner/internal/postgres/recipe"
	"github.com/ZetoOfficial/meal-planner/internal/usecase/reality"
)

// secondTemplate seeds another template filled by the fixture's role, so a
// batch has something to be swapped to.
func (f *planFixture) secondTemplate() recipedom.Template {
	f.t.Helper()
	store := postgresdb.NewStore(f.database.Pool)
	roles, err := catalogrepo.New(store).ListRoleTags(f.ctx)
	if err != nil || len(roles) == 0 {
		f.t.Fatalf("list role tags: %v", err)
	}
	recipes := reciperepo.New(store)
	resolved, err := recipes.ResolveRoleTags(f.ctx, []string{roles[0].Code})
	if err != nil {
		f.t.Fatalf("resolve role tags: %v", err)
	}
	template, err := recipes.Create(f.ctx, recipedom.Template{
		Name:             "Other batch",
		Status:           recipedom.StatusPermanent,
		Source:           recipedom.SourceManual,
		ComponentKind:    recipedom.ComponentKindMixedMain,
		AllowedMealTypes: []mealdom.Type{mealdom.TypeLunch},
		BaseServings:     4,
		Storage:          recipedom.StorageAssembleFresh,
		Weight:           1,
		Slots:            []recipedom.Slot{{RoleTagCode: roles[0].Code, Label: "Main", QtyG: 300, Position: 0}},
	}, resolved)
	if err != nil {
		f.t.Fatalf("seed second recipe: %v", err)
	}
	return template
}

func TestReplacePlanRecipeRewritesTheBatchAndKeepsTheMeals(t *testing.T) {
	fixture := newPlanFixture(t)
	saved, err := fixture.repository.Save(fixture.ctx, fixture.plan())
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	other := fixture.secondTemplate()

	swapped := saved
	swapped.Recipes = append([]planningdom.PlanRecipe(nil), saved.Recipes...)
	swapped.Recipes[0].TemplateID = other.ID
	swapped.Recipes[0].ScaleFactor = 1
	swapped.Recipes[0].ExpectedOutputG = nil
	swapped.Recipes[0].Fills = []planningdom.SlotFill{{
		SlotID: other.Slots[0].ID, ProductID: fixture.productID, PlannedInputQtyG: 300,
	}}
	swapped.Meals = append([]planningdom.Meal(nil), saved.Meals...)
	newMacros := nutrition.Macros{ProteinG: 45, FatG: 15, CarbG: 60, Kcal: 600}
	for i := range swapped.Meals {
		if swapped.Meals[i].FixedMealID != nil {
			continue
		}
		swapped.Meals[i].Planned = newMacros
		swapped.Meals[i].Components = []planningdom.ComponentAllocation{{PlanRecipeIndex: 0, PortionFactor: 0.5}}
	}

	err = fixture.transactions.WithTx(fixture.ctx, func(ctx context.Context) error {
		return fixture.repository.ReplacePlanRecipe(ctx, saved.ID, swapped, 0)
	})
	if err != nil {
		t.Fatalf("ReplacePlanRecipe() error = %v", err)
	}

	loaded, err := fixture.repository.Get(fixture.ctx, saved.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if err := loaded.Validate(); err != nil {
		t.Fatalf("loaded plan is not consistent: %v", err)
	}
	recipe := loaded.Recipes[0]
	if recipe.ID != saved.Recipes[0].ID || recipe.TemplateID != other.ID || recipe.ScaleFactor != 1 {
		t.Fatalf("recipe = %#v, want the same batch row on the new template", recipe)
	}
	if recipe.ExpectedOutputG != nil {
		t.Fatalf("expected output = %v, want unknown for a template without a yield", *recipe.ExpectedOutputG)
	}
	if len(recipe.Fills) != 1 || recipe.Fills[0].SlotID != other.Slots[0].ID || recipe.Fills[0].PlannedInputQtyG != 300 {
		t.Fatalf("fills = %#v, want only the new template's slot", recipe.Fills)
	}
	lunch := mealOfType(t, loaded, mealdom.TypeLunch)
	if lunch.ID != mealOfType(t, saved, mealdom.TypeLunch).ID {
		t.Fatalf("lunch id = %d, want the meal row kept so its log survives", lunch.ID)
	}
	if lunch.Planned != newMacros {
		t.Fatalf("planned macros = %#v, want the recomputed ones", lunch.Planned)
	}
	if lunch.Components[0].PlannedServingG != nil {
		t.Fatalf("planned serving = %v, want unknown once the yield is unknown", *lunch.Components[0].PlannedServingG)
	}
	if !loaded.ShoppingListStale {
		t.Fatal("shopping list is not stale after the batch changed its products")
	}
}

func TestPrepDependenciesRoundTripByRecipeIndex(t *testing.T) {
	fixture := newPlanFixture(t)
	other := fixture.secondTemplate()
	plan := fixture.plan()
	plan.Recipes = append(plan.Recipes, planningdom.PlanRecipe{
		TemplateID:  other.ID,
		ScaleFactor: 1,
		Fills:       []planningdom.SlotFill{{SlotID: other.Slots[0].ID, ProductID: fixture.productID, PlannedInputQtyG: 300}},
	})
	note := "assembled fresh from plan recipe 1"
	plan.PrepDependencies = []planningdom.PrepDependency{{DependentRecipeIndex: 1, SourceRecipeIndex: 0, Note: &note}}

	saved, err := fixture.repository.Save(fixture.ctx, plan)
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	if len(saved.PrepDependencies) != 1 {
		t.Fatalf("prep dependencies = %#v, want the stored edge", saved.PrepDependencies)
	}
	edge := saved.PrepDependencies[0]
	if edge.DependentRecipeIndex != 1 || edge.SourceRecipeIndex != 0 || edge.Note == nil || *edge.Note != note {
		t.Fatalf("edge = %#v, want the assembled batch depending on the cooked one", edge)
	}
}

func TestListTemplateLastPlannedSeesOnlyEarlierPlans(t *testing.T) {
	fixture := newPlanFixture(t)
	if _, err := fixture.repository.Save(fixture.ctx, fixture.plan()); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	same, err := fixture.repository.ListTemplateLastPlanned(fixture.ctx, fixture.start)
	if err != nil {
		t.Fatalf("ListTemplateLastPlanned() error = %v", err)
	}
	if len(same) != 0 {
		t.Fatalf("last planned = %v, want a plan starting the same day left out", same)
	}

	later, err := fixture.repository.ListTemplateLastPlanned(fixture.ctx, fixture.start.AddDate(0, 0, 7))
	if err != nil {
		t.Fatalf("ListTemplateLastPlanned() error = %v", err)
	}
	if got, ok := later[fixture.templateID]; !ok || !got.Equal(fixture.start) {
		t.Fatalf("last planned = %v, want the template planned on %s", later, fixture.start)
	}
}

func TestUpdatePlanQualityAndMealMacros(t *testing.T) {
	fixture := newPlanFixture(t)
	saved, err := fixture.repository.Save(fixture.ctx, fixture.plan())
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	score := 72.5
	if err := fixture.repository.UpdatePlanQuality(fixture.ctx, saved.ID, &score, 3); err != nil {
		t.Fatalf("UpdatePlanQuality() error = %v", err)
	}
	changed := saved
	changed.Meals = append([]planningdom.Meal(nil), saved.Meals...)
	macros := nutrition.Macros{ProteinG: 33, FatG: 11, CarbG: 44, Kcal: 440}
	for i := range changed.Meals {
		if changed.Meals[i].FixedMealID == nil {
			changed.Meals[i].Planned = macros
		}
	}
	if err := fixture.repository.UpdateMealMacros(fixture.ctx, changed); err != nil {
		t.Fatalf("UpdateMealMacros() error = %v", err)
	}

	loaded, err := fixture.repository.Get(fixture.ctx, saved.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if loaded.QualityScore == nil || *loaded.QualityScore != score || loaded.QualityWarningsCount != 3 {
		t.Fatalf("headline = %v / %d, want 72.5 / 3", loaded.QualityScore, loaded.QualityWarningsCount)
	}
	if got := mealOfType(t, loaded, mealdom.TypeLunch).Planned; got != macros {
		t.Fatalf("lunch macros = %#v, want the updated ones", got)
	}
	fixedMacros := nutrition.Macros{ProteinG: 20, FatG: 8, CarbG: 30, Kcal: 300}
	if got := mealOfType(t, loaded, mealdom.TypeBreakfast).Planned; got != fixedMacros {
		t.Fatalf("fixed breakfast macros = %#v, want them untouched", got)
	}
}

func TestProductFrequencyReadsEachDayFromOnePlan(t *testing.T) {
	fixture := newPlanFixture(t)
	saved, err := fixture.repository.Save(fixture.ctx, fixture.plan())
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	analytics := analyticsrepo.New(postgresdb.NewStore(fixture.database.Pool))
	to := fixture.start.AddDate(0, 0, 6)
	frequency := func() []reality.ProductFrequency {
		t.Helper()
		rows, err := analytics.ListProductFrequency(fixture.ctx, fixture.eaterID, fixture.start, to, 10)
		if err != nil {
			t.Fatalf("ListProductFrequency() error = %v", err)
		}
		return rows
	}

	// Two lunches each take half of a 200 g fill.
	rows := frequency()
	if len(rows) != 1 || rows[0].ProductID != catalogdom.ProductID(fixture.productID) ||
		rows[0].MealsCount != 2 || rows[0].TotalQtyG != 200 {
		t.Fatalf("frequency = %#v, want the product in two meals for 200 g", rows)
	}

	// A second draft for the same days must not double the figures.
	duplicate, err := fixture.repository.Save(fixture.ctx, fixture.plan())
	if err != nil {
		t.Fatalf("Save() duplicate error = %v", err)
	}
	if rows = frequency(); len(rows) != 1 || rows[0].MealsCount != 2 || rows[0].TotalQtyG != 200 {
		t.Fatalf("frequency = %#v, want two drafts for the same days counted once", rows)
	}
	meals, err := fixture.repository.ListEaterMeals(fixture.ctx, fixture.eaterID, fixture.start, to)
	if err != nil {
		t.Fatalf("ListEaterMeals() error = %v", err)
	}
	for i := range meals {
		if meals[i].PlanID != duplicate.ID {
			t.Fatalf("meal = %#v, want every day read from the latest draft %d", meals[i], duplicate.ID)
		}
	}
	if err := fixture.repository.Archive(fixture.ctx, duplicate.ID); err != nil {
		t.Fatalf("Archive() error = %v", err)
	}
	if rows = frequency(); len(rows) != 1 || rows[0].MealsCount != 2 || rows[0].TotalQtyG != 200 {
		t.Fatalf("frequency = %#v, want the remaining plan to take over from the archived draft", rows)
	}

	lunch := mealOfType(t, saved, mealdom.TypeLunch)
	if _, err := fixture.repository.LogMeal(fixture.ctx, lunch.ID, planningdom.MealLog{
		Status: planningdom.MealLogStatusSkipped, LoggedBy: fixture.eaterID,
	}); err != nil {
		t.Fatalf("LogMeal() error = %v", err)
	}
	if rows = frequency(); len(rows) != 1 || rows[0].MealsCount != 1 || rows[0].TotalQtyG != 100 {
		t.Fatalf("frequency = %#v, want the skipped lunch left out", rows)
	}

	// An archived plan keeps the days that were logged in it and drops the rest.
	if err := fixture.repository.Archive(fixture.ctx, saved.ID); err != nil {
		t.Fatalf("Archive() error = %v", err)
	}
	if rows = frequency(); len(rows) != 0 {
		t.Fatalf("frequency = %#v, want the unlogged day of an archived plan left out", rows)
	}
	meals, err = fixture.repository.ListEaterMeals(fixture.ctx, fixture.eaterID, fixture.start, to)
	if err != nil {
		t.Fatalf("ListEaterMeals() error = %v", err)
	}
	if len(meals) == 0 {
		t.Fatal("meals = none, want the logged day of the archived plan kept as history")
	}
	for i := range meals {
		if !meals[i].Date.Equal(lunch.Date) {
			t.Fatalf("meal = %#v, want only the day that was logged", meals[i])
		}
	}
}

func TestEaterWithPlanHistoryCannotBeDeleted(t *testing.T) {
	fixture := newPlanFixture(t)
	if _, err := fixture.repository.Save(fixture.ctx, fixture.plan()); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	err := eaterrepo.New(postgresdb.NewStore(fixture.database.Pool)).Delete(fixture.ctx, fixture.eaterID)

	var appErr *apperr.Error
	if !errors.As(err, &appErr) || appErr.Code != apperr.CodeConflict {
		t.Fatalf("Delete() error = %v, want a conflict: plans still refer to the eater", err)
	}
}
