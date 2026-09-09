package httpapi

import (
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/ZetoOfficial/meal-planner/api/sdk/go/model"
	eaterdom "github.com/ZetoOfficial/meal-planner/internal/domain/eater"
	mealdom "github.com/ZetoOfficial/meal-planner/internal/domain/meal"
	"github.com/ZetoOfficial/meal-planner/internal/domain/nutrition"
	planningdom "github.com/ZetoOfficial/meal-planner/internal/domain/planning"
	"github.com/ZetoOfficial/meal-planner/internal/solver"
	usecaseplanning "github.com/ZetoOfficial/meal-planner/internal/usecase/planning"
)

func generatePlanCommand(body model.PlanGenerateInput) usecaseplanning.GeneratePlanCommand {
	command := usecaseplanning.GeneratePlanCommand{
		Label:     body.Label,
		StartDate: body.StartDate.Time,
		Targets:   make([]usecaseplanning.EaterTarget, 0, len(body.Targets)),
	}
	if body.Days != nil {
		command.Days = *body.Days
	}
	if body.NoveltyLevel != nil {
		command.Novelty = *body.NoveltyLevel
	}
	if body.CookDate != nil {
		cookDate := body.CookDate.Time
		command.CookDate = &cookDate
	}
	for _, target := range body.Targets {
		command.Targets = append(command.Targets, usecaseplanning.EaterTarget{
			EaterID:            eaterdom.ID(target.EaterId),
			GeneratedMealTypes: mealTypesFromAPI(target.GeneratedMealTypes),
		})
	}
	return command
}

func mealTypesFromAPI(types []model.MealType) []mealdom.Type {
	result := make([]mealdom.Type, 0, len(types))
	for _, mealType := range types {
		result = append(result, mealdom.Type(mealType))
	}
	return result
}

func planSummaryResponse(plan planningdom.Plan) model.PlanSummary {
	return model.PlanSummary{
		Id:                   int64(plan.ID),
		Label:                plan.Label,
		StartDate:            openapiDate(plan.StartDate),
		Days:                 plan.Days,
		Status:               model.PlanSummaryStatus(plan.Status),
		QualityScore:         float32Pointer(plan.QualityScore),
		QualityWarningsCount: plan.QualityWarningsCount,
		ShoppingListStale:    plan.ShoppingListStale,
	}
}

func planSummariesResponse(plans []planningdom.Plan) []model.PlanSummary {
	result := make([]model.PlanSummary, 0, len(plans))
	for i := range plans {
		result = append(result, planSummaryResponse(plans[i]))
	}
	return result
}

func planResponse(result usecaseplanning.PlanResult) model.Plan {
	plan := result.Plan
	response := model.Plan{
		Id:                int64(plan.ID),
		Label:             plan.Label,
		StartDate:         openapiDate(plan.StartDate),
		Days:              plan.Days,
		CookDate:          openapiDate(plan.CookDate),
		NoveltyLevel:      plan.Novelty,
		Status:            model.PlanStatus(plan.Status),
		EaterDays:         planEaterDaysResponse(plan.EaterDays),
		Recipes:           planRecipesResponse(plan),
		Meals:             planMealsResponse(plan),
		PrepDependencies:  []model.PrepDependency{},
		ShoppingListStale: plan.ShoppingListStale,
	}
	if summary := qualitySummaryResponse(plan, result.Snapshots); summary != nil {
		response.QualitySummary = summary
	}
	return response
}

func planEaterDaysResponse(days []planningdom.EaterDay) []model.PlanEaterDay {
	result := make([]model.PlanEaterDay, 0, len(days))
	for _, day := range days {
		types := make([]model.MealType, 0, len(day.GeneratedMealTypes))
		for _, mealType := range day.GeneratedMealTypes {
			types = append(types, model.MealType(mealType))
		}
		result = append(result, model.PlanEaterDay{
			EaterId:            int64(day.EaterID),
			Date:               openapiDate(day.Date),
			MacroGoalId:        int64(day.MacroGoalID),
			GeneratedMealTypes: types,
		})
	}
	return result
}

func planRecipesResponse(plan planningdom.Plan) []model.PlanRecipe {
	allocated := allocatedPortions(plan)
	result := make([]model.PlanRecipe, 0, len(plan.Recipes))
	for i, recipe := range plan.Recipes {
		fills := make([]model.SlotFill, 0, len(recipe.Fills))
		for _, fill := range recipe.Fills {
			fills = append(fills, model.SlotFill{
				SlotId:           int64(fill.SlotID),
				ProductId:        int64(fill.ProductID),
				PlannedInputQtyG: float32(fill.PlannedInputQtyG),
			})
		}
		result = append(result, model.PlanRecipe{
			Id:              int64(recipe.ID),
			TemplateId:      int64(recipe.TemplateID),
			Label:           recipe.Label,
			ScaleFactor:     float32(recipe.ScaleFactor),
			ExpectedOutputG: float32Pointer(recipe.ExpectedOutputG),
			ReserveFactor:   floatValuePointer(max(0, 1-allocated[i])),
			Fills:           fills,
		})
	}
	return result
}

// allocatedPortions sums the batch shares handed to meals so the response can
// report the reserve that stays uneaten.
func allocatedPortions(plan planningdom.Plan) []float64 {
	portions := make([]float64, len(plan.Recipes))
	for i := range plan.Meals {
		for _, component := range plan.Meals[i].Components {
			if component.PlanRecipeIndex >= 0 && component.PlanRecipeIndex < len(portions) {
				portions[component.PlanRecipeIndex] += component.PortionFactor
			}
		}
	}
	return portions
}

func planMealsResponse(plan planningdom.Plan) []model.Meal {
	result := make([]model.Meal, 0, len(plan.Meals))
	for i := range plan.Meals {
		meal := &plan.Meals[i]
		components := make([]model.MealComponentAllocation, 0, len(meal.Components))
		for _, component := range meal.Components {
			allocation := model.MealComponentAllocation{PortionFactor: float32(component.PortionFactor)}
			if component.PlanRecipeIndex >= 0 && component.PlanRecipeIndex < len(plan.Recipes) {
				allocation.PlanRecipeId = int64(plan.Recipes[component.PlanRecipeIndex].ID)
			}
			allocation.PlannedServingG = float32Pointer(component.PlannedServingG)
			components = append(components, allocation)
		}

		var fixedMealID *int64
		if meal.FixedMealID != nil {
			id := int64(*meal.FixedMealID)
			fixedMealID = &id
		}
		result = append(result, model.Meal{
			Id:          int64(meal.ID),
			PlanId:      int64(plan.ID),
			EaterId:     int64(meal.EaterID),
			MealDate:    openapiDate(meal.Date),
			MealType:    model.MealType(meal.Type),
			FixedMealId: fixedMealID,
			Components:  components,
			Planned:     macrosResponse(meal.Planned),
			LogStatus:   model.MealLogStatus(logStatusOrPlanned(meal.LogStatus)),
		})
	}
	return result
}

func macrosResponse(macros nutrition.Macros) model.Macros {
	return model.Macros{
		ProteinG: float32(macros.ProteinG),
		FatG:     float32(macros.FatG),
		CarbG:    float32(macros.CarbG),
		Kcal:     macros.Kcal,
	}
}

// qualitySummaryResponse folds the stored per eater-day snapshots into the
// plan-level quality view, keeping every rule verdict for explainability.
func qualitySummaryResponse(
	plan planningdom.Plan,
	snapshots []planningdom.QualitySnapshot,
) *model.NutritionQualitySummary {
	if len(snapshots) == 0 {
		return nil
	}

	summary := model.NutritionQualitySummary{
		WarningsCount:   plan.QualityWarningsCount,
		RuleEvaluations: make([]model.RuleEvaluation, 0, len(snapshots)),
	}
	if plan.QualityScore != nil {
		summary.QualityScore = float32(*plan.QualityScore)
	}

	var coverage float64
	for i := range snapshots {
		snapshot := &snapshots[i]
		coverage += snapshot.Day.Coverage.Score()
		addSnapshotTotals(&summary, *snapshot)
		for _, rule := range snapshot.DayRules {
			summary.RuleEvaluations = append(summary.RuleEvaluations, ruleEvaluationResponse(*snapshot, rule, false))
		}
		for _, rule := range snapshot.RollingRules {
			summary.RuleEvaluations = append(summary.RuleEvaluations, ruleEvaluationResponse(*snapshot, rule, true))
		}
	}
	summary.DataCoverageScore = float32(coverage / float64(len(snapshots)))
	return &summary
}

// addSnapshotTotals accumulates the day totals across eater-days; the rolling
// projection of the last snapshot wins because weekly rules are cumulative.
func addSnapshotTotals(summary *model.NutritionQualitySummary, snapshot planningdom.QualitySnapshot) {
	totals := snapshot.Day.Totals
	addFloat(&summary.FiberG, totals.FiberG)
	addFloat(&summary.SodiumMg, totals.SodiumMg)
	addFloat(&summary.SaturatedFatG, totals.SaturatedFatG)
	addFloat(&summary.AddedSugarsG, totals.AddedSugarsG)
	addFloat(&summary.FruitVegG, totals.FruitVegG)
	addFloat(&summary.GrainGEquiv, totals.GrainGEquiv)
	addFloat(&summary.WholeGrainGEquiv, totals.WholeGrainGEquiv)
	addFloat(&summary.LegumeG, totals.LegumeG)

	if summary.GrainGEquiv != nil && *summary.GrainGEquiv > 0 && summary.WholeGrainGEquiv != nil {
		ratio := *summary.WholeGrainGEquiv / *summary.GrainGEquiv
		summary.WholeGrainRatio = &ratio
	}
	summary.LegumeServingsRolling7d = floatValuePointer(snapshot.Rolling.LegumeServings)
	summary.RedMeatGRolling7d = floatValuePointer(snapshot.Rolling.RedMeatG)
	summary.ProcessedMeatGRolling7d = floatValuePointer(snapshot.Rolling.ProcessedMeatG)
	summary.UpfKcalShareEst = float32Pointer(snapshot.Day.Shares.UPFKcalShare(snapshot.Day.Macros.Kcal))
}

func ruleEvaluationResponse(
	snapshot planningdom.QualitySnapshot,
	rule nutrition.RuleEvaluationResult,
	rolling bool,
) model.RuleEvaluation {
	eaterID := int64(snapshot.EaterID)
	evaluation := model.RuleEvaluation{
		RuleCode:      rule.RuleCode,
		EntityType:    model.RuleEvaluationEntityType(planningdom.RuleEntityDay),
		EaterId:       &eaterID,
		Basis:         model.RuleEvaluationBasis(snapshot.BasisOrPlanned()),
		Status:        model.RuleEvaluationStatus(rule.Status),
		ActualValue:   float32Pointer(rule.Actual),
		TargetValue:   float32Pointer(rule.Target),
		Unit:          optionalString(rule.Unit),
		CoverageScore: float32(rule.Coverage),
		Message:       optionalString(rule.Message),
	}
	if rolling {
		evaluation.EntityType = model.RuleEvaluationEntityType(planningdom.RuleEntityRollingWindow)
		start := openapiDate(snapshot.WindowStart())
		end := openapiDate(snapshot.Date)
		evaluation.WindowStart = &start
		evaluation.WindowEnd = &end
		return evaluation
	}
	date := openapiDate(snapshot.Date)
	evaluation.Date = &date
	return evaluation
}

// solverErrorResponse turns solver diagnostics into the documented 422 body.
func solverErrorResponse(failure solver.Failure) model.SolverError {
	response := model.SolverError{
		Error:       model.SolverErrorErrorSolverError,
		Code:        model.SolverErrorCode(failure.Code),
		Exhaustive:  failure.Exhaustive,
		Diagnostics: make([]model.SolverDiagnostic, 0),
	}

	eaterID := int64(failure.EaterID)
	date := openapiDate(failure.Date)
	for _, diagnostic := range failure.Diagnostics() {
		if diagnostic.Code == solver.DiagnosticMacroInfeasible || diagnostic.Code == solver.DiagnosticSearchLimit {
			response.Message = diagnostic.Message
		}
		entry := model.SolverDiagnostic{
			EaterId:    &eaterID,
			Date:       &date,
			Constraint: solverConstraint(diagnostic.Code),
			Reason:     diagnostic.Message,
		}
		response.Diagnostics = append(response.Diagnostics, entry)
	}
	for _, constraint := range failure.MacroConstraints {
		response.Diagnostics = append(response.Diagnostics, macroDiagnostic(eaterID, date, constraint))
	}
	if response.Message == "" {
		response.Message = "no plan satisfies the request"
	}
	return response
}

func macroDiagnostic(eaterID int64, date openapi_types.Date, constraint solver.MacroConstraint) model.SolverDiagnostic {
	targetMin := float32(constraint.TargetMin)
	targetMax := float32(constraint.TargetMax)
	reachableMin := float32(constraint.ReachableMin)
	reachableMax := float32(constraint.ReachableMax)
	return model.SolverDiagnostic{
		EaterId:      &eaterID,
		Date:         &date,
		Constraint:   model.SolverDiagnosticConstraint(constraint.Code),
		TargetMin:    &targetMin,
		TargetMax:    &targetMax,
		ReachableMin: &reachableMin,
		ReachableMax: &reachableMax,
		Reason:       "the available candidates cannot reach this target bound",
	}
}

func solverConstraint(code solver.DiagnosticCode) model.SolverDiagnosticConstraint {
	switch code {
	case solver.DiagnosticAllergenConflict, solver.DiagnosticExclusionConflict:
		return model.HardExclusion
	case solver.DiagnosticHardQualityFailure:
		return model.IndustrialTransFat
	case solver.DiagnosticSearchLimit:
		return model.SearchBudget
	case solver.DiagnosticMacroInfeasible:
		return model.Kcal
	default:
		return model.CandidateAvailability
	}
}

func logStatusOrPlanned(status planningdom.MealLogStatus) planningdom.MealLogStatus {
	if status.Valid() {
		return status
	}
	return planningdom.MealLogStatusPlanned
}

const dayDuration = 24 * time.Hour

func openapiDate(value time.Time) openapi_types.Date {
	return openapi_types.Date{Time: value.UTC().Truncate(dayDuration)}
}

func floatValuePointer(value float64) *float32 {
	converted := float32(value)
	return &converted
}

func addFloat(target **float32, value float64) {
	if *target == nil {
		*target = floatValuePointer(value)
		return
	}
	**target += float32(value)
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
