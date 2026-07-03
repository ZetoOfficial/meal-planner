// Package httpapi is the HTTP delivery adapter: routing, auth, DTO mapping,
// and error mapping. Business logic belongs in use cases and domain packages.
package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ZetoOfficial/meal-planner/internal/api/openapi"
	"github.com/ZetoOfficial/meal-planner/internal/llm"
)

// Deps are HTTP-layer dependencies assembled by the composition root.
type Deps struct {
	APIToken string
	DB       *pgxpool.Pool
	LLM      llm.Client
	// TODO: add use cases here as they are implemented.
}

// New returns the root HTTP handler.
func New(d Deps) http.Handler {
	mux := http.NewServeMux()

	// Healthcheck stays unauthenticated for docker/k8s probes.
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		if d.DB != nil {
			if err := d.DB.Ping(r.Context()); err != nil {
				writeJSON(w, http.StatusServiceUnavailable, map[string]string{
					"status": "down",
					"error":  "database unreachable",
				})
				return
			}
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	openapi.HandlerWithOptions(NewAPI(d), openapi.StdHTTPServerOptions{
		BaseRouter: mux,
		ErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error":   "bad_request",
				"message": err.Error(),
			})
		},
	})

	return withAuth(d.APIToken, mux)
}

// withAuth enforces static owner-only X-API-Token auth. /healthz is public.
func withAuth(token string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if token == "" || r.URL.Path == "/healthz" {
			next.ServeHTTP(w, r)
			return
		}
		if r.Header.Get("X-API-Token") != token {
			writeJSON(w, http.StatusUnauthorized, map[string]string{
				"error":   "unauthorized",
				"message": "missing or invalid X-API-Token",
			})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}
