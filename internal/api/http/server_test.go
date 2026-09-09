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

func TestPanicInAHandlerBecomesAJSONInternalError(t *testing.T) {
	handler := recoverPanics(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("handler exploded")
	}))

	req := httptest.NewRequest(http.MethodGet, "/products", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500 instead of a dropped connection", rec.Code)
	}
	if rec.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("content type = %q, want a JSON error body", rec.Header().Get("Content-Type"))
	}
}

func TestResponseCarriesARequestID(t *testing.T) {
	handler := New(Deps{})

	req := httptest.NewRequest(http.MethodGet, "/livez", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Header().Get(requestIDHeader) == "" {
		t.Fatalf("response has no %s header", requestIDHeader)
	}
}

func TestRequestIDOfTheCallerIsKept(t *testing.T) {
	handler := New(Deps{})

	req := httptest.NewRequest(http.MethodGet, "/livez", http.NoBody)
	req.Header.Set(requestIDHeader, "abc-123")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get(requestIDHeader); got != "abc-123" {
		t.Fatalf("%s = %q, want the caller's id echoed", requestIDHeader, got)
	}
}
