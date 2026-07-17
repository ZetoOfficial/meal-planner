package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/ZetoOfficial/meal-planner/internal/api/openapi"
	eaterdom "github.com/ZetoOfficial/meal-planner/internal/domain/eater"
	apperr "github.com/ZetoOfficial/meal-planner/internal/errors"
)

func (a *api) ListEaters(w http.ResponseWriter, r *http.Request) {
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
	if err := a.eater.DeleteExclusion(r.Context(), eaterdom.ExclusionID(id)); err != nil {
		writeError(w, err)
		return
	}
	writeOpenAPIResponse(w, openapi.DeleteExclusion204Response{}.VisitDeleteExclusionResponse)
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
