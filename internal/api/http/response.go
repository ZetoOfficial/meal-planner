package httpapi

import (
	"encoding/json"
	"net/http"
)

func respondJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeOpenAPIResponse(w http.ResponseWriter, visit func(http.ResponseWriter) error) {
	if err := visit(w); err != nil {
		writeError(w, err)
	}
}
