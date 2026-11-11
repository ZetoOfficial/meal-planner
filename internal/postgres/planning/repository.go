package planning

import (
	"context"
	"errors"
	"math"
	"sort"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/ZetoOfficial/meal-planner/internal/apperr"
	catalogdom "github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
	eaterdom "github.com/ZetoOfficial/meal-planner/internal/domain/eater"
	"github.com/ZetoOfficial/meal-planner/internal/domain/nutrition"
	planningdom "github.com/ZetoOfficial/meal-planner/internal/domain/planning"
	recipedom "github.com/ZetoOfficial/meal-planner/internal/domain/recipe"
	"github.com/ZetoOfficial/meal-planner/internal/postgres"
	"github.com/ZetoOfficial/meal-planner/internal/postgres/dberror"
	"github.com/ZetoOfficial/meal-planner/internal/postgres/pgvalue"
	"github.com/ZetoOfficial/meal-planner/internal/postgres/sqlc"
)

type Repository struct {
	store *postgres.Store
}

func New(store *postgres.Store) *Repository {
	return &Repository{store: store}
}

// Save writes the whole plan aggregate. It expects the caller's transaction:
// every statement runs on the same executor, so a failure anywhere leaves no
// partially written plan behind.
func (r *Repository) Save(ctx context.Context, plan planningdom.Plan) (planningdom.Plan, error) {
	if err := plan.Validate(); err != nil {
		return planningdom.Plan{}, apperr.Validation("plan is not internally consistent", err.Error())
	}

	query := r.store.Queries(ctx)
	row, err := query.CreatePlan(ctx, planParams(plan))
	if err != nil {
		return planningdom.Plan{}, dberror.Map(err, "plan")
	}
	planID := planningdom.PlanID(row.ID)

	for _, day := range plan.EaterDays {
		if _, err := query.CreatePlanEaterDay(ctx, eaterDayParams(planID, day)); err != nil {
			return planningdom.Plan{}, dberror.Map(err, "plan eater day")
		}
	}

	recipeIDs, err := r.saveRecipes(ctx, query, planID, plan.Recipes)
	if err != nil {
		return planningdom.Plan{}, err
	}
	if err := r.saveMeals(ctx, query, planID, plan.Meals, recipeIDs); err != nil {
		return planningdom.Plan{}, err
	}
	if err := r.savePrepDependencies(ctx, query, planID, plan.PrepDependencies, recipeIDs); err != nil {
		return planningdom.Plan{}, err
	}
	return r.Get(ctx, planID)
}

func (r *Repository) saveRecipes(
	ctx context.Context,
	query *sqlc.Queries,
	planID planningdom.PlanID,
	recipes []planningdom.PlanRecipe,
) ([]planningdom.PlanRecipeID, error) {
	ids := make([]planningdom.PlanRecipeID, 0, len(recipes))
	for i := range recipes {
		recipe := &recipes[i]
		stored, err := query.CreatePlanRecipe(ctx, planRecipeParams(planID, *recipe))
		if err != nil {
			return nil, dberror.Map(err, "plan recipe")
		}
		id := planningdom.PlanRecipeID(stored.ID)
		for _, fill := range recipe.Fills {
			if _, err := query.CreateSlotFill(ctx, slotFillParams(id, fill)); err != nil {
				return nil, dberror.Map(err, "slot fill")
			}
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func (r *Repository) saveMeals(
	ctx context.Context,
	query *sqlc.Queries,
	planID planningdom.PlanID,
	meals []planningdom.Meal,
	recipeIDs []planningdom.PlanRecipeID,
) error {
	for i := range meals {
		meal := &meals[i]
		stored, err := query.CreateMeal(ctx, mealParams(planID, *meal))
		if err != nil {
			return dberror.Map(err, "meal")
		}
		mealID := planningdom.MealID(stored.ID)
		for _, component := range meal.Components {
			if component.PlanRecipeIndex >= len(recipeIDs) {
				return apperr.Validation("meal allocates a plan recipe outside the plan", strconv.Itoa(component.PlanRecipeIndex))
			}
			params := allocationParams(mealID, recipeIDs[component.PlanRecipeIndex], component)
			if _, err := query.CreateMealComponentAllocation(ctx, params); err != nil {
				return dberror.Map(err, "meal component allocation")
			}
		}
	}
	return nil
}

// savePrepDependencies stores the graph. The aggregate addresses batches by
// slice index; recipeIDs holds the database id each index just received.
func (r *Repository) savePrepDependencies(
	ctx context.Context,
	query *sqlc.Queries,
	planID planningdom.PlanID,
	dependencies []planningdom.PrepDependency,
	recipeIDs []planningdom.PlanRecipeID,
) error {
	for _, dependency := range dependencies {
		if dependency.DependentRecipeIndex < 0 || dependency.DependentRecipeIndex >= len(recipeIDs) ||
			dependency.SourceRecipeIndex < 0 || dependency.SourceRecipeIndex >= len(recipeIDs) {
			return apperr.Validation("prep dependency points to a plan recipe outside the plan")
		}
		params := prepDependencyParams(
			planID,
			recipeIDs[dependency.DependentRecipeIndex],
			recipeIDs[dependency.SourceRecipeIndex],
			dependency.Note,
		)
		if _, err := query.CreatePrepDependency(ctx, params); err != nil {
			return dberror.Map(err, "prep dependency")
		}
	}
	return nil
}

func (r *Repository) Get(ctx context.Context, id planningdom.PlanID) (planningdom.Plan, error) {
	query := r.store.Queries(ctx)
	row, err := query.GetPlan(ctx, int64(id))
	if err != nil {
		return planningdom.Plan{}, dberror.Map(err, "plan")
	}
	plan, err := planFromRow(row)
	if err != nil {
		return planningdom.Plan{}, err
	}

	if plan.EaterDays, err = r.loadEaterDays(ctx, query, id); err != nil {
		return planningdom.Plan{}, err
	}
	recipeIndex, err := r.loadRecipes(ctx, query, id, &plan)
	if err != nil {
		return planningdom.Plan{}, err
	}
	if err := r.loadMeals(ctx, query, id, &plan, recipeIndex); err != nil {
		return planningdom.Plan{}, err
	}
	dependencies, err := query.ListPrepDependencies(ctx, int64(id))
	if err != nil {
		return planningdom.Plan{}, dberror.Map(err, "prep dependency")
	}
	plan.PrepDependencies = make([]planningdom.PrepDependency, 0, len(dependencies))
	for i := range dependencies {
		dependency, err := prepDependencyFromRow(dependencies[i], recipeIndex)
		if err != nil {
			return planningdom.Plan{}, err
		}
		plan.PrepDependencies = append(plan.PrepDependencies, dependency)
	}
	return plan, nil
}

func (r *Repository) loadEaterDays(
	ctx context.Context,
	query *sqlc.Queries,
	id planningdom.PlanID,
) ([]planningdom.EaterDay, error) {
	rows, err := query.ListPlanEaterDays(ctx, int64(id))
	if err != nil {
		return nil, dberror.Map(err, "plan eater day")
	}
	days := make([]planningdom.EaterDay, 0, len(rows))
	for _, row := range rows {
		days = append(days, eaterDayFromRow(row))
	}
	return days, nil
}

// loadRecipes returns the stored plan recipe id to slice index mapping that
// component allocations use to address batches without database ids leaking
// into the domain aggregate.
func (r *Repository) loadRecipes(
	ctx context.Context,
	query *sqlc.Queries,
	id planningdom.PlanID,
	plan *planningdom.Plan,
) (map[int64]int, error) {
	rows, err := query.ListPlanRecipes(ctx, int64(id))
	if err != nil {
		return nil, dberror.Map(err, "plan recipe")
	}
	index := make(map[int64]int, len(rows))
	plan.Recipes = make([]planningdom.PlanRecipe, 0, len(rows))
	for i, row := range rows {
		recipe, err := planRecipeFromRow(row)
		if err != nil {
			return nil, err
		}
		index[row.ID] = i
		plan.Recipes = append(plan.Recipes, recipe)
	}

	fills, err := query.ListPlanSlotFills(ctx, int64(id))
	if err != nil {
		return nil, dberror.Map(err, "slot fill")
	}
	for _, row := range fills {
		position, ok := index[row.PlanRecipeID]
		if !ok {
			return nil, apperr.Conflict("slot fill references a plan recipe outside the plan")
		}
		fill, err := slotFillFromRow(row)
		if err != nil {
			return nil, err
		}
		plan.Recipes[position].Fills = append(plan.Recipes[position].Fills, fill)
	}
	return index, nil
}

func (r *Repository) loadMeals(
	ctx context.Context,
	query *sqlc.Queries,
	id planningdom.PlanID,
	plan *planningdom.Plan,
	recipeIndex map[int64]int,
) error {
	rows, err := query.ListPlanMeals(ctx, int64(id))
	if err != nil {
		return dberror.Map(err, "meal")
	}
	mealIndex := make(map[int64]int, len(rows))
	plan.Meals = make([]planningdom.Meal, 0, len(rows))
	for i := range rows {
		meal, err := mealFromRow(rows[i])
		if err != nil {
			return err
		}
		mealIndex[rows[i].ID] = i
		plan.Meals = append(plan.Meals, meal)
	}

	allocations, err := query.ListPlanMealAllocations(ctx, int64(id))
	if err != nil {
		return dberror.Map(err, "meal component allocation")
	}
	for _, row := range allocations {
		position, ok := mealIndex[row.MealID]
		if !ok {
			return apperr.Conflict("component allocation references a meal outside the plan")
		}
		recipePosition, ok := recipeIndex[row.PlanRecipeID]
		if !ok {
			return apperr.Conflict("component allocation references a plan recipe outside the plan")
		}
		component, err := allocationFromRow(row, recipePosition)
		if err != nil {
			return err
		}
		plan.Meals[position].Components = append(plan.Meals[position].Components, component)
	}
	return nil
}

// SaveQualitySnapshots replaces the planned quality snapshot of every supplied
// eater-day. Identity is (plan, eater, date, basis), so regenerating a plan
// overwrites only its own snapshot and never another plan's.
func (r *Repository) SaveQualitySnapshots(
	ctx context.Context,
	planID planningdom.PlanID,
	snapshots []planningdom.QualitySnapshot,
) error {
	query := r.store.Queries(ctx)
	for i := range snapshots {
		snapshot := &snapshots[i]
		day, err := query.UpsertDayNutritionSummary(ctx, daySummaryParams(planID, *snapshot))
		if err != nil {
			return dberror.Map(err, "day nutrition summary")
		}
		if err := r.saveRuleResults(ctx, query, planningdom.RuleEntityDay, day.ID, *snapshot, snapshot.DayRules); err != nil {
			return err
		}

		if snapshot.Rolling.WindowDays <= 0 {
			continue
		}
		rolling, err := query.UpsertRollingNutritionSummary(ctx, rollingSummaryParams(planID, *snapshot))
		if err != nil {
			return dberror.Map(err, "rolling nutrition summary")
		}
		entity := planningdom.RuleEntityRollingWindow
		if err := r.saveRuleResults(ctx, query, entity, rolling.ID, *snapshot, snapshot.RollingRules); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) saveRuleResults(
	ctx context.Context,
	query *sqlc.Queries,
	entityType planningdom.RuleEntityType,
	entityID int64,
	snapshot planningdom.QualitySnapshot,
	results []nutrition.RuleEvaluationResult,
) error {
	for _, result := range results {
		if _, err := query.UpsertRuleEvaluationResult(ctx, ruleResultParams(entityType, entityID, snapshot, result)); err != nil {
			return dberror.Map(err, "rule evaluation result")
		}
	}
	return nil
}

// ListQualitySnapshots reassembles the stored evaluation of a plan: one
// snapshot per eater-day with its day summary, rolling projection and the rule
// verdicts that explain both.
func (r *Repository) ListQualitySnapshots(
	ctx context.Context,
	planID planningdom.PlanID,
	basis nutrition.SummaryBasis,
) ([]planningdom.QualitySnapshot, error) {
	query := r.store.Queries(ctx)
	dayRows, err := query.ListPlanDaySummaries(ctx, sqlc.ListPlanDaySummariesParams{
		PlanID: int64(planID),
		Basis:  sqlc.SummaryBasis(basis),
	})
	if err != nil {
		return nil, dberror.Map(err, "day nutrition summary")
	}

	snapshots := make([]planningdom.QualitySnapshot, 0, len(dayRows))
	byKey := make(map[string]int, len(dayRows))
	dayIDs := make([]int64, 0, len(dayRows))
	dayEntities := make(map[int64]int, len(dayRows))
	for i := range dayRows {
		snapshot, err := daySummaryFromRow(dayRows[i])
		if err != nil {
			return nil, err
		}
		byKey[snapshotKey(snapshot.EaterID, snapshot.Date)] = len(snapshots)
		dayEntities[dayRows[i].ID] = len(snapshots)
		dayIDs = append(dayIDs, dayRows[i].ID)
		snapshots = append(snapshots, snapshot)
	}

	rollingEntities, err := r.attachRollingSummaries(ctx, query, planID, basis, byKey, snapshots)
	if err != nil {
		return nil, err
	}
	if err := r.attachRuleResults(ctx, query, planningdom.RuleEntityDay, dayIDs, dayEntities, snapshots, true); err != nil {
		return nil, err
	}
	rollingIDs := make([]int64, 0, len(rollingEntities))
	for id := range rollingEntities {
		rollingIDs = append(rollingIDs, id)
	}
	sort.Slice(rollingIDs, func(i, j int) bool { return rollingIDs[i] < rollingIDs[j] })
	if err := r.attachRuleResults(ctx, query, planningdom.RuleEntityRollingWindow, rollingIDs, rollingEntities, snapshots, false); err != nil {
		return nil, err
	}
	return snapshots, nil
}

func (r *Repository) attachRollingSummaries(
	ctx context.Context,
	query *sqlc.Queries,
	planID planningdom.PlanID,
	basis nutrition.SummaryBasis,
	byKey map[string]int,
	snapshots []planningdom.QualitySnapshot,
) (map[int64]int, error) {
	rows, err := query.ListPlanRollingSummaries(ctx, sqlc.ListPlanRollingSummariesParams{
		PlanID: int64(planID),
		Basis:  sqlc.SummaryBasis(basis),
	})
	if err != nil {
		return nil, dberror.Map(err, "rolling nutrition summary")
	}
	entities := make(map[int64]int, len(rows))
	for i := range rows {
		position, ok := byKey[snapshotKey(eaterdom.ID(rows[i].EaterID), rows[i].AsOfDate.Time)]
		if !ok {
			continue
		}
		rolling, err := rollingSummaryFromRow(rows[i])
		if err != nil {
			return nil, err
		}
		snapshots[position].Rolling = rolling
		entities[rows[i].ID] = position
	}
	return entities, nil
}

func (r *Repository) attachRuleResults(
	ctx context.Context,
	query *sqlc.Queries,
	entityType planningdom.RuleEntityType,
	entityIDs []int64,
	entities map[int64]int,
	snapshots []planningdom.QualitySnapshot,
	daily bool,
) error {
	if len(entityIDs) == 0 {
		return nil
	}
	rows, err := query.ListRuleEvaluationResults(ctx, sqlc.ListRuleEvaluationResultsParams{
		EntityType: sqlc.RuleEntityType(entityType),
		EntityIds:  entityIDs,
	})
	if err != nil {
		return dberror.Map(err, "rule evaluation result")
	}
	for i := range rows {
		position, ok := entities[rows[i].EntityID]
		if !ok {
			continue
		}
		result, err := ruleResultFromRow(rows[i])
		if err != nil {
			return err
		}
		if daily {
			snapshots[position].DayRules = append(snapshots[position].DayRules, result)
			continue
		}
		snapshots[position].RollingRules = append(snapshots[position].RollingRules, result)
	}
	return nil
}

func (r *Repository) List(ctx context.Context) ([]planningdom.Plan, error) {
	rows, err := r.store.Queries(ctx).ListPlans(ctx)
	if err != nil {
		return nil, dberror.Map(err, "plan")
	}
	plans := make([]planningdom.Plan, 0, len(rows))
	for i := range rows {
		plan, err := planFromRow(rows[i])
		if err != nil {
			return nil, err
		}
		plans = append(plans, plan)
	}
	return plans, nil
}

// ListTemplateLastPlanned returns, per template, the start date of the latest
// plan that began before the given day.
func (r *Repository) ListTemplateLastPlanned(
	ctx context.Context,
	before time.Time,
) (map[recipedom.ID]time.Time, error) {
	rows, err := r.store.Queries(ctx).ListTemplateLastPlanned(ctx, pgvalue.Date(before))
	if err != nil {
		return nil, dberror.Map(err, "plan recipe")
	}
	result := make(map[recipedom.ID]time.Time, len(rows))
	for i := range rows {
		if !rows[i].LastPlanned.Valid {
			continue
		}
		result[recipedom.ID(rows[i].TemplateID)] = rows[i].LastPlanned.Time
	}
	return result, nil
}

func (r *Repository) Archive(ctx context.Context, id planningdom.PlanID) error {
	if _, err := r.store.Queries(ctx).ArchivePlan(ctx, int64(id)); err != nil {
		return dberror.Map(err, "plan")
	}
	return nil
}

// SwapSlotFill changes the product of one stored fill and marks the shopping
// list stale, because the plan now buys something else.
func (r *Repository) SwapSlotFill(
	ctx context.Context,
	planID planningdom.PlanID,
	fillID planningdom.SlotFillID,
	productID catalogdom.ProductID,
) error {
	query := r.store.Queries(ctx)
	if _, err := query.UpdateSlotFillProduct(ctx, sqlc.UpdateSlotFillProductParams{
		ID:        int64(fillID),
		ProductID: int64(productID),
	}); err != nil {
		return dberror.Map(err, "slot fill")
	}
	if _, err := query.MarkShoppingListStale(ctx, sqlc.MarkShoppingListStaleParams{
		ID:    int64(planID),
		Stale: true,
	}); err != nil {
		return dberror.Map(err, "plan")
	}
	return nil
}

// UpdateMealMacros stores the planned macros of every generated meal. A swap
// changes what a batch contains, so the meals served from it must follow.
func (r *Repository) UpdateMealMacros(ctx context.Context, plan planningdom.Plan) error {
	query := r.store.Queries(ctx)
	for i := range plan.Meals {
		if plan.Meals[i].FixedMealID != nil {
			continue
		}
		if _, err := query.UpdateMealPlannedMacros(ctx, mealMacroParams(plan.Meals[i])); err != nil {
			return dberror.Map(err, "meal")
		}
	}
	return nil
}

// UpdatePlanQuality refreshes the plan headline after its content changed.
func (r *Repository) UpdatePlanQuality(
	ctx context.Context,
	planID planningdom.PlanID,
	score *float64,
	warnings int,
) error {
	if warnings < 0 || int64(warnings) > math.MaxInt32 {
		return apperr.Validation("quality warnings count must fit a nonnegative int32")
	}
	_, err := r.store.Queries(ctx).UpdatePlanQuality(ctx, sqlc.UpdatePlanQualityParams{
		ID:                   int64(planID),
		QualityScore:         pgvalue.OptionalNumeric(score),
		QualityWarningsCount: int32(warnings),
	})
	if err != nil {
		return dberror.Map(err, "plan")
	}
	return nil
}

// ReplacePlanRecipe rewrites one batch in place after a recipe-level swap: its
// template, scale and fills, the serving hints of the meals that eat from it,
// every generated meal's planned macros and the prep graph. Meal rows keep
// their ids, so anything already logged against them survives.
func (r *Repository) ReplacePlanRecipe(
	ctx context.Context,
	planID planningdom.PlanID,
	plan planningdom.Plan,
	recipeIndex int,
) error {
	if recipeIndex < 0 || recipeIndex >= len(plan.Recipes) {
		return apperr.Validation("swapped plan recipe is outside the plan")
	}
	query := r.store.Queries(ctx)
	if err := replaceBatch(ctx, query, plan.Recipes[recipeIndex]); err != nil {
		return err
	}
	if err := r.UpdateMealMacros(ctx, plan); err != nil {
		return err
	}
	if err := replaceBatchAllocations(ctx, query, plan, recipeIndex); err != nil {
		return err
	}

	if err := query.DeletePlanPrepDependencies(ctx, int64(planID)); err != nil {
		return dberror.Map(err, "prep dependency")
	}
	recipeIDs := make([]planningdom.PlanRecipeID, 0, len(plan.Recipes))
	for i := range plan.Recipes {
		recipeIDs = append(recipeIDs, plan.Recipes[i].ID)
	}
	if err := r.savePrepDependencies(ctx, query, planID, plan.PrepDependencies, recipeIDs); err != nil {
		return err
	}
	return r.MarkShoppingListStale(ctx, planID, true)
}

// replaceBatch points the stored batch at its new template and swaps its fills.
func replaceBatch(ctx context.Context, query *sqlc.Queries, recipe planningdom.PlanRecipe) error {
	if _, err := query.UpdatePlanRecipeTemplate(ctx, sqlc.UpdatePlanRecipeTemplateParams{
		ID:              int64(recipe.ID),
		TemplateID:      int64(recipe.TemplateID),
		ScaleFactor:     pgvalue.Numeric(recipe.ScaleFactor),
		ExpectedOutputG: pgvalue.OptionalNumeric(recipe.ExpectedOutputG),
	}); err != nil {
		return dberror.Map(err, "plan recipe")
	}
	if err := query.DeletePlanRecipeSlotFills(ctx, int64(recipe.ID)); err != nil {
		return dberror.Map(err, "slot fill")
	}
	for _, fill := range recipe.Fills {
		if _, err := query.CreateSlotFill(ctx, slotFillParams(recipe.ID, fill)); err != nil {
			return dberror.Map(err, "slot fill")
		}
	}
	return nil
}

// replaceBatchAllocations rewrites the allocations of the meals that eat from
// the swapped batch: their serving hints follow the new expected output.
func replaceBatchAllocations(
	ctx context.Context,
	query *sqlc.Queries,
	plan planningdom.Plan,
	recipeIndex int,
) error {
	for i := range plan.Meals {
		meal := &plan.Meals[i]
		if !mealUsesRecipe(*meal, recipeIndex) {
			continue
		}
		if err := query.DeleteMealComponentAllocations(ctx, int64(meal.ID)); err != nil {
			return dberror.Map(err, "meal component allocation")
		}
		for _, component := range meal.Components {
			if component.PlanRecipeIndex >= len(plan.Recipes) {
				return apperr.Conflict("allocation points to a plan recipe outside the plan")
			}
			params := allocationParams(meal.ID, plan.Recipes[component.PlanRecipeIndex].ID, component)
			if _, err := query.CreateMealComponentAllocation(ctx, params); err != nil {
				return dberror.Map(err, "meal component allocation")
			}
		}
	}
	return nil
}

func mealUsesRecipe(meal planningdom.Meal, recipeIndex int) bool {
	for _, component := range meal.Components {
		if component.PlanRecipeIndex == recipeIndex {
			return true
		}
	}
	return false
}

// MarkShoppingListStale records whether the plan still matches its list.
func (r *Repository) MarkShoppingListStale(ctx context.Context, planID planningdom.PlanID, stale bool) error {
	_, err := r.store.Queries(ctx).MarkShoppingListStale(ctx, sqlc.MarkShoppingListStaleParams{
		ID:    int64(planID),
		Stale: stale,
	})
	if err != nil {
		return dberror.Map(err, "plan")
	}
	return nil
}

// ReplacePlanAllocations rewrites the parts a quantity adjustment moves: the
// slot fill quantities and the meal component allocations. It runs on the
// caller's executor, so the whole adjustment commits or rolls back together.
func (r *Repository) ReplacePlanAllocations(
	ctx context.Context,
	planID planningdom.PlanID,
	plan planningdom.Plan,
) (planningdom.Plan, error) {
	query := r.store.Queries(ctx)
	for i := range plan.Recipes {
		for _, fill := range plan.Recipes[i].Fills {
			if _, err := query.UpdateSlotFillQuantity(ctx, sqlc.UpdateSlotFillQuantityParams{
				ID:               int64(fill.ID),
				PlannedInputQtyG: pgvalue.Numeric(fill.PlannedInputQtyG),
			}); err != nil {
				return planningdom.Plan{}, dberror.Map(err, "slot fill")
			}
		}
	}

	for i := range plan.Meals {
		meal := &plan.Meals[i]
		if meal.FixedMealID != nil {
			continue
		}
		if _, err := query.UpdateMealPlannedMacros(ctx, mealMacroParams(*meal)); err != nil {
			return planningdom.Plan{}, dberror.Map(err, "meal")
		}
		// (meal_id, plan_recipe_id) is the allocation primary key, so the old
		// rows have to go before the rebalanced ones are written.
		if err := query.DeleteMealComponentAllocations(ctx, int64(meal.ID)); err != nil {
			return planningdom.Plan{}, dberror.Map(err, "meal component allocation")
		}
		for _, component := range meal.Components {
			if component.PlanRecipeIndex >= len(plan.Recipes) {
				return planningdom.Plan{}, apperr.Conflict("allocation points to a plan recipe outside the plan")
			}
			params := allocationParams(meal.ID, plan.Recipes[component.PlanRecipeIndex].ID, component)
			if _, err := query.CreateMealComponentAllocation(ctx, params); err != nil {
				return planningdom.Plan{}, dberror.Map(err, "meal component allocation")
			}
		}
	}
	return r.Get(ctx, planID)
}

// ListEaterDaySummaries reads the stored day summaries of one eater in a date
// range, across plans: a rolling window reaches back past the plan being built.
func (r *Repository) ListEaterDaySummaries(
	ctx context.Context,
	eaterID eaterdom.ID,
	basis nutrition.SummaryBasis,
	from, to time.Time,
) ([]planningdom.QualitySnapshot, error) {
	rows, err := r.store.Queries(ctx).ListEaterDaySummariesForPeriod(ctx, sqlc.ListEaterDaySummariesForPeriodParams{
		EaterID:  int64(eaterID),
		Basis:    sqlc.SummaryBasis(basis),
		FromDate: pgvalue.Date(from),
		ToDate:   pgvalue.Date(to),
	})
	if err != nil {
		return nil, dberror.Map(err, "day nutrition summary")
	}
	summaries := make([]planningdom.QualitySnapshot, 0, len(rows))
	for i := range rows {
		summary, err := daySummaryFromRow(rows[i])
		if err != nil {
			return nil, err
		}
		summaries = append(summaries, summary)
	}
	return summaries, nil
}

// ListPlanMeals returns the meals of a plan, optionally for one eater.
func (r *Repository) ListPlanMeals(
	ctx context.Context,
	planID planningdom.PlanID,
	eaterID *eaterdom.ID,
) ([]planningdom.Meal, error) {
	var filter *int64
	if eaterID != nil {
		id := int64(*eaterID)
		filter = &id
	}
	rows, err := r.store.Queries(ctx).ListPlanMealsForEater(ctx, sqlc.ListPlanMealsForEaterParams{
		PlanID:  int64(planID),
		EaterID: filter,
	})
	if err != nil {
		return nil, dberror.Map(err, "meal")
	}
	return mealsFromRows(rows)
}

// ListEaterMeals returns the meals of one eater inside a period, one plan per
// day: the plan the day was logged in, otherwise the latest one not archived.
func (r *Repository) ListEaterMeals(
	ctx context.Context,
	eaterID eaterdom.ID,
	from, to time.Time,
) ([]planningdom.Meal, error) {
	rows, err := r.store.Queries(ctx).ListMealsForEaterPeriod(ctx, sqlc.ListMealsForEaterPeriodParams{
		EaterID:  int64(eaterID),
		FromDate: pgvalue.Date(from),
		ToDate:   pgvalue.Date(to),
	})
	if err != nil {
		return nil, dberror.Map(err, "meal")
	}
	return mealsFromRows(rows)
}

func (r *Repository) GetMeal(ctx context.Context, mealID planningdom.MealID) (planningdom.Meal, error) {
	row, err := r.store.Queries(ctx).GetMeal(ctx, int64(mealID))
	if errors.Is(err, pgx.ErrNoRows) {
		return planningdom.Meal{}, apperr.NotFound("meal")
	}
	if err != nil {
		return planningdom.Meal{}, dberror.Map(err, "meal")
	}
	return mealFromRow(row)
}

// LogMeal records what actually happened to one meal.
func (r *Repository) LogMeal(
	ctx context.Context,
	mealID planningdom.MealID,
	log planningdom.MealLog,
) (planningdom.Meal, error) {
	row, err := r.store.Queries(ctx).LogMeal(ctx, logMealParams(mealID, log))
	if errors.Is(err, pgx.ErrNoRows) {
		return planningdom.Meal{}, apperr.NotFound("meal")
	}
	if err != nil {
		return planningdom.Meal{}, dberror.Map(err, "meal")
	}
	return mealFromRow(row)
}

// ListPlansForEaterPeriod finds the plans that cover an eater's dates, so
// analytics can rebuild actual summaries from real plan data.
func (r *Repository) ListPlansForEaterPeriod(
	ctx context.Context,
	eaterID eaterdom.ID,
	from, to time.Time,
) ([]planningdom.Plan, error) {
	rows, err := r.store.Queries(ctx).ListPlansForEaterPeriod(ctx, sqlc.ListPlansForEaterPeriodParams{
		EaterID:  int64(eaterID),
		FromDate: pgvalue.Date(from),
		ToDate:   pgvalue.Date(to),
	})
	if err != nil {
		return nil, dberror.Map(err, "plan")
	}
	plans := make([]planningdom.Plan, 0, len(rows))
	for i := range rows {
		plan, err := r.Get(ctx, planningdom.PlanID(rows[i].ID))
		if err != nil {
			return nil, err
		}
		plans = append(plans, plan)
	}
	return plans, nil
}

func mealsFromRows(rows []sqlc.Meal) ([]planningdom.Meal, error) {
	meals := make([]planningdom.Meal, 0, len(rows))
	for i := range rows {
		meal, err := mealFromRow(rows[i])
		if err != nil {
			return nil, err
		}
		meals = append(meals, meal)
	}
	return meals, nil
}
