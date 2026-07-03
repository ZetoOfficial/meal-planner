package httpapi

import (
	"errors"
	"log/slog"
	"net/http"

	api "github.com/ZetoOfficial/meal-planner/internal/api/openapi"
	apperr "github.com/ZetoOfficial/meal-planner/internal/errors"
)

func writeError(w http.ResponseWriter, err error) {
	status, body := mapError(err)
	if status >= http.StatusInternalServerError {
		slog.Error("http internal error", slog.Any("error", err))
	}
	writeJSON(w, status, body)
}

func mapError(err error) (status int, body any) {
	var impossible *apperr.ImpossiblePlanError
	if errors.As(err, &impossible) {
		return http.StatusUnprocessableEntity, solverError(impossible)
	}

	var app *apperr.Error
	if errors.As(err, &app) {
		return statusForCode(app.Code), api.Error{
			Error:   string(app.Code),
			Message: app.Error(),
			Details: optionalDetails(app.Details),
		}
	}

	return http.StatusInternalServerError, api.Error{
		Error:   string(apperr.CodeInternal),
		Message: "internal server error",
	}
}

func statusForCode(code apperr.Code) int {
	switch code {
	case apperr.CodeValidation:
		return http.StatusUnprocessableEntity
	case apperr.CodeNotFound:
		return http.StatusNotFound
	case apperr.CodeConflict:
		return http.StatusConflict
	case apperr.CodeUnauthorized:
		return http.StatusUnauthorized
	case apperr.CodeUnavailable:
		return http.StatusServiceUnavailable
	case apperr.CodeImpossiblePlan:
		return http.StatusUnprocessableEntity
	default:
		return http.StatusInternalServerError
	}
}

func optionalDetails(details []string) *[]string {
	if len(details) == 0 {
		return nil
	}
	return &details
}

func solverError(err *apperr.ImpossiblePlanError) api.SolverError {
	body := api.SolverError{
		Error:   string(apperr.CodeImpossiblePlan),
		Message: err.Error(),
	}
	if len(err.UnmetConstraints) == 0 {
		return body
	}

	constraints := make([]api.UnmetConstraint, 0, len(err.UnmetConstraints))
	for _, constraint := range err.UnmetConstraints {
		constraints = append(constraints, api.UnmetConstraint{
			Reason:  constraint.Reason,
			RoleTag: constraint.RoleTag,
		})
	}
	body.UnmetConstraints = &constraints
	return body
}
