package middleware

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/kube-dashboard/kube_dashboard/internal/config"
)

func Auth(next http.Handler, cfg config.Config) http.Handler {
	if len(cfg.AllowedEmails) == 0 {
		return next
	}

	allowed := map[string]struct{}{}
	for _, email := range cfg.AllowedEmails {
		allowed[strings.ToLower(strings.TrimSpace(email))] = struct{}{}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Keep health endpoints unauthenticated for probes.
		if r.URL.Path == "/health" {
			next.ServeHTTP(w, r)
			return
		}

		if r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}

		headerName := strings.TrimSpace(cfg.EmailHeader)
		if headerName == "" {
			headerName = "X-Forwarded-Email"
		}

		email := strings.TrimSpace(r.Header.Get(headerName))
		if email == "" {
			email = strings.TrimSpace(r.Header.Get("X-User-Email"))
		}
		if email == "" {
			auth := r.Header.Get("Authorization")
			if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
				email = strings.TrimSpace(auth[7:])
			}
		}

		if email == "" {
			writeJSONError(w, http.StatusUnauthorized, "email authorization header is required")
			return
		}

		if _, ok := allowed[strings.ToLower(email)]; !ok {
			writeJSONError(w, http.StatusForbidden, "email not allowed")
			return
		}

		next.ServeHTTP(w, r)
	})
}

func writeJSONError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
