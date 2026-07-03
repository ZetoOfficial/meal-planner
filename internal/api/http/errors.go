package httpapi

import (
	"errors"
	"net/http"

	"github.com/ZetoOfficial/meal-planner/internal/api/openapi"
	apperr "github.com/ZetoOfficial/meal-planner/internal/errors"
)

func writeError(w http.ResponseWriter, err error) {
	status, body := mapError(err)
	writeJSON(w, status, body)
}

func mapError(err error) (int, any) {
	var impossible *apperr.ImpossiblePlanError
	if errors.As(err, &impossible) {
		return http.StatusUnprocessableEntity, solverError(impossible)
	}

	var app *apperr.Error
	if errors.As(err, &app) {
		return statusForCode(app.Code), openapi.Error{
			Error:   string(app.Code),
			Message: app.Error(),
			Details: optionalDetails(app.Details),
		}
	}

	switch {
	case errors.Is(err, apperr.ErrValidation):
		return simpleError(http.StatusUnprocessableEntity, apperr.CodeValidation, "validation failed")
	case errors.Is(err, apperr.ErrNotFound):
		return simpleError(http.StatusNotFound, apperr.CodeNotFound, "not found")
	case errors.Is(err, apperr.ErrConflict):
		return simpleError(http.StatusConflict, apperr.CodeConflict, "conflict")
	case errors.Is(err, apperr.ErrUnauthorized):
		return simpleError(http.StatusUnauthorized, apperr.CodeUnauthorized, "unauthorized")
	case errors.Is(err, apperr.ErrUnavailable):
		return simpleError(http.StatusServiceUnavailable, apperr.CodeUnavailable, "temporarily unavailable")
	case errors.Is(err, apperr.ErrImpossiblePlan):
		return http.StatusUnprocessableEntity, openapi.SolverError{
			Error:   string(apperr.CodeImpossiblePlan),
			Message: "plan cannot satisfy constraints",
		}
	default:
		return simpleError(http.StatusInternalServerError, "internal_error", "internal server error")
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

func simpleError(status int, code apperr.Code, message string) (int, openapi.Error) {
	return status, openapi.Error{
		Error:   string(code),
		Message: message,
	}
}

func optionalDetails(details []string) *[]string {
	if len(details) == 0 {
		return nil
	}
	return &details
}

func solverError(err *apperr.ImpossiblePlanError) openapi.SolverError {
	body := openapi.SolverError{
		Error:   string(apperr.CodeImpossiblePlan),
		Message: err.Error(),
	}
	if len(err.UnmetConstraints) == 0 {
		return body
	}

	constraints := make([]struct {
		Reason  *string `json:"reason,omitempty"`
		RoleTag *string `json:"role_tag,omitempty"`
	}, 0, len(err.UnmetConstraints))
	for _, constraint := range err.UnmetConstraints {
		reason := constraint.Reason
		roleTag := constraint.RoleTag
		constraints = append(constraints, struct {
			Reason  *string `json:"reason,omitempty"`
			RoleTag *string `json:"role_tag,omitempty"`
		}{
			Reason:  optionalString(reason),
			RoleTag: optionalString(roleTag),
		})
	}
	body.UnmetConstraints = &constraints
	return body
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
