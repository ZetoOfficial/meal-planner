package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	apperr "github.com/ZetoOfficial/meal-planner/internal/errors"
)

func TestWriteErrorMapsApplicationErrorClasses(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{
			name:   "validation",
			err:    apperr.Validation("bad input", "name is required"),
			status: http.StatusUnprocessableEntity,
			code:   string(apperr.CodeValidation),
		},
		{
			name:   "not found",
			err:    apperr.NotFound("product"),
			status: http.StatusNotFound,
			code:   string(apperr.CodeNotFound),
		},
		{
			name:   "conflict",
			err:    apperr.Conflict("recipe already permanent"),
			status: http.StatusConflict,
			code:   string(apperr.CodeConflict),
		},
		{
			name:   "unauthorized",
			err:    apperr.Unauthorized("missing token"),
			status: http.StatusUnauthorized,
			code:   string(apperr.CodeUnauthorized),
		},
		{
			name:   "unavailable",
			err:    apperr.Unavailable("generation unavailable", fmt.Errorf("provider timeout")),
			status: http.StatusServiceUnavailable,
			code:   string(apperr.CodeUnavailable),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()

			writeError(rec, tt.err)

			if rec.Code != tt.status {
				t.Fatalf("status = %d, want %d", rec.Code, tt.status)
			}

			var body map[string]any
			if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
				t.Fatalf("decode response body: %v", err)
			}
			if body["error"] != tt.code {
				t.Fatalf("error = %q, want %q", body["error"], tt.code)
			}
			if body["message"] == "" {
				t.Fatal("message is empty")
			}
		})
	}
}

func TestWriteErrorMapsImpossiblePlanDiagnostics(t *testing.T) {
	rec := httptest.NewRecorder()

	writeError(rec, apperr.ImpossiblePlan(
		"plan cannot satisfy macro constraints",
		apperr.UnmetConstraint{RoleTag: "protein:lean", Reason: "not enough usable products"},
	))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}

	var body struct {
		Error            string `json:"error"`
		Message          string `json:"message"`
		UnmetConstraints []struct {
			RoleTag string `json:"role_tag"`
			Reason  string `json:"reason"`
		} `json:"unmet_constraints"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	if body.Error != string(apperr.CodeImpossiblePlan) {
		t.Fatalf("error = %q, want %q", body.Error, apperr.CodeImpossiblePlan)
	}
	if len(body.UnmetConstraints) != 1 {
		t.Fatalf("unmet constraints count = %d, want 1", len(body.UnmetConstraints))
	}
	if body.UnmetConstraints[0].RoleTag != "protein:lean" {
		t.Fatalf("role_tag = %q, want %q", body.UnmetConstraints[0].RoleTag, "protein:lean")
	}
}

func TestWriteErrorMapsWrappedApplicationErrors(t *testing.T) {
	rec := httptest.NewRecorder()

	writeError(rec, fmt.Errorf("load product: %w", apperr.NotFound("product")))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}

	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	if body["error"] != string(apperr.CodeNotFound) {
		t.Fatalf("error = %q, want %q", body["error"], apperr.CodeNotFound)
	}
}
