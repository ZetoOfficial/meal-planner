package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthzDoesNotRequireAPIToken(t *testing.T) {
	handler := New(Deps{APIToken: "secret"})

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /healthz status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestOpenAPIRouteRequiresTokenWhenConfigured(t *testing.T) {
	handler := New(Deps{APIToken: "secret"})

	req := httptest.NewRequest(http.MethodGet, "/eaters", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("GET /eaters status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestOpenAPIRouteReturnsNotImplementedWithValidToken(t *testing.T) {
	handler := New(Deps{APIToken: "secret"})

	req := httptest.NewRequest(http.MethodGet, "/eaters", nil)
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
