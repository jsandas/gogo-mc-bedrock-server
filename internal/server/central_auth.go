package server

import (
	"crypto/subtle"
	"net/http"
)

// authMiddleware wraps an http.HandlerFunc with authentication
func (s *CentralServer) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Try to get auth key from different sources
		var authKey string

		// Check Authorization Bearer token
		authHeader := r.Header.Get("Authorization")
		if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
			authKey = authHeader[7:]
		}

		// Check X-Auth-Key header
		if authKey == "" {
			authKey = r.Header.Get("X-Auth-Key")
		}

		// Check query parameter
		if authKey == "" {
			authKey = r.URL.Query().Get("auth")
		}

		if authKey == "" {
			http.Error(w, "Missing authentication key", http.StatusUnauthorized)
			return
		}

		// Use constant-time comparison to prevent timing attacks
		if subtle.ConstantTimeCompare([]byte(authKey), []byte(s.authKey)) != 1 {
			http.Error(w, "Invalid authentication key", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	}
}
