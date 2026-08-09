package httpapi

import (
	openapitypes "github.com/oapi-codegen/runtime/types"

	"github.com/ZetoOfficial/meal-planner/internal/api/openapi"
	eaterdom "github.com/ZetoOfficial/meal-planner/internal/domain/eater"
	usecaseeater "github.com/ZetoOfficial/meal-planner/internal/usecase/eater"
)

func eaterCommand(input openapi.EaterInput) usecaseeater.CreateEaterCommand {
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

func macroRangeCommand(input openapi.MacroRange) usecaseeater.MacroGoalCommand {
	return usecaseeater.MacroGoalCommand{Range: eaterdom.MacroRange{
		ProteinMinG: float64(input.ProteinMinG), ProteinMaxG: float64(input.ProteinMaxG), FatMinG: float64(input.FatMinG), FatMaxG: float64(input.FatMaxG),
		CarbMinG: float64(input.CarbMinG), CarbMaxG: float64(input.CarbMaxG), KcalMin: input.KcalMin, KcalMax: input.KcalMax,
	}}
}

func macroGoalCommand(input openapi.MacroGoalInput) usecaseeater.MacroGoalCommand {
	result := macroRangeCommand(openapi.MacroRange{
		ProteinMinG: input.ProteinMinG, ProteinMaxG: input.ProteinMaxG, FatMinG: input.FatMinG, FatMaxG: input.FatMaxG,
		CarbMinG: input.CarbMinG, CarbMaxG: input.CarbMaxG, KcalMin: input.KcalMin, KcalMax: input.KcalMax,
	})
	result.Note = input.Note
	if input.EffectiveFrom != nil {
		result.EffectiveFrom = input.EffectiveFrom.Time
	}
	return result
}

func fixedMealCommand(input openapi.FixedMealInput) usecaseeater.CreateFixedMealCommand {
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

func fixedMealQualityCommand(input openapi.FixedMealQuality) eaterdom.FixedMealQuality {
	return eaterdom.FixedMealQuality{
		SaturatedFatG: float64Pointer(input.SaturatedFatG), TransFatG: float64Pointer(input.TransFatG), FiberG: float64Pointer(input.FiberG),
		SodiumMg: float64Pointer(input.SodiumMg), AddedSugarsG: float64Pointer(input.AddedSugarsG), FreeSugarsG: float64Pointer(input.FreeSugarsG),
		ProcessedMeatG: float64Pointer(input.ProcessedMeatG), RedMeatG: float64Pointer(input.RedMeatG),
		GrainGEquiv: float64Pointer(input.GrainGEquiv), WholeGrainGEquiv: float64Pointer(input.WholeGrainGEquiv),
		LegumeG: float64Pointer(input.LegumeG), FruitVegG: float64Pointer(input.FruitVegG), UPFKcal: float64Pointer(input.UpfKcal),
	}
}

func exclusionCommand(input openapi.ExclusionInput) usecaseeater.CreateExclusionCommand {
	result := usecaseeater.CreateExclusionCommand{TargetKind: eaterdom.ExclusionTarget(input.TargetKind), TargetValue: input.TargetValue, Note: input.Note}
	if input.Severity != nil {
		result.Severity = eaterdom.ExclusionSeverity(*input.Severity)
	}
	return result
}

func eaterResponse(value eaterdom.Eater) openapi.Eater {
	allergens := make([]string, len(value.Allergens))
	for i := range value.Allergens {
		allergens[i] = string(value.Allergens[i])
	}
	exclusions := make([]openapi.Exclusion, 0, len(value.Exclusions))
	for i := range value.Exclusions {
		exclusions = append(exclusions, exclusionResponse(value.Exclusions[i]))
	}
	result := openapi.Eater{Id: int64(value.ID), Name: value.Name, Allergens: &allergens, Exclusions: &exclusions}
	sex := openapi.Sex(value.Sex)
	result.Sex = &sex
	if value.CurrentGoal != nil {
		goal := macroGoalResponse(*value.CurrentGoal)
		result.CurrentGoal = &goal
	}
	return result
}

func eatersResponse(eaters []eaterdom.Eater) []openapi.Eater {
	result := make([]openapi.Eater, len(eaters))
	for i := range eaters {
		result[i] = eaterResponse(eaters[i])
	}
	return result
}

func macroGoalResponse(value eaterdom.MacroGoal) openapi.MacroGoal {
	return openapi.MacroGoal{
		Id: int64(value.ID), EffectiveFrom: openapitypes.Date{Time: value.EffectiveFrom}, Note: value.Note,
		ProteinMinG: float32(value.Range.ProteinMinG), ProteinMaxG: float32(value.Range.ProteinMaxG), FatMinG: float32(value.Range.FatMinG), FatMaxG: float32(value.Range.FatMaxG),
		CarbMinG: float32(value.Range.CarbMinG), CarbMaxG: float32(value.Range.CarbMaxG), KcalMin: value.Range.KcalMin, KcalMax: value.Range.KcalMax,
	}
}

func macroGoalsResponse(goals []eaterdom.MacroGoal) []openapi.MacroGoal {
	result := make([]openapi.MacroGoal, len(goals))
	for i := range goals {
		result[i] = macroGoalResponse(goals[i])
	}
	return result
}

func fixedMealResponse(value eaterdom.FixedMeal) openapi.FixedMeal {
	active := value.Active
	return openapi.FixedMeal{
		Id: int64(value.ID), EaterId: int64(value.EaterID), MealType: openapi.MealType(value.MealType), Label: value.Label,
		Macros:  openapi.Macros{ProteinG: float32(value.Macros.ProteinG), FatG: float32(value.Macros.FatG), CarbG: float32(value.Macros.CarbG), Kcal: value.Macros.Kcal},
		Quality: fixedMealQualityResponse(value.Quality), Active: &active,
	}
}

func fixedMealsResponse(meals []eaterdom.FixedMeal) []openapi.FixedMeal {
	result := make([]openapi.FixedMeal, len(meals))
	for i := range meals {
		result[i] = fixedMealResponse(meals[i])
	}
	return result
}

func fixedMealQualityResponse(value eaterdom.FixedMealQuality) *openapi.FixedMealQuality {
	return &openapi.FixedMealQuality{
		SaturatedFatG: float32Pointer(value.SaturatedFatG), TransFatG: float32Pointer(value.TransFatG), FiberG: float32Pointer(value.FiberG), SodiumMg: float32Pointer(value.SodiumMg),
		AddedSugarsG: float32Pointer(value.AddedSugarsG), FreeSugarsG: float32Pointer(value.FreeSugarsG), ProcessedMeatG: float32Pointer(value.ProcessedMeatG), RedMeatG: float32Pointer(value.RedMeatG),
		GrainGEquiv: float32Pointer(value.GrainGEquiv), WholeGrainGEquiv: float32Pointer(value.WholeGrainGEquiv),
		LegumeG: float32Pointer(value.LegumeG), FruitVegG: float32Pointer(value.FruitVegG), UpfKcal: float32Pointer(value.UPFKcal),
		IncludeInAnalytics: boolPointer(value.IncludeInAnalytics), IncludeInWarnings: boolPointer(value.IncludeInWarnings),
	}
}

func exclusionResponse(value eaterdom.Exclusion) openapi.Exclusion {
	return openapi.Exclusion{Id: int64(value.ID), TargetKind: openapi.ExclusionTargetKind(value.TargetKind), TargetValue: value.TargetValue, Severity: openapi.ExclusionSeverity(value.Severity), Note: value.Note}
}

func exclusionsResponse(exclusions []eaterdom.Exclusion) []openapi.Exclusion {
	result := make([]openapi.Exclusion, len(exclusions))
	for i := range exclusions {
		result[i] = exclusionResponse(exclusions[i])
	}
	return result
}
