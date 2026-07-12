package middleware

import (
	"net/http"

	apperr "github.com/ZetoOfficial/meal-planner/internal/errors"
)

func RequireAPIToken(token string, next http.Handler, onError func(http.ResponseWriter, error)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if token == "" {
			next.ServeHTTP(w, r)
			return
		}
		if r.Header.Get("X-API-Token") != token {
			onError(w, apperr.Unauthorized("missing or invalid X-API-Token"))
			return
		}
		next.ServeHTTP(w, r)
	})
}
