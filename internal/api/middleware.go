package api

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

// SecurityHeadersMiddleware adds security headers to responses
func SecurityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Content Security Policy
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-eval'; style-src 'self' 'unsafe-inline'; img-src 'self' data:;")
		// Strict Transport Security (HSTS) - 1 year
		if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		// X-Frame-Options
		w.Header().Set("X-Frame-Options", "DENY")
		// X-Content-Type-Options
		w.Header().Set("X-Content-Type-Options", "nosniff")
		// Referrer Policy
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

		next.ServeHTTP(w, r)
	})
}

// AuthMiddleware checks for API token
func AuthMiddleware(token string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only authenticate API routes
		if !strings.HasPrefix(r.URL.Path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}

		// If token is not set, skip auth
		if token == "" {
			next.ServeHTTP(w, r)
			return
		}

		// Check Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Check Bearer token
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" || subtle.ConstantTimeCompare([]byte(parts[1]), []byte(token)) != 1 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}
