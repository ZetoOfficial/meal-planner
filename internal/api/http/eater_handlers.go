package httpapi

import (
	"encoding/json"
	"net/http"

	openapitypes "github.com/oapi-codegen/runtime/types"

	"github.com/ZetoOfficial/meal-planner/internal/api/openapi"
	eaterdom "github.com/ZetoOfficial/meal-planner/internal/domain/eater"
	apperr "github.com/ZetoOfficial/meal-planner/internal/errors"
	usecaseeater "github.com/ZetoOfficial/meal-planner/internal/usecase/eater"
)

func (a *api) ListEaters(w http.ResponseWriter, r *http.Request) {
	if !a.requireEater(w) {
		return
	}
	eaters, err := a.eater.ListEaters(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	result := make([]openapi.Eater, 0, len(eaters))
	for i := range eaters {
		result = append(result, eaterResponse(eaters[i]))
	}
	writeOpenAPIResponse(w, openapi.ListEaters200JSONResponse(result).VisitListEatersResponse)
}

func (a *api) CreateEater(w http.ResponseWriter, r *http.Request) {
	if !a.requireEater(w) {
		return
	}
	var input openapi.EaterInput
	if err := decodeJSON(r, &input, validateEaterInput); err != nil {
		writeError(w, err)
		return
	}
	eater, err := a.eater.CreateEater(r.Context(), eaterCommand(input))
	if err != nil {
		writeError(w, err)
		return
	}
	writeOpenAPIResponse(w, openapi.CreateEater201JSONResponse{
		EaterOKJSONResponse: openapi.EaterOKJSONResponse(eaterResponse(eater)),
	}.VisitCreateEaterResponse)
}

func (a *api) GetEater(w http.ResponseWriter, r *http.Request, id openapi.EaterId) {
	if !a.requireEater(w) {
		return
	}
	eater, err := a.eater.GetEater(r.Context(), eaterdom.ID(id))
	if err != nil {
		writeError(w, err)
		return
	}
	writeOpenAPIResponse(w, openapi.GetEater200JSONResponse{
		EaterOKJSONResponse: openapi.EaterOKJSONResponse(eaterResponse(eater)),
	}.VisitGetEaterResponse)
}

func (a *api) UpdateEater(w http.ResponseWriter, r *http.Request, id openapi.EaterId) {
	if !a.requireEater(w) {
		return
	}
	var input openapi.EaterInput
	if err := decodeJSON(r, &input, validateEaterInput); err != nil {
		writeError(w, err)
		return
	}
	eater, err := a.eater.UpdateEater(r.Context(), eaterdom.ID(id), eaterCommand(input))
	if err != nil {
		writeError(w, err)
		return
	}
	writeOpenAPIResponse(w, openapi.UpdateEater200JSONResponse{
		EaterOKJSONResponse: openapi.EaterOKJSONResponse(eaterResponse(eater)),
	}.VisitUpdateEaterResponse)
}

func (a *api) ListEaterGoals(w http.ResponseWriter, r *http.Request, id openapi.EaterId) {
	if !a.requireEater(w) {
		return
	}
	goals, err := a.eater.ListGoals(r.Context(), eaterdom.ID(id))
	if err != nil {
		writeError(w, err)
		return
	}
	result := make([]openapi.MacroGoal, 0, len(goals))
	for i := range goals {
		result = append(result, macroGoalResponse(goals[i]))
	}
	writeOpenAPIResponse(w, openapi.ListEaterGoals200JSONResponse(result).VisitListEaterGoalsResponse)
}

func (a *api) CreateEaterGoal(w http.ResponseWriter, r *http.Request, id openapi.EaterId) {
	if !a.requireEater(w) {
		return
	}
	var input openapi.MacroGoalInput
	if err := decodeJSON(r, &input, validateMacroGoalInput); err != nil {
		writeError(w, err)
		return
	}
	goal, err := a.eater.CreateGoal(r.Context(), eaterdom.ID(id), macroGoalCommand(input))
	if err != nil {
		writeError(w, err)
		return
	}
	writeOpenAPIResponse(w, openapi.CreateEaterGoal201JSONResponse(macroGoalResponse(goal)).VisitCreateEaterGoalResponse)
}

func (a *api) ListFixedMeals(w http.ResponseWriter, r *http.Request, id openapi.EaterId) {
	if !a.requireEater(w) {
		return
	}
	meals, err := a.eater.ListFixedMeals(r.Context(), eaterdom.ID(id))
	if err != nil {
		writeError(w, err)
		return
	}
	result := make([]openapi.FixedMeal, 0, len(meals))
	for i := range meals {
		result = append(result, fixedMealResponse(meals[i]))
	}
	writeOpenAPIResponse(w, openapi.ListFixedMeals200JSONResponse(result).VisitListFixedMealsResponse)
}

func (a *api) CreateFixedMeal(w http.ResponseWriter, r *http.Request, id openapi.EaterId) {
	if !a.requireEater(w) {
		return
	}
	var input openapi.FixedMealInput
	if err := decodeJSON(r, &input, validateFixedMealInput); err != nil {
		writeError(w, err)
		return
	}
	meal, err := a.eater.CreateFixedMeal(r.Context(), eaterdom.ID(id), fixedMealCommand(input))
	if err != nil {
		writeError(w, err)
		return
	}
	writeOpenAPIResponse(w, openapi.CreateFixedMeal201JSONResponse(fixedMealResponse(meal)).VisitCreateFixedMealResponse)
}

func (a *api) ListExclusions(w http.ResponseWriter, r *http.Request, id openapi.EaterId) {
	if !a.requireEater(w) {
		return
	}
	exclusions, err := a.eater.ListExclusions(r.Context(), eaterdom.ID(id))
	if err != nil {
		writeError(w, err)
		return
	}
	result := make([]openapi.Exclusion, 0, len(exclusions))
	for i := range exclusions {
		result = append(result, exclusionResponse(exclusions[i]))
	}
	writeOpenAPIResponse(w, openapi.ListExclusions200JSONResponse(result).VisitListExclusionsResponse)
}

func (a *api) CreateExclusion(w http.ResponseWriter, r *http.Request, id openapi.EaterId) {
	if !a.requireEater(w) {
		return
	}
	var input openapi.ExclusionInput
	if err := decodeJSON(r, &input, validateExclusionInput); err != nil {
		writeError(w, err)
		return
	}
	exclusion, err := a.eater.CreateExclusion(r.Context(), eaterdom.ID(id), exclusionCommand(input))
	if err != nil {
		writeError(w, err)
		return
	}
	writeOpenAPIResponse(w, openapi.CreateExclusion201JSONResponse(exclusionResponse(exclusion)).VisitCreateExclusionResponse)
}

func (a *api) DeleteExclusion(w http.ResponseWriter, r *http.Request, id int64) {
	if !a.requireEater(w) {
		return
	}
	if err := a.eater.DeleteExclusion(r.Context(), eaterdom.ExclusionID(id)); err != nil {
		writeError(w, err)
		return
	}
	writeOpenAPIResponse(w, openapi.DeleteExclusion204Response{}.VisitDeleteExclusionResponse)
}

func eaterCommand(input openapi.EaterInput) usecaseeater.CreateEaterCommand {
	result := usecaseeater.CreateEaterCommand{
		Name: input.Name,
		Goal: macroRangeCommand(input.Goal),
	}
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
		ProteinMinG: float64(input.ProteinMinG), ProteinMaxG: float64(input.ProteinMaxG),
		FatMinG: float64(input.FatMinG), FatMaxG: float64(input.FatMaxG),
		CarbMinG: float64(input.CarbMinG), CarbMaxG: float64(input.CarbMaxG),
		KcalMin: input.KcalMin, KcalMax: input.KcalMax,
	}}
}

func macroGoalCommand(input openapi.MacroGoalInput) usecaseeater.MacroGoalCommand {
	result := usecaseeater.MacroGoalCommand{
		Range: eaterdom.MacroRange{
			ProteinMinG: float64(input.ProteinMinG), ProteinMaxG: float64(input.ProteinMaxG),
			FatMinG: float64(input.FatMinG), FatMaxG: float64(input.FatMaxG),
			CarbMinG: float64(input.CarbMinG), CarbMaxG: float64(input.CarbMaxG),
			KcalMin: input.KcalMin, KcalMax: input.KcalMax,
		},
		Note: input.Note,
	}
	if input.EffectiveFrom != nil {
		result.EffectiveFrom = input.EffectiveFrom.Time
	}
	return result
}

func fixedMealCommand(input openapi.FixedMealInput) usecaseeater.CreateFixedMealCommand {
	result := usecaseeater.CreateFixedMealCommand{
		MealType: eaterdom.MealType(input.MealType),
		Label:    input.Label,
		Macros: eaterdom.Macros{
			ProteinG: float64(input.Macros.ProteinG), FatG: float64(input.Macros.FatG),
			CarbG: float64(input.Macros.CarbG), Kcal: input.Macros.Kcal,
		},
	}
	if input.Quality != nil {
		result.Quality = fixedMealQualityCommand(*input.Quality)
		result.IncludeInAnalytics = input.Quality.IncludeInAnalytics
		result.IncludeInWarnings = input.Quality.IncludeInWarnings
	}
	return result
}

func fixedMealQualityCommand(input openapi.FixedMealQuality) eaterdom.FixedMealQuality {
	return eaterdom.FixedMealQuality{
		SaturatedFatG: float64Pointer(input.SaturatedFatG), TransFatG: float64Pointer(input.TransFatG),
		FiberG: float64Pointer(input.FiberG), SodiumMg: float64Pointer(input.SodiumMg),
		AddedSugarsG: float64Pointer(input.AddedSugarsG), FreeSugarsG: float64Pointer(input.FreeSugarsG),
		ProcessedMeatG: float64Pointer(input.ProcessedMeatG), RedMeatG: float64Pointer(input.RedMeatG),
		WholeGrainGEquiv: float64Pointer(input.WholeGrainGEquiv), FruitVegG: float64Pointer(input.FruitVegG),
	}
}

func exclusionCommand(input openapi.ExclusionInput) usecaseeater.CreateExclusionCommand {
	result := usecaseeater.CreateExclusionCommand{
		TargetKind:  eaterdom.ExclusionTarget(input.TargetKind),
		TargetValue: input.TargetValue,
		Note:        input.Note,
	}
	if input.Severity != nil {
		result.Severity = eaterdom.ExclusionSeverity(*input.Severity)
	}
	return result
}

func validateEaterInput(object map[string]json.RawMessage) error {
	if err := requireJSONFields(object, "name", "goal"); err != nil {
		return err
	}
	return validateNestedObject(object, "goal", requireMacroRange)
}

func validateMacroGoalInput(object map[string]json.RawMessage) error {
	return requireMacroRange(object)
}

func validateFixedMealInput(object map[string]json.RawMessage) error {
	if err := requireJSONFields(object, "meal_type", "label", "macros"); err != nil {
		return err
	}
	return validateNestedObject(object, "macros", requireMacros)
}

func validateExclusionInput(object map[string]json.RawMessage) error {
	return requireJSONFields(object, "target_kind", "target_value")
}

func validateNestedObject(object map[string]json.RawMessage, field string, validate jsonObjectValidator) error {
	var nested map[string]json.RawMessage
	if err := json.Unmarshal(object[field], &nested); err != nil || nested == nil {
		return apperr.Validation("required JSON field must be an object", field)
	}
	return validate(nested)
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

func macroGoalResponse(value eaterdom.MacroGoal) openapi.MacroGoal {
	return openapi.MacroGoal{
		Id: int64(value.ID), EffectiveFrom: openapitypes.Date{Time: value.EffectiveFrom}, Note: value.Note,
		ProteinMinG: float32(value.Range.ProteinMinG), ProteinMaxG: float32(value.Range.ProteinMaxG),
		FatMinG: float32(value.Range.FatMinG), FatMaxG: float32(value.Range.FatMaxG),
		CarbMinG: float32(value.Range.CarbMinG), CarbMaxG: float32(value.Range.CarbMaxG),
		KcalMin: value.Range.KcalMin, KcalMax: value.Range.KcalMax,
	}
}

func fixedMealResponse(value eaterdom.FixedMeal) openapi.FixedMeal {
	active := value.Active
	return openapi.FixedMeal{
		Id: int64(value.ID), EaterId: int64(value.EaterID), MealType: openapi.MealType(value.MealType), Label: value.Label,
		Macros:  openapi.Macros{ProteinG: float32(value.Macros.ProteinG), FatG: float32(value.Macros.FatG), CarbG: float32(value.Macros.CarbG), Kcal: value.Macros.Kcal},
		Quality: fixedMealQualityResponse(value.Quality), Active: &active,
	}
}

func fixedMealQualityResponse(value eaterdom.FixedMealQuality) *openapi.FixedMealQuality {
	return &openapi.FixedMealQuality{
		SaturatedFatG: float32Pointer(value.SaturatedFatG), TransFatG: float32Pointer(value.TransFatG),
		FiberG: float32Pointer(value.FiberG), SodiumMg: float32Pointer(value.SodiumMg),
		AddedSugarsG: float32Pointer(value.AddedSugarsG), FreeSugarsG: float32Pointer(value.FreeSugarsG),
		ProcessedMeatG: float32Pointer(value.ProcessedMeatG), RedMeatG: float32Pointer(value.RedMeatG),
		WholeGrainGEquiv: float32Pointer(value.WholeGrainGEquiv), FruitVegG: float32Pointer(value.FruitVegG),
		IncludeInAnalytics: boolPointer(value.IncludeInAnalytics), IncludeInWarnings: boolPointer(value.IncludeInWarnings),
	}
}

func exclusionResponse(value eaterdom.Exclusion) openapi.Exclusion {
	return openapi.Exclusion{
		Id: int64(value.ID), TargetKind: openapi.ExclusionTargetKind(value.TargetKind), TargetValue: value.TargetValue,
		Severity: openapi.ExclusionSeverity(value.Severity), Note: value.Note,
	}
}
