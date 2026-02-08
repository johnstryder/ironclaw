package gateway

import (
	"net/http"
	"strings"
)

// BearerAuth returns middleware that, when token is non-empty, requires
// Authorization: Bearer <token>. Missing or incorrect token returns 401 Unauthorized.
// When token is empty, the next handler is called without checking.
func BearerAuth(token string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if token == "" {
				next.ServeHTTP(w, r)
				return
			}
			auth := r.Header.Get("Authorization")
			if auth == "" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			const prefix = "Bearer "
			if !strings.HasPrefix(auth, prefix) {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			got := strings.TrimSpace(auth[len(prefix):])
			if got != token {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
