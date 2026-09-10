package httpapi

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"net/http"

	"github.com/getkin/kin-openapi/routers"
	nethttpmiddleware "github.com/oapi-codegen/nethttp-middleware"

	"github.com/ZetoOfficial/meal-planner/api/sdk/go/api"
	"github.com/ZetoOfficial/meal-planner/internal/apperr"
)

type Deps struct {
	APIToken string
	Catalog  CatalogUseCases
	Eater    EaterUseCases
	Recipe   RecipeUseCases
	Planning PlanningUseCases
	Shopping ShoppingUseCases
	Reality  RealityUseCases
}

func New(d Deps) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /livez", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	swagger, err := api.GetSpec()
	if err != nil {
		panic(fmt.Errorf("load OpenAPI spec: %w", err))
	}
	swagger.Servers = nil
	swagger.Security = nil

	strict := api.NewStrictHandlerWithOptions(newServer(d.Catalog, d.Eater, d.Recipe, d.Planning, d.Shopping, d.Reality), nil, api.StrictHTTPServerOptions{
		RequestErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			requestError(w, r, err, http.StatusBadRequest)
		},
		ResponseErrorHandlerFunc: applicationError,
	})

	apiMux := http.NewServeMux()
	api.HandlerWithOptions(strict, api.StdHTTPServerOptions{
		BaseRouter: apiMux,
		ErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			requestError(w, r, err, http.StatusBadRequest)
		},
	})

	validated := nethttpmiddleware.OapiRequestValidatorWithOptions(swagger, &nethttpmiddleware.Options{
		ErrorHandlerWithOpts: func(_ context.Context, err error, w http.ResponseWriter, r *http.Request, opts nethttpmiddleware.ErrorHandlerOpts) {
			if errors.Is(err, routers.ErrMethodNotAllowed) {
				apiMux.ServeHTTP(w, r)
				return
			}
			requestError(w, r, err, opts.StatusCode)
		},
	})(apiMux)

	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !authorized(d.APIToken, r.Header.Get("X-API-Token")) {
			applicationError(w, r, apperr.Unauthorized("missing or invalid X-API-Token"))
			return
		}
		validated.ServeHTTP(w, r)
	}))

	return withRequestID(recoverPanics(logRequests(mux)))
}

// authorized compares in constant time so the response latency cannot be used
// to recover the token byte by byte. An empty configured token disables auth.
func authorized(want, got string) bool {
	if want == "" {
		return true
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}
