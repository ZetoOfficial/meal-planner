package httpapi

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/getkin/kin-openapi/routers"
	nethttpmiddleware "github.com/oapi-codegen/nethttp-middleware"

	"github.com/ZetoOfficial/meal-planner/internal/api/openapi"
	apperr "github.com/ZetoOfficial/meal-planner/internal/errors"
)

type Deps struct {
	APIToken string
	Catalog  CatalogUseCases
	Eater    EaterUseCases
	Logger   *slog.Logger
}

func New(d Deps) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /livez", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	log := d.Logger
	if log == nil {
		log = slog.Default()
	}
	httpErrors := errorHandler{log: log}
	swagger, err := openapi.GetSpec()
	if err != nil {
		panic(fmt.Errorf("load OpenAPI spec: %w", err))
	}
	swagger.Servers = nil
	swagger.Security = nil

	strict := openapi.NewStrictHandlerWithOptions(newAPI(d.Catalog, d.Eater), nil, openapi.StrictHTTPServerOptions{
		RequestErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			httpErrors.request(w, r, err, http.StatusBadRequest)
		},
		ResponseErrorHandlerFunc: httpErrors.application,
	})

	apiMux := http.NewServeMux()
	openapi.HandlerWithOptions(strict, openapi.StdHTTPServerOptions{
		BaseRouter: apiMux,
		ErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			httpErrors.request(w, r, err, http.StatusBadRequest)
		},
	})

	validated := nethttpmiddleware.OapiRequestValidatorWithOptions(swagger, &nethttpmiddleware.Options{
		ErrorHandlerWithOpts: func(_ context.Context, err error, w http.ResponseWriter, r *http.Request, opts nethttpmiddleware.ErrorHandlerOpts) {
			if errors.Is(err, routers.ErrMethodNotAllowed) {
				apiMux.ServeHTTP(w, r)
				return
			}
			httpErrors.request(w, r, err, opts.StatusCode)
		},
	})(apiMux)

	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if d.APIToken != "" && r.Header.Get("X-API-Token") != d.APIToken {
			httpErrors.application(w, r, apperr.Unauthorized("missing or invalid X-API-Token"))
			return
		}
		validated.ServeHTTP(w, r)
	}))

	return mux
}
