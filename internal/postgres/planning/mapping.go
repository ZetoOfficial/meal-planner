package planning

import (
	"math"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	catalogdom "github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
	eaterdom "github.com/ZetoOfficial/meal-planner/internal/domain/eater"
	mealdom "github.com/ZetoOfficial/meal-planner/internal/domain/meal"
	"github.com/ZetoOfficial/meal-planner/internal/domain/nutrition"
	planningdom "github.com/ZetoOfficial/meal-planner/internal/domain/planning"
	recipedom "github.com/ZetoOfficial/meal-planner/internal/domain/recipe"
	"github.com/ZetoOfficial/meal-planner/internal/postgres/pgvalue"
	"github.com/ZetoOfficial/meal-planner/internal/postgres/sqlc"
)

func planParams(plan planningdom.Plan) sqlc.CreatePlanParams {
	days := plan.Days
	warnings := plan.QualityWarningsCount
	if days < 0 || int64(days) > math.MaxInt32 ||
		warnings < 0 || int64(warnings) > math.MaxInt32 ||
		plan.Novelty < 0 || plan.Novelty > math.MaxInt16 {
		panic("plan days, novelty and warning count must fit the stored integer widths")
	}
	return sqlc.CreatePlanParams{
		Label:                plan.Label,
		StartDate:            dateValue(plan.StartDate),
		Days:                 int32(days),
		CookDate:             dateValue(plan.CookDate),
		NoveltyLevel:         int16(plan.Novelty),
		Status:               sqlc.PlanStatus(statusOrDraft(plan.Status)),
		QualityScore:         pgvalue.OptionalNumeric(plan.QualityScore),
		QualityWarningsCount: int32(warnings),
		ShoppingListStale:    plan.ShoppingListStale,
	}
}

func eaterDayParams(planID planningdom.PlanID, day planningdom.EaterDay) sqlc.CreatePlanEaterDayParams {
	types := make([]string, 0, len(day.GeneratedMealTypes))
	for _, mealType := range day.GeneratedMealTypes {
		types = append(types, string(mealType))
	}
	return sqlc.CreatePlanEaterDayParams{
		PlanID:             int64(planID),
		EaterID:            int64(day.EaterID),
		PlanDate:           dateValue(day.Date),
		MacroGoalID:        int64(day.MacroGoalID),
		GeneratedMealTypes: types,
	}
}

func planRecipeParams(planID planningdom.PlanID, recipe planningdom.PlanRecipe) sqlc.CreatePlanRecipeParams {
	return sqlc.CreatePlanRecipeParams{
		PlanID:          int64(planID),
		TemplateID:      int64(recipe.TemplateID),
		Label:           recipe.Label,
		ScaleFactor:     pgvalue.Numeric(recipe.ScaleFactor),
		ExpectedOutputG: pgvalue.OptionalNumeric(recipe.ExpectedOutputG),
	}
}

func slotFillParams(planRecipeID planningdom.PlanRecipeID, fill planningdom.SlotFill) sqlc.CreateSlotFillParams {
	return sqlc.CreateSlotFillParams{
		PlanRecipeID:     int64(planRecipeID),
		SlotID:           int64(fill.SlotID),
		ProductID:        int64(fill.ProductID),
		PlannedInputQtyG: pgvalue.Numeric(fill.PlannedInputQtyG),
	}
}

func mealParams(planID planningdom.PlanID, meal planningdom.Meal) sqlc.CreateMealParams {
	kcal := meal.Planned.Kcal
	if kcal < 0 || int64(kcal) > math.MaxInt32 {
		panic("planned meal kcal must be a nonnegative int32")
	}
	var fixedMealID *int64
	if meal.FixedMealID != nil {
		id := int64(*meal.FixedMealID)
		fixedMealID = &id
	}
	return sqlc.CreateMealParams{
		PlanID:          int64(planID),
		EaterID:         int64(meal.EaterID),
		MealDate:        dateValue(meal.Date),
		MealType:        sqlc.MealType(meal.Type),
		FixedMealID:     fixedMealID,
		PlannedProteinG: pgvalue.Numeric(meal.Planned.ProteinG),
		PlannedFatG:     pgvalue.Numeric(meal.Planned.FatG),
		PlannedCarbG:    pgvalue.Numeric(meal.Planned.CarbG),
		PlannedKcal:     int32(kcal),
		LogStatus:       sqlc.MealLogStatus(logStatusOrPlanned(meal.LogStatus)),
	}
}

func allocationParams(
	mealID planningdom.MealID,
	planRecipeID planningdom.PlanRecipeID,
	component planningdom.ComponentAllocation,
) sqlc.CreateMealComponentAllocationParams {
	return sqlc.CreateMealComponentAllocationParams{
		MealID:          int64(mealID),
		PlanRecipeID:    int64(planRecipeID),
		PortionFactor:   pgvalue.Numeric(component.PortionFactor),
		PlannedServingG: pgvalue.OptionalNumeric(component.PlannedServingG),
	}
}

func planFromRow(row sqlc.Plan) (planningdom.Plan, error) {
	score, err := pgvalue.OptionalFloat64(row.QualityScore)
	if err != nil {
		return planningdom.Plan{}, err
	}
	return planningdom.Plan{
		ID:                   planningdom.PlanID(row.ID),
		Label:                row.Label,
		Status:               planningdom.PlanStatus(row.Status),
		StartDate:            row.StartDate.Time,
		Days:                 int(row.Days),
		CookDate:             row.CookDate.Time,
		Novelty:              int(row.NoveltyLevel),
		QualityScore:         score,
		QualityWarningsCount: int(row.QualityWarningsCount),
		ShoppingListStale:    row.ShoppingListStale,
	}, nil
}

func eaterDayFromRow(row sqlc.PlanEaterDay) planningdom.EaterDay {
	types := make([]mealdom.Type, 0, len(row.GeneratedMealTypes))
	for _, mealType := range row.GeneratedMealTypes {
		types = append(types, mealdom.Type(mealType))
	}
	return planningdom.EaterDay{
		EaterID:            eaterdom.ID(row.EaterID),
		Date:               row.PlanDate.Time,
		MacroGoalID:        eaterdom.MacroGoalID(row.MacroGoalID),
		GeneratedMealTypes: types,
	}
}

func planRecipeFromRow(row sqlc.PlanRecipe) (planningdom.PlanRecipe, error) {
	scaleFactor, err := pgvalue.Float64(row.ScaleFactor)
	if err != nil {
		return planningdom.PlanRecipe{}, err
	}
	expectedOutput, err := pgvalue.OptionalFloat64(row.ExpectedOutputG)
	if err != nil {
		return planningdom.PlanRecipe{}, err
	}
	return planningdom.PlanRecipe{
		ID:              planningdom.PlanRecipeID(row.ID),
		Label:           row.Label,
		TemplateID:      recipedom.ID(row.TemplateID),
		ScaleFactor:     scaleFactor,
		ExpectedOutputG: expectedOutput,
	}, nil
}

func slotFillFromRow(row sqlc.SlotFill) (planningdom.SlotFill, error) {
	qtyG, err := pgvalue.Float64(row.PlannedInputQtyG)
	if err != nil {
		return planningdom.SlotFill{}, err
	}
	return planningdom.SlotFill{
		ID:               planningdom.SlotFillID(row.ID),
		SlotID:           recipedom.SlotID(row.SlotID),
		ProductID:        catalogdom.ProductID(row.ProductID),
		PlannedInputQtyG: qtyG,
	}, nil
}

func mealFromRow(row sqlc.Meal) (planningdom.Meal, error) {
	proteinG, err := pgvalue.Float64(row.PlannedProteinG)
	if err != nil {
		return planningdom.Meal{}, err
	}
	fatG, err := pgvalue.Float64(row.PlannedFatG)
	if err != nil {
		return planningdom.Meal{}, err
	}
	carbG, err := pgvalue.Float64(row.PlannedCarbG)
	if err != nil {
		return planningdom.Meal{}, err
	}

	var fixedMealID *eaterdom.FixedMealID
	if row.FixedMealID != nil {
		id := eaterdom.FixedMealID(*row.FixedMealID)
		fixedMealID = &id
	}
	actual, err := actualMacros(row)
	if err != nil {
		return planningdom.Meal{}, err
	}
	return planningdom.Meal{
		ID:          planningdom.MealID(row.ID),
		PlanID:      planningdom.PlanID(row.PlanID),
		LogStatus:   planningdom.MealLogStatus(row.LogStatus),
		EaterID:     eaterdom.ID(row.EaterID),
		Date:        row.MealDate.Time,
		Type:        mealdom.Type(row.MealType),
		FixedMealID: fixedMealID,
		Planned: nutrition.Macros{
			ProteinG: proteinG,
			FatG:     fatG,
			CarbG:    carbG,
			Kcal:     int(row.PlannedKcal),
		},
		Actual:   actual,
		CookedBy: optionalEaterID(row.CookedBy),
		CookedAt: optionalTime(row.CookedAt),
		LoggedBy: optionalEaterID(row.LoggedBy),
		LoggedAt: optionalTime(row.LoggedAt),
	}, nil
}

// actualMacros is present only when the meal was logged as eaten differently;
// a partially filled row would be a fiction, so all four values must be there.
func actualMacros(row sqlc.Meal) (*nutrition.Macros, error) {
	if row.ActualKcal == nil {
		return nil, nil
	}
	proteinG, err := pgvalue.OptionalFloat64(row.ActualProteinG)
	if err != nil {
		return nil, err
	}
	fatG, err := pgvalue.OptionalFloat64(row.ActualFatG)
	if err != nil {
		return nil, err
	}
	carbG, err := pgvalue.OptionalFloat64(row.ActualCarbG)
	if err != nil {
		return nil, err
	}
	if proteinG == nil || fatG == nil || carbG == nil {
		return nil, nil
	}
	return &nutrition.Macros{
		ProteinG: *proteinG,
		FatG:     *fatG,
		CarbG:    *carbG,
		Kcal:     int(*row.ActualKcal),
	}, nil
}

func optionalEaterID(value *int64) *eaterdom.ID {
	if value == nil {
		return nil
	}
	id := eaterdom.ID(*value)
	return &id
}

func optionalTime(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	moment := value.Time
	return &moment
}

func logMealParams(mealID planningdom.MealID, log planningdom.MealLog) sqlc.LogMealParams {
	params := sqlc.LogMealParams{
		ID:        int64(mealID),
		LogStatus: sqlc.MealLogStatus(log.Status),
		LoggedBy:  int64Pointer(int64(log.LoggedBy)),
		LoggedAt:  pgtype.Timestamptz{Time: log.At, Valid: true},
	}
	if log.CookedBy != nil {
		params.CookedBy = int64Pointer(int64(*log.CookedBy))
	}
	if log.Actual != nil {
		kcal := log.Actual.Kcal
		if kcal < 0 || int64(kcal) > math.MaxInt32 {
			panic("actual meal kcal must be a nonnegative int32")
		}
		converted := int32(kcal)
		params.ActualProteinG = pgvalue.Numeric(log.Actual.ProteinG)
		params.ActualFatG = pgvalue.Numeric(log.Actual.FatG)
		params.ActualCarbG = pgvalue.Numeric(log.Actual.CarbG)
		params.ActualKcal = &converted
	}
	return params
}

func int64Pointer(value int64) *int64 { return &value }

func allocationFromRow(row sqlc.MealComponentAllocation, planRecipeIndex int) (planningdom.ComponentAllocation, error) {
	portionFactor, err := pgvalue.Float64(row.PortionFactor)
	if err != nil {
		return planningdom.ComponentAllocation{}, err
	}
	servingG, err := pgvalue.OptionalFloat64(row.PlannedServingG)
	if err != nil {
		return planningdom.ComponentAllocation{}, err
	}
	return planningdom.ComponentAllocation{
		PlanRecipeIndex: planRecipeIndex,
		PortionFactor:   portionFactor,
		PlannedServingG: servingG,
	}, nil
}

const dayDuration = 24 * time.Hour

func mealMacroParams(meal planningdom.Meal) sqlc.UpdateMealPlannedMacrosParams {
	kcal := meal.Planned.Kcal
	if kcal < 0 || int64(kcal) > math.MaxInt32 {
		panic("planned meal kcal must be a nonnegative int32")
	}
	return sqlc.UpdateMealPlannedMacrosParams{
		ID:              int64(meal.ID),
		PlannedProteinG: pgvalue.Numeric(meal.Planned.ProteinG),
		PlannedFatG:     pgvalue.Numeric(meal.Planned.FatG),
		PlannedCarbG:    pgvalue.Numeric(meal.Planned.CarbG),
		PlannedKcal:     int32(kcal),
	}
}

func prepDependencyParams(planID planningdom.PlanID, dependency planningdom.PrepDependency) sqlc.CreatePrepDependencyParams {
	return sqlc.CreatePrepDependencyParams{
		PlanID:            int64(planID),
		DependentRecipeID: int64(dependency.DependentRecipeID),
		SourceRecipeID:    int64(dependency.SourceRecipeID),
		Note:              dependency.Note,
	}
}

func prepDependencyFromRow(row sqlc.PrepDependency) planningdom.PrepDependency {
	return planningdom.PrepDependency{
		ID:                row.ID,
		DependentRecipeID: planningdom.PlanRecipeID(row.DependentRecipeID),
		SourceRecipeID:    planningdom.PlanRecipeID(row.SourceRecipeID),
		Note:              row.Note,
	}
}

func dateValue(value time.Time) pgtype.Date {
	return pgtype.Date{Time: value.UTC().Truncate(dayDuration), Valid: true}
}

func statusOrDraft(status planningdom.PlanStatus) planningdom.PlanStatus {
	if status.Valid() {
		return status
	}
	return planningdom.PlanStatusDraft
}

func logStatusOrPlanned(status planningdom.MealLogStatus) planningdom.MealLogStatus {
	if status.Valid() {
		return status
	}
	return planningdom.MealLogStatusPlanned
}
