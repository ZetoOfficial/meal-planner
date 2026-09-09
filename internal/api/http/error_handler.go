package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/ZetoOfficial/meal-planner/api/sdk/go/model"
	apperr "github.com/ZetoOfficial/meal-planner/internal/errors"
)

func requestError(w http.ResponseWriter, r *http.Request, err error, status int) {
	response := model.Error{
		Error:   string(apperr.CodeValidation),
		Message: "invalid request",
		Details: &[]string{err.Error()},
	}
	if status >= http.StatusInternalServerError {
		slog.ErrorContext(r.Context(), "request validation failed", slog.Any("error", err))
		status = http.StatusInternalServerError
		response = model.Error{
			Error:   string(apperr.CodeInternal),
			Message: "internal server error",
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if encodeErr := json.NewEncoder(w).Encode(response); encodeErr != nil {
		slog.ErrorContext(r.Context(), "write error response", slog.Any("error", encodeErr))
	}
}

func applicationError(w http.ResponseWriter, r *http.Request, err error) {
	status := http.StatusInternalServerError
	response := model.Error{
		Error:   string(apperr.CodeInternal),
		Message: "internal server error",
	}

	var app *apperr.Error
	if errors.As(err, &app) {
		switch app.Code {
		case apperr.CodeValidation:
			status = http.StatusUnprocessableEntity
		case apperr.CodeNotFound:
			status = http.StatusNotFound
		case apperr.CodeConflict:
			status = http.StatusConflict
		case apperr.CodeUnauthorized:
			status = http.StatusUnauthorized
		case apperr.CodeUnavailable:
			status = http.StatusServiceUnavailable
		}
		if status != http.StatusInternalServerError {
			var details *[]string
			if len(app.Details) > 0 {
				details = &app.Details
			}
			response = model.Error{
				Error:   string(app.Code),
				Message: app.Error(),
				Details: details,
			}
		}
	}

	if status >= http.StatusInternalServerError {
		slog.ErrorContext(r.Context(), "request failed", slog.Any("error", err))
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if encodeErr := json.NewEncoder(w).Encode(response); encodeErr != nil {
		slog.ErrorContext(r.Context(), "write error response", slog.Any("error", encodeErr))
	}
}
