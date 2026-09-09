package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ZetoOfficial/meal-planner/api/sdk/go/model"
	apperr "github.com/ZetoOfficial/meal-planner/internal/errors"
)

func TestApplicationErrorMapsErrors(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{name: "validation", err: apperr.Validation("bad input"), status: http.StatusUnprocessableEntity, code: string(apperr.CodeValidation)},
		{name: "not found", err: fmt.Errorf("load product: %w", apperr.NotFound("product")), status: http.StatusNotFound, code: string(apperr.CodeNotFound)},
		{name: "conflict", err: apperr.Conflict("duplicate"), status: http.StatusConflict, code: string(apperr.CodeConflict)},
		{name: "unauthorized", err: apperr.Unauthorized("missing token"), status: http.StatusUnauthorized, code: string(apperr.CodeUnauthorized)},
		{name: "unavailable", err: apperr.Unavailable("provider unavailable", errors.New("timeout")), status: http.StatusServiceUnavailable, code: string(apperr.CodeUnavailable)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)

			applicationError(rec, req, test.err)

			if rec.Code != test.status {
				t.Fatalf("status = %d, want %d", rec.Code, test.status)
			}
			var body model.Error
			if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
				t.Fatalf("decode response body: %v", err)
			}
			if body.Error != test.code {
				t.Fatalf("error = %q, want %q", body.Error, test.code)
			}
		})
	}
}

func TestApplicationErrorDoesNotExposeUnknownErrors(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)

	applicationError(rec, req, errors.New("database password leaked"))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	var body model.Error
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	if body.Error != string(apperr.CodeInternal) || body.Message != "internal server error" {
		t.Fatalf("body = %#v", body)
	}
}
