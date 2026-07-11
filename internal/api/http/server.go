package httpapi

import (
	"encoding/json"
	"net/http"

	api "github.com/ZetoOfficial/meal-planner/internal/api/openapi"
	apperr "github.com/ZetoOfficial/meal-planner/internal/errors"
	"github.com/ZetoOfficial/meal-planner/internal/llm"
)

type Deps struct {
	APIToken  string
	Readiness func() error
	LLM       llm.Client
}

func New(d Deps) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /livez", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		if d.Readiness != nil {
			if err := d.Readiness(); err != nil {
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
