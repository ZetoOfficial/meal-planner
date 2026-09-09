package httpapi

import (
	"github.com/ZetoOfficial/meal-planner/api/sdk/go/api"
	"github.com/ZetoOfficial/meal-planner/api/sdk/go/model"
	eaterdom "github.com/ZetoOfficial/meal-planner/internal/domain/eater"
	feedbackdom "github.com/ZetoOfficial/meal-planner/internal/domain/feedback"
	"github.com/ZetoOfficial/meal-planner/internal/domain/nutrition"
	planningdom "github.com/ZetoOfficial/meal-planner/internal/domain/planning"
	usecasereality "github.com/ZetoOfficial/meal-planner/internal/usecase/reality"
)

func mealLogCommand(body model.MealLogInput) planningdom.MealLog {
	log := planningdom.MealLog{
		Status:   planningdom.MealLogStatus(body.LogStatus),
		LoggedBy: eaterdom.ID(body.LoggedBy),
	}
	if body.CookedBy != nil {
		cookedBy := eaterdom.ID(*body.CookedBy)
		log.CookedBy = &cookedBy
	}
	if body.Actual != nil {
		log.Actual = &nutrition.Macros{
			ProteinG: float64(body.Actual.ProteinG),
			FatG:     float64(body.Actual.FatG),
			CarbG:    float64(body.Actual.CarbG),
			Kcal:     body.Actual.Kcal,
		}
	}
	return log
}

func mealsResponse(planID planningdom.PlanID, meals []planningdom.Meal) []model.Meal {
	result := make([]model.Meal, 0, len(meals))
	for i := range meals {
		result = append(result, mealResponseWithPlan(planID, meals[i]))
	}
	return result
}

func mealResponseWithPlan(planID planningdom.PlanID, meal planningdom.Meal) model.Meal {
	components := make([]model.MealComponentAllocation, 0, len(meal.Components))
	for _, component := range meal.Components {
		components = append(components, model.MealComponentAllocation{
			PortionFactor:   float32(component.PortionFactor),
			PlannedServingG: float32Pointer(component.PlannedServingG),
		})
	}

	response := model.Meal{
		Id:         int64(meal.ID),
		PlanId:     int64(planID),
		EaterId:    int64(meal.EaterID),
		MealDate:   openapiDate(meal.Date),
		MealType:   model.MealType(meal.Type),
		Components: components,
		Planned:    macrosResponse(meal.Planned),
		LogStatus:  model.MealLogStatus(logStatusOrPlanned(meal.LogStatus)),
	}
	if meal.FixedMealID != nil {
		id := int64(*meal.FixedMealID)
		response.FixedMealId = &id
	}
	if meal.Actual != nil {
		actual := macrosResponse(*meal.Actual)
		response.Actual = &actual
	}
	if meal.CookedBy != nil {
		id := int64(*meal.CookedBy)
		response.CookedBy = &id
	}
	if meal.LoggedBy != nil {
		id := int64(*meal.LoggedBy)
		response.LoggedBy = &id
	}
	return response
}

func feedbackCommand(body model.FeedbackInput) feedbackdom.Event {
	return feedbackdom.Event{
		EaterID:    eaterdom.ID(body.EaterId),
		TargetType: feedbackdom.TargetType(body.TargetType),
		TargetID:   body.TargetId,
		Rating:     body.Rating,
		Comment:    body.Comment,
	}
}

func feedbackResponse(event feedbackdom.Event) model.FeedbackEvent {
	targetType := string(event.TargetType)
	return model.FeedbackEvent{
		Id:          int64(event.ID),
		EaterId:     int64(event.EaterID),
		TargetType:  targetType,
		TargetId:    event.TargetID,
		Rating:      event.Rating,
		Comment:     event.Comment,
		WeightDelta: float32Pointer(event.WeightDelta),
		CreatedAt:   &event.CreatedAt,
	}
}

func macroAnalyticsResponse(analytics usecasereality.MacroAnalytics) model.MacroSummary {
	eaterID := int64(analytics.Period.EaterID)
	days := analytics.Days
	response := model.MacroSummary{
		EaterId:    eaterID,
		From:       openapiDate(analytics.Period.From),
		To:         openapiDate(analytics.Period.To),
		AvgPlanned: macrosResponse(analytics.AvgPlanned),
		Days:       &days,
	}
	if analytics.HasActual {
		actual := macrosResponse(analytics.AvgActual)
		response.AvgActual = &actual
	}
	return response
}

// qualityAnalyticsResponse reports the stored averages, the weekly exposure and
// the coverage behind them, so a low-confidence period reads as uncertain
// rather than as good.
func qualityAnalyticsResponse(
	analytics usecasereality.QualityAnalytics,
) api.GetNutritionQualityAnalytics200JSONResponse {
	eaterID := int64(analytics.Period.EaterID)
	from := openapiDate(analytics.Period.From)
	to := openapiDate(analytics.Period.To)
	totals := analytics.Averages

	response := api.GetNutritionQualityAnalytics200JSONResponse{
		EaterId:                 &eaterID,
		From:                    &from,
		To:                      &to,
		DataCoverageScore:       float32(analytics.Coverage),
		FiberG:                  floatValuePointer(totals.FiberG),
		SodiumMg:                floatValuePointer(totals.SodiumMg),
		SaturatedFatG:           floatValuePointer(totals.SaturatedFatG),
		AddedSugarsG:            floatValuePointer(totals.AddedSugarsG),
		FruitVegG:               floatValuePointer(totals.FruitVegG),
		GrainGEquiv:             floatValuePointer(totals.GrainGEquiv),
		WholeGrainGEquiv:        floatValuePointer(totals.WholeGrainGEquiv),
		LegumeG:                 floatValuePointer(totals.LegumeG),
		LegumeServingsRolling7d: floatValuePointer(analytics.Rolling.LegumeServings),
		RedMeatGRolling7d:       floatValuePointer(analytics.Rolling.RedMeatG),
		ProcessedMeatGRolling7d: floatValuePointer(analytics.Rolling.ProcessedMeatG),
		RuleEvaluations:         ruleEvaluationsResponse(analytics),
	}
	if totals.GrainGEquiv > 0 {
		ratio := float32(totals.WholeGrainGEquiv / totals.GrainGEquiv)
		response.WholeGrainRatio = &ratio
	}
	return response
}

func ruleEvaluationsResponse(analytics usecasereality.QualityAnalytics) []model.RuleEvaluation {
	eaterID := int64(analytics.Period.EaterID)
	results := make([]model.RuleEvaluation, 0, len(analytics.Evaluations))
	for _, rule := range analytics.Evaluations {
		results = append(results, model.RuleEvaluation{
			RuleCode:      rule.RuleCode,
			EntityType:    model.RuleEvaluationEntityType("day"),
			EaterId:       &eaterID,
			Basis:         model.RuleEvaluationBasis(analytics.Basis),
			Status:        model.RuleEvaluationStatus(rule.Status),
			ActualValue:   float32Pointer(rule.Actual),
			TargetValue:   float32Pointer(rule.Target),
			Unit:          optionalString(rule.Unit),
			CoverageScore: float32(rule.Coverage),
			Message:       optionalString(rule.Message),
		})
	}
	return results
}

func topRecipesResponse(recipes []usecasereality.TopRecipe) []model.TopRecipe {
	result := make([]model.TopRecipe, 0, len(recipes))
	for _, recipe := range recipes {
		result = append(result, model.TopRecipe{
			RecipeId:    int64(recipe.RecipeID),
			Name:        recipe.Name,
			CookedCount: recipe.CookedCount,
		})
	}
	return result
}
