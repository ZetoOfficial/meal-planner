package httpapi

import (
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

func TestReadyzIsNotRegistered(t *testing.T) {
	handler := New(Deps{})

	req := httptest.NewRequest(http.MethodGet, "/readyz", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET /readyz status = %d, want %d", rec.Code, http.StatusNotFound)
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

func TestOpenAPIRouteRejectsUnsupportedMethod(t *testing.T) {
	handler := New(Deps{})

	req := httptest.NewRequest(http.MethodPatch, "/products/42", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("PATCH /products/42 status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
	if rec.Header().Get("Allow") == "" {
		t.Fatal("PATCH /products/42 response has no Allow header")
	}
}
