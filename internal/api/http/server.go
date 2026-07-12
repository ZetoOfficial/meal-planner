package httpapi

import (
	"net/http"

	"github.com/ZetoOfficial/meal-planner/internal/api/http/middleware"
)

type Deps struct {
	APIToken string
}

func New(d Deps) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /livez", func(w http.ResponseWriter, r *http.Request) {
		respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	mux.Handle("/", middleware.RequireAPIToken(d.APIToken, http.NotFoundHandler(), writeError))

	return mux
}
