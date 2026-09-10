package httpapi

import (
	openapitypes "github.com/oapi-codegen/runtime/types"

	"github.com/ZetoOfficial/meal-planner/api/sdk/go/model"
	eaterdom "github.com/ZetoOfficial/meal-planner/internal/domain/eater"
	usecaseeater "github.com/ZetoOfficial/meal-planner/internal/usecase/eater"
)

func eaterCommand(input model.EaterInput) usecaseeater.CreateEaterCommand {
	result := usecaseeater.CreateEaterCommand{Name: input.Name, Goal: macroRangeCommand(input.Goal)}
	if input.Sex != nil {
		result.Sex = eaterdom.Sex(*input.Sex)
	}
	if input.Allergens != nil {
		result.Allergens = make([]eaterdom.Allergen, len(*input.Allergens))
		for i, allergen := range *input.Allergens {
			result.Allergens[i] = eaterdom.Allergen(allergen)
		}
	}
	return result
}

func macroRangeCommand(input model.MacroRange) usecaseeater.MacroGoalCommand {
	return usecaseeater.MacroGoalCommand{Range: eaterdom.MacroRange{
		ProteinMinG: float64(input.ProteinMinG), ProteinMaxG: float64(input.ProteinMaxG), FatMinG: float64(input.FatMinG), FatMaxG: float64(input.FatMaxG),
		CarbMinG: float64(input.CarbMinG), CarbMaxG: float64(input.CarbMaxG), KcalMin: input.KcalMin, KcalMax: input.KcalMax,
	}}
}

func macroGoalCommand(input model.MacroGoalInput) usecaseeater.MacroGoalCommand {
	result := macroRangeCommand(model.MacroRange{
		ProteinMinG: input.ProteinMinG, ProteinMaxG: input.ProteinMaxG, FatMinG: input.FatMinG, FatMaxG: input.FatMaxG,
		CarbMinG: input.CarbMinG, CarbMaxG: input.CarbMaxG, KcalMin: input.KcalMin, KcalMax: input.KcalMax,
	})
	result.Note = input.Note
	if input.EffectiveFrom != nil {
		result.EffectiveFrom = input.EffectiveFrom.Time
	}
	return result
}

func fixedMealCommand(input model.FixedMealInput) usecaseeater.CreateFixedMealCommand {
	result := usecaseeater.CreateFixedMealCommand{MealType: eaterdom.MealType(input.MealType), Label: input.Label, Macros: eaterdom.Macros{
		ProteinG: float64(input.Macros.ProteinG), FatG: float64(input.Macros.FatG), CarbG: float64(input.Macros.CarbG), Kcal: input.Macros.Kcal,
	}}
	if input.Quality != nil {
		result.Quality = fixedMealQualityCommand(*input.Quality)
		result.IncludeInAnalytics = input.Quality.IncludeInAnalytics
		result.IncludeInWarnings = input.Quality.IncludeInWarnings
	}
	return result
}

func fixedMealQualityCommand(input model.FixedMealQuality) eaterdom.FixedMealQuality {
	return eaterdom.FixedMealQuality{
		SaturatedFatG: input.SaturatedFatG, TransFatG: input.TransFatG, FiberG: input.FiberG,
		SodiumMg: input.SodiumMg, AddedSugarsG: input.AddedSugarsG, FreeSugarsG: input.FreeSugarsG,
		ProcessedMeatG: input.ProcessedMeatG, RedMeatG: input.RedMeatG,
		GrainGEquiv: input.GrainGEquiv, WholeGrainGEquiv: input.WholeGrainGEquiv,
		LegumeG: input.LegumeG, FruitVegG: input.FruitVegG, UPFKcal: input.UpfKcal,
	}
}

func exclusionCommand(input model.ExclusionInput) usecaseeater.CreateExclusionCommand {
	result := usecaseeater.CreateExclusionCommand{TargetKind: eaterdom.ExclusionTarget(input.TargetKind), TargetValue: input.TargetValue, Note: input.Note}
	if input.Severity != nil {
		result.Severity = eaterdom.ExclusionSeverity(*input.Severity)
	}
	return result
}

func eaterResponse(value eaterdom.Eater) model.Eater {
	allergens := make([]string, len(value.Allergens))
	for i := range value.Allergens {
		allergens[i] = string(value.Allergens[i])
	}
	exclusions := make([]model.Exclusion, 0, len(value.Exclusions))
	for i := range value.Exclusions {
		exclusions = append(exclusions, exclusionResponse(value.Exclusions[i]))
	}
	result := model.Eater{Id: int64(value.ID), Name: value.Name, Allergens: &allergens, Exclusions: &exclusions}
	sex := model.Sex(value.Sex)
	result.Sex = &sex
	if value.CurrentGoal != nil {
		goal := macroGoalResponse(*value.CurrentGoal)
		result.CurrentGoal = &goal
	}
	return result
}

func eatersResponse(eaters []eaterdom.Eater) []model.Eater {
	result := make([]model.Eater, len(eaters))
	for i := range eaters {
		result[i] = eaterResponse(eaters[i])
	}
	return result
}

func macroGoalResponse(value eaterdom.MacroGoal) model.MacroGoal {
	return model.MacroGoal{
		Id: int64(value.ID), EffectiveFrom: openapitypes.Date{Time: value.EffectiveFrom}, Note: value.Note,
		ProteinMinG: value.Range.ProteinMinG, ProteinMaxG: value.Range.ProteinMaxG, FatMinG: value.Range.FatMinG, FatMaxG: value.Range.FatMaxG,
		CarbMinG: value.Range.CarbMinG, CarbMaxG: value.Range.CarbMaxG, KcalMin: value.Range.KcalMin, KcalMax: value.Range.KcalMax,
	}
}

func macroGoalsResponse(goals []eaterdom.MacroGoal) []model.MacroGoal {
	result := make([]model.MacroGoal, len(goals))
	for i := range goals {
		result[i] = macroGoalResponse(goals[i])
	}
	return result
}

func fixedMealResponse(value eaterdom.FixedMeal) model.FixedMeal {
	active := value.Active
	return model.FixedMeal{
		Id: int64(value.ID), EaterId: int64(value.EaterID), MealType: model.MealType(value.MealType), Label: value.Label,
		Macros:  model.Macros{ProteinG: value.Macros.ProteinG, FatG: value.Macros.FatG, CarbG: value.Macros.CarbG, Kcal: value.Macros.Kcal},
		Quality: fixedMealQualityResponse(value.Quality), Active: &active,
	}
}

func fixedMealsResponse(meals []eaterdom.FixedMeal) []model.FixedMeal {
	result := make([]model.FixedMeal, len(meals))
	for i := range meals {
		result[i] = fixedMealResponse(meals[i])
	}
	return result
}

func fixedMealQualityResponse(value eaterdom.FixedMealQuality) *model.FixedMealQuality {
	return &model.FixedMealQuality{
		SaturatedFatG: value.SaturatedFatG, TransFatG: value.TransFatG, FiberG: value.FiberG, SodiumMg: value.SodiumMg,
		AddedSugarsG: value.AddedSugarsG, FreeSugarsG: value.FreeSugarsG, ProcessedMeatG: value.ProcessedMeatG, RedMeatG: value.RedMeatG,
		GrainGEquiv: value.GrainGEquiv, WholeGrainGEquiv: value.WholeGrainGEquiv,
		LegumeG: value.LegumeG, FruitVegG: value.FruitVegG, UpfKcal: value.UPFKcal,
		IncludeInAnalytics: boolPointer(value.IncludeInAnalytics), IncludeInWarnings: boolPointer(value.IncludeInWarnings),
	}
}

func exclusionResponse(value eaterdom.Exclusion) model.Exclusion {
	return model.Exclusion{Id: int64(value.ID), TargetKind: model.ExclusionTargetKind(value.TargetKind), TargetValue: value.TargetValue, Severity: model.ExclusionSeverity(value.Severity), Note: value.Note}
}

func exclusionsResponse(exclusions []eaterdom.Exclusion) []model.Exclusion {
	result := make([]model.Exclusion, len(exclusions))
	for i := range exclusions {
		result[i] = exclusionResponse(exclusions[i])
	}
	return result
}
