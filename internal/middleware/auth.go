package middleware

import (
	"net/http"

	"buh/internal/auth"
)

// RequireAuth wraps a handler to redirect unauthenticated requests to /login.
func RequireAuth(sm *auth.SessionManager, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := sm.Get(r); !ok {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		next.ServeHTTP(w, r)
	})
}
