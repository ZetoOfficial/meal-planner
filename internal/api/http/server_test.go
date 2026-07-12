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

func TestFutureAPIRouteReturnsStructuredNotImplementedWithValidToken(t *testing.T) {
	handler := New(Deps{APIToken: "secret"})

	req := httptest.NewRequest(http.MethodGet, "/analytics/macros?eaterId=1&from=2026-01-01&to=2026-01-02", http.NoBody)
	req.Header.Set("X-API-Token", "secret")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("GET /analytics/macros status = %d, want %d", rec.Code, http.StatusNotImplemented)
	}
}
