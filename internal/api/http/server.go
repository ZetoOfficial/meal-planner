// Package httpapi is the HTTP delivery adapter: routing, auth, DTO mapping,
// and error mapping. Business logic belongs in use cases and domain packages.
package httpapi

import (
	"context"
	"encoding/json"
	"net/http"

	api "github.com/ZetoOfficial/meal-planner/internal/api/openapi"
	apperr "github.com/ZetoOfficial/meal-planner/internal/errors"
	"github.com/ZetoOfficial/meal-planner/internal/llm"
)

// Deps are HTTP-layer dependencies assembled by the composition root.
type Deps struct {
	APIToken   string
	ReadyCheck func(context.Context) error
	LLM        llm.Client
	// TODO: add use cases here as they are implemented.
}

// New returns the root HTTP handler.
func New(d Deps) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /livez", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		if d.ReadyCheck != nil {
			if err := d.ReadyCheck(r.Context()); err != nil {
				writeJSON(w, http.StatusServiceUnavailable, map[string]string{
					"status": "down",
					"error":  "database unreachable",
				})
				return
			}
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	apiMux := http.NewServeMux()
	strictHandler := api.NewStrictHandlerWithOptions(NewAPI(d), nil, api.StrictHTTPServerOptions{
		RequestErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			writeError(w, apperr.Validation(err.Error()))
		},
		ResponseErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			writeJSON(w, http.StatusInternalServerError, map[string]string{
				"error":   "internal_error",
				"message": "internal server error",
			})
		},
	})
	api.HandlerWithOptions(strictHandler, api.StdHTTPServerOptions{
		BaseRouter: apiMux,
		ErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			writeError(w, apperr.Validation(err.Error()))
		},
	})

	mux.Handle("/", withAuth(d.APIToken, apiMux))

	return mux
}

// withAuth enforces static owner-only X-API-Token auth.
func withAuth(token string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if token == "" {
			next.ServeHTTP(w, r)
			return
		}
		if r.Header.Get("X-API-Token") != token {
			writeError(w, apperr.Unauthorized("missing or invalid X-API-Token"))
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
