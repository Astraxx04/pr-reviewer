package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"pr-reviewer/internal/http/middleware"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func getUser(r *http.Request) *middleware.AuthUser {
	return middleware.UserFromCtx(r.Context())
}

func pathID(r *http.Request, key string) (uint, bool) {
	v := r.PathValue(key)
	n, err := strconv.ParseUint(v, 10, 64)
	if err != nil {
		return 0, false
	}
	return uint(n), true
}

func queryInt(r *http.Request, key string, def int) int {
	v := r.URL.Query().Get(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}
