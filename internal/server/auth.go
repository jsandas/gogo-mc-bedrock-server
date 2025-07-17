package server

import (
	"crypto/subtle"
	"errors"
	"net/http"
)

var (
	ErrMissingAuthKey = errors.New("missing X-Auth-Key header")
	ErrInvalidAuthKey = errors.New("invalid authentication key")
)

// authMiddleware checks for the presence and validity of the pre-shared key
func (s *Server) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Skip auth check for the index page
		if r.URL.Path == "/" {
			next.ServeHTTP(w, r)
			return
		}

		// Try to get auth key from header or query parameter
		authKey := r.Header.Get("X-Auth-Key")
		if authKey == "" {
			authKey = r.URL.Query().Get("auth")
		}

		if authKey == "" {
			http.Error(w, ErrMissingAuthKey.Error(), http.StatusUnauthorized)
			return
		}

		// Use constant-time comparison to prevent timing attacks
		if subtle.ConstantTimeCompare([]byte(authKey), []byte(s.authKey)) != 1 {
			http.Error(w, ErrInvalidAuthKey.Error(), http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	}
}
