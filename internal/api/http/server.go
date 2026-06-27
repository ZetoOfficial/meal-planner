// Package httpapi — delivery-адаптер: роутинг, auth, маппинг DTO ↔ usecase (ARCHITECTURE §9).
// В хендлерах нет solver/SQL/quality/LLM-логики — только транспорт.
package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ZetoOfficial/meal-planner/internal/llm"
)

// Deps — зависимости HTTP-слоя. Собираются в composition root (internal/app),
// сюда передаётся уже готовый набор, без импорта app (избегаем цикла).
type Deps struct {
	APIToken string
	DB       *pgxpool.Pool
	LLM      llm.Client
	// TODO: use cases (catalog/eater/recipe/planning/...) монтируются сюда.
}

// New возвращает корневой http.Handler.
// Сгенерированные oapi-codegen хендлеры монтируются здесь после `make generate`:
//
//	openapi.HandlerFromMux(apiImpl, mux)  // см. internal/api/openapi/generated.go
func New(d Deps) http.Handler {
	mux := http.NewServeMux()

	// healthcheck — без авторизации (для docker healthcheck / k8s probe).
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		if err := d.DB.Ping(r.Context()); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{
				"status": "down", "error": "database unreachable",
			})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// Защищённый API монтируется под auth-middleware.
	return withAuth(d.APIToken, mux)
}

// withAuth — статический X-API-Token (owner-only, SPEC §9). /healthz пропускаем.
func withAuth(token string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if token == "" || r.URL.Path == "/healthz" {
			next.ServeHTTP(w, r)
			return
		}
		if r.Header.Get("X-API-Token") != token {
			writeJSON(w, http.StatusUnauthorized, map[string]string{
				"error": "unauthorized", "message": "missing or invalid X-API-Token",
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
