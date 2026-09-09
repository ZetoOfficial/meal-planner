package planning

import (
	"context"
	"sort"
	"strconv"

	eaterdom "github.com/ZetoOfficial/meal-planner/internal/domain/eater"
	"github.com/ZetoOfficial/meal-planner/internal/domain/nutrition"
	planningdom "github.com/ZetoOfficial/meal-planner/internal/domain/planning"
	apperr "github.com/ZetoOfficial/meal-planner/internal/errors"
	"github.com/ZetoOfficial/meal-planner/internal/postgres"
	"github.com/ZetoOfficial/meal-planner/internal/postgres/dberror"
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
