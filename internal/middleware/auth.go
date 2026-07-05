package middleware

import (
	"net/http"

	"buh/internal/auth"
)

// RequireAccountant redirects to /login if the session is not an accountant session.
func RequireAccountant(sm *auth.SessionManager, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess, ok := sm.Get(r)
		if !ok || sess.UserType != auth.UserTypeAccountant {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireEntrepreneur redirects to /login if the session is not an entrepreneur session.
func RequireEntrepreneur(sm *auth.SessionManager, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess, ok := sm.Get(r)
		if !ok || sess.UserType != auth.UserTypeEntrepreneur {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		next.ServeHTTP(w, r)
	})
}
