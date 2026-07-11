package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLivezDoesNotRequireAPIToken(t *testing.T) {
	handler := New(Deps{APIToken: "secret"})

	req := httptest.NewRequest(http.MethodGet, "/livez", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /livez status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestReadyzDoesNotRequireAPIToken(t *testing.T) {
	handler := New(Deps{APIToken: "secret"})

	req := httptest.NewRequest(http.MethodGet, "/readyz", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /readyz status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestReadyzReturnsServiceUnavailableWhenReadinessFails(t *testing.T) {
	handler := New(Deps{
		APIToken: "secret",
		Readiness: func() error {
			return errors.New("database unavailable")
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/readyz", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("GET /readyz status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

func TestOpenAPIRouteRequiresTokenWhenConfigured(t *testing.T) {
	handler := New(Deps{APIToken: "secret"})

	req := httptest.NewRequest(http.MethodGet, "/eaters", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("GET /eaters status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestOpenAPIRouteReturnsNotImplementedWithValidToken(t *testing.T) {
	handler := New(Deps{APIToken: "secret"})

	req := httptest.NewRequest(http.MethodGet, "/eaters", http.NoBody)
	req.Header.Set("X-API-Token", "secret")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("GET /eaters status = %d, want %d", rec.Code, http.StatusNotImplemented)
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	if body["error"] != "not_implemented" {
		t.Fatalf("error = %q, want %q", body["error"], "not_implemented")
	}
	if body["message"] == "" {
		t.Fatal("message is empty")
	}
}

func TestOpenAPIValidationErrorsMapToUnprocessableEntity(t *testing.T) {
	handler := New(Deps{APIToken: "secret"})

	req := httptest.NewRequest(http.MethodGet, "/analytics/macros", http.NoBody)
	req.Header.Set("X-API-Token", "secret")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("GET /analytics/macros status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	if body["error"] != "validation_failed" {
		t.Fatalf("error = %q, want %q", body["error"], "validation_failed")
	}
}
