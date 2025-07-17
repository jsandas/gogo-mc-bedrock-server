package server

import (
	"crypto/subtle"
	"net/http"
)

// authMiddleware wraps an http.HandlerFunc with authentication
func (s *CentralServer) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Try to get auth key from header or query parameter
		authKey := r.Header.Get("X-Auth-Key")
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
