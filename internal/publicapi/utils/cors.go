package utils

import "net/http"

// WithCORS wraps next with Access-Control-Allow-* for listed origins and handles OPTIONS preflight.
func WithCORS(allowedOrigins []string, next http.Handler) http.Handler {
	allow := make(map[string]struct{}, len(allowedOrigins))
	for _, o := range allowedOrigins {
		if o == "" {
			continue
		}
		allow[o] = struct{}{}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			if _, ok := allow[origin]; ok {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Vary", "Origin")
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Idempotency-Key")
			}
		}
		if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
