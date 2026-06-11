package middleware

import (
	"net/http"
	"strings"
)

func CORS(allowedOrigins string) func(http.Handler) http.Handler {
	var list []string
	for _, o := range strings.Split(allowedOrigins, ",") {
		if o = strings.TrimSpace(o); o != "" {
			list = append(list, o)
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			allowed := matchOrigin(origin, list)
			if allowed != "" {
				w.Header().Set("Access-Control-Allow-Origin", allowed)
			}
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization,Content-Type,ngrok-skip-browser-warning")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func matchOrigin(origin string, allowed []string) string {
	if len(allowed) == 0 {
		return "*"
	}
	for _, o := range allowed {
		if o == "*" || o == origin {
			return o
		}
	}
	return ""
}
