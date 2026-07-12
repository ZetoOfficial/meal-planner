package httpapi

import (
	"net/http"

	"github.com/ZetoOfficial/meal-planner/internal/api/http/middleware"
	"github.com/ZetoOfficial/meal-planner/internal/api/openapi"
	apperr "github.com/ZetoOfficial/meal-planner/internal/errors"
)

type Deps struct {
	APIToken string
	Catalog  CatalogUseCases
	Eater    EaterUseCases
}

func New(d Deps) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /livez", func(w http.ResponseWriter, r *http.Request) {
		respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	apiMux := http.NewServeMux()
	openapi.HandlerWithOptions(newAPI(d.Catalog, d.Eater), openapi.StdHTTPServerOptions{
		BaseRouter: apiMux,
		ErrorHandlerFunc: func(w http.ResponseWriter, _ *http.Request, err error) {
			writeError(w, apperr.Validation("invalid request parameter", err.Error()))
		},
	})
	mux.Handle("/", middleware.RequireAPIToken(d.APIToken, apiMux, writeError))

	return mux
}
