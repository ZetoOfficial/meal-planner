//go:build integration

package planning_test

import (
	"context"
	"errors"
	"testing"
	"time"

	catalogdom "github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
	eaterdom "github.com/ZetoOfficial/meal-planner/internal/domain/eater"
	mealdom "github.com/ZetoOfficial/meal-planner/internal/domain/meal"
	"github.com/ZetoOfficial/meal-planner/internal/domain/nutrition"
	planningdom "github.com/ZetoOfficial/meal-planner/internal/domain/planning"
	recipedom "github.com/ZetoOfficial/meal-planner/internal/domain/recipe"
	postgresdb "github.com/ZetoOfficial/meal-planner/internal/postgres"
	catalogrepo "github.com/ZetoOfficial/meal-planner/internal/postgres/catalog"
	eaterrepo "github.com/ZetoOfficial/meal-planner/internal/postgres/eater"
	planningrepo "github.com/ZetoOfficial/meal-planner/internal/postgres/planning"
	reciperepo "github.com/ZetoOfficial/meal-planner/internal/postgres/recipe"
	testpostgres "github.com/ZetoOfficial/meal-planner/internal/testkit/postgres"
)

func TestPlanRoundTripRestoresAllocationsAndFills(t *testing.T) {
	fixture := newPlanFixture(t)

	saved, err := fixture.repository.Save(fixture.ctx, fixture.plan())
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if !saved.ID.Valid() {
		t.Fatalf("saved plan id = %d, want a generated identity", saved.ID)
	}

	loaded, err := fixture.repository.Get(fixture.ctx, saved.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if err := loaded.Validate(); err != nil {
		t.Fatalf("loaded plan is not consistent: %v", err)
	}

	if len(loaded.EaterDays) != 2 || len(loaded.Recipes) != 1 || len(loaded.Meals) != 3 {
		t.Fatalf("loaded plan shape = %d days, %d recipes, %d meals; want 2/1/3",
			len(loaded.EaterDays), len(loaded.Recipes), len(loaded.Meals))
	}

	recipe := loaded.Recipes[0]
	if recipe.ScaleFactor != 2 || len(recipe.Fills) != 1 {
		t.Fatalf("loaded plan recipe = %#v, want scale 2 with one fill", recipe)
	}
	if recipe.Fills[0].ProductID != fixture.productID || recipe.Fills[0].PlannedInputQtyG != 200 {
		t.Fatalf("loaded slot fill = %#v, want the stored product and quantity", recipe.Fills[0])
	}
	if recipe.ExpectedOutputG == nil || *recipe.ExpectedOutputG != 400 {
		t.Fatalf("loaded expected output = %v, want 400 g", recipe.ExpectedOutputG)
	}

	generated := mealOfType(t, loaded, mealdom.TypeLunch)
	if len(generated.Components) != 1 {
		t.Fatalf("generated meal components = %#v, want one allocation", generated.Components)
	}
	if generated.Components[0].PortionFactor != 0.5 || generated.Components[0].PlanRecipeIndex != 0 {
		t.Fatalf("allocation = %#v, want half of the first plan recipe", generated.Components[0])
	}
	if generated.Components[0].PlannedServingG == nil || *generated.Components[0].PlannedServingG != 200 {
		t.Fatalf("planned serving = %v, want 200 g", generated.Components[0].PlannedServingG)
	}
	if generated.Planned != (nutrition.Macros{ProteinG: 30, FatG: 10, CarbG: 40, Kcal: 400}) {
		t.Fatalf("planned macros = %#v, want the stored solver macros", generated.Planned)
	}

	fixed := mealOfType(t, loaded, mealdom.TypeBreakfast)
	if fixed.FixedMealID == nil || *fixed.FixedMealID != fixture.fixedMealID {
		t.Fatalf("fixed meal reference = %v, want the seeded fixed meal", fixed.FixedMealID)
	}
	if len(fixed.Components) != 0 {
		t.Fatalf("fixed meal components = %#v, want none", fixed.Components)
	}
	if loaded.Meals[0].LogStatus != planningdom.MealLogStatusPlanned {
		t.Fatalf("log status = %q, want planned", loaded.Meals[0].LogStatus)
	}
}

func TestPlanSaveRollsBackWhenAStepFails(t *testing.T) {
	fixture := newPlanFixture(t)
	broken := fixture.plan()
	// A slot fill for a product that does not exist fails the foreign key after
	// the plan row and its eater days are already inserted.
	broken.Recipes[0].Fills[0].ProductID = catalogdom.ProductID(999_999)

	err := fixture.transactions.WithTx(fixture.ctx, func(ctx context.Context) error {
		_, saveErr := fixture.repository.Save(ctx, broken)
		return saveErr
	})
	if err == nil {
		t.Fatal("Save() = nil, want a failure for an unknown product")
	}

	var count int
	if queryErr := fixture.database.Pool.QueryRow(fixture.ctx, "SELECT count(*) FROM plan").Scan(&count); queryErr != nil {
		t.Fatalf("count plans: %v", queryErr)
	}
	if count != 0 {
		t.Fatalf("stored plans = %d, want the failed transaction to leave none", count)
	}
}

func TestPlanSaveRejectsInconsistentAggregate(t *testing.T) {
	fixture := newPlanFixture(t)
	inconsistent := fixture.plan()
	inconsistent.Meals[1].Components = nil

	if _, err := fixture.repository.Save(fixture.ctx, inconsistent); err == nil {
		t.Fatal("Save() = nil, want a generated meal without allocations to be rejected")
	}
}

func TestPlanGetReportsMissingPlan(t *testing.T) {
	fixture := newPlanFixture(t)

	_, err := fixture.repository.Get(fixture.ctx, planningdom.PlanID(999_999))
	if err == nil {
		t.Fatal("Get() = nil, want an error for an unknown plan")
	}
	if errors.Is(err, context.Canceled) {
		t.Fatalf("Get() error = %v, want a not found error", err)
	}
}

type planFixture struct {
	t            *testing.T
	ctx          context.Context
	database     testpostgres.Database
	repository   *planningrepo.Repository
	transactions *postgresdb.TxManager

	start       time.Time
	eaterID     eaterdom.ID
	goalID      eaterdom.MacroGoalID
	fixedMealID eaterdom.FixedMealID
	productID   catalogdom.ProductID
	templateID  recipedom.ID
	slotID      recipedom.SlotID
}

func newPlanFixture(t *testing.T) *planFixture {
	t.Helper()
	database := testpostgres.RequireDatabase(t)
	store := postgresdb.NewStore(database.Pool)
	ctx := context.Background()
	start := time.Date(2026, time.August, 10, 0, 0, 0, 0, time.UTC)

	eater, err := eaterrepo.New(store).Create(ctx,
		eaterdom.Eater{Name: "Owner", Sex: eaterdom.SexUnspecified},
		eaterdom.MacroGoal{
			EffectiveFrom: start,
			Range: eaterdom.MacroRange{
				ProteinMinG: 100, ProteinMaxG: 200,
				FatMinG: 30, FatMaxG: 90,
				CarbMinG: 100, CarbMaxG: 300,
				KcalMin: 1500, KcalMax: 2500,
			},
		})
	if err != nil {
		t.Fatalf("seed eater: %v", err)
	}
	goals, err := eaterrepo.New(store).ListGoals(ctx, eater.ID)
	if err != nil || len(goals) == 0 {
		t.Fatalf("seed macro goal: %v", err)
	}
	fixedMeal, err := eaterrepo.New(store).CreateFixedMeal(ctx, eaterdom.FixedMeal{
		EaterID:  eater.ID,
		MealType: mealdom.TypeBreakfast,
		Label:    "Yogurt",
		Macros:   nutrition.Macros{ProteinG: 20, FatG: 8, CarbG: 30, Kcal: 300},
		Active:   true,
	})
	if err != nil {
		t.Fatalf("seed fixed meal: %v", err)
	}

	roles, err := catalogrepo.New(store).ListRoleTags(ctx)
	if err != nil || len(roles) == 0 {
		t.Fatalf("seed role tags: %v", err)
	}
	role := roles[0]
	product, err := catalogrepo.New(store).Create(ctx, catalogdom.Product{
		Brand:          "Test",
		Name:           "Chicken breast",
		ProteinPer100G: 30,
		FatPer100G:     10,
		CarbPer100G:    40,
		KcalPer100G:    400,
		DefaultUnit:    "g",
		Purchasable:    true,
		DataQuality:    catalogdom.NutritionData{Quality: catalogdom.NutritionDataQualityLabelExact},
		Classification: catalogdom.FoodClassification{ProcessingLevel: catalogdom.ProcessingLevelMinimallyProcessed},
		Roles:          []catalogdom.RoleTag{role},
	})
	if err != nil {
		t.Fatalf("seed product: %v", err)
	}

	recipes := reciperepo.New(store)
	resolved, err := recipes.ResolveRoleTags(ctx, []string{role.Code})
	if err != nil {
		t.Fatalf("resolve role tags: %v", err)
	}
	expectedOutput := 200.0
	template, err := recipes.Create(ctx, recipedom.Template{
		Name:             "Batch",
		Status:           recipedom.StatusPermanent,
		Source:           recipedom.SourceManual,
		ComponentKind:    recipedom.ComponentKindMixedMain,
		AllowedMealTypes: []mealdom.Type{mealdom.TypeLunch},
		BaseServings:     2,
		ExpectedOutputG:  &expectedOutput,
		Storage:          recipedom.StorageKeeps3d,
		Equipment:        []string{"oven"},
		Slots:            []recipedom.Slot{{RoleTagCode: role.Code, Label: "Main", QtyG: 100, Position: 0}},
	}, resolved)
	if err != nil {
		t.Fatalf("seed recipe: %v", err)
	}

	return &planFixture{
		t:            t,
		ctx:          ctx,
		database:     database,
		repository:   planningrepo.New(store),
		transactions: postgresdb.NewTxManager(database.Pool),
		start:        start,
		eaterID:      eater.ID,
		goalID:       goals[0].ID,
		fixedMealID:  fixedMeal.ID,
		productID:    product.ID,
		templateID:   template.ID,
		slotID:       template.Slots[0].ID,
	}
}

func (f *planFixture) plan() planningdom.Plan {
	expectedOutput := 400.0
	servingG := 200.0
	macros := nutrition.Macros{ProteinG: 30, FatG: 10, CarbG: 40, Kcal: 400}
	fixedMacros := nutrition.Macros{ProteinG: 20, FatG: 8, CarbG: 30, Kcal: 300}
	second := f.start.AddDate(0, 0, 1)

	return planningdom.Plan{
		StartDate: f.start,
		Days:      2,
		CookDate:  f.start,
		Novelty:   1,
		Status:    planningdom.PlanStatusDraft,
		EaterDays: []planningdom.EaterDay{
			{EaterID: f.eaterID, Date: f.start, MacroGoalID: f.goalID, GeneratedMealTypes: []mealdom.Type{mealdom.TypeLunch}},
			{EaterID: f.eaterID, Date: second, MacroGoalID: f.goalID, GeneratedMealTypes: []mealdom.Type{mealdom.TypeLunch}},
		},
		Recipes: []planningdom.PlanRecipe{{
			TemplateID:      f.templateID,
			ScaleFactor:     2,
			ExpectedOutputG: &expectedOutput,
			Fills:           []planningdom.SlotFill{{SlotID: f.slotID, ProductID: f.productID, PlannedInputQtyG: 200}},
		}},
		Meals: []planningdom.Meal{
			{
				EaterID:     f.eaterID,
				Date:        f.start,
				Type:        mealdom.TypeBreakfast,
				FixedMealID: &f.fixedMealID,
				Planned:     fixedMacros,
			},
			{
				EaterID:    f.eaterID,
				Date:       f.start,
				Type:       mealdom.TypeLunch,
				Planned:    macros,
				Components: []planningdom.ComponentAllocation{{PlanRecipeIndex: 0, PortionFactor: 0.5, PlannedServingG: &servingG}},
			},
			{
				EaterID:    f.eaterID,
				Date:       second,
				Type:       mealdom.TypeLunch,
				Planned:    macros,
				Components: []planningdom.ComponentAllocation{{PlanRecipeIndex: 0, PortionFactor: 0.5, PlannedServingG: &servingG}},
			},
		},
	}
}

func mealOfType(t *testing.T, plan planningdom.Plan, mealType mealdom.Type) planningdom.Meal {
	t.Helper()
	for _, meal := range plan.Meals {
		if meal.Type == mealType {
			return meal
		}
	}
	t.Fatalf("plan has no %s meal", mealType)
	return planningdom.Meal{}
}
