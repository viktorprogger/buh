package web_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"buh/internal/accountant"
	"buh/internal/auth"
	"buh/internal/entrepreneuruser"
	"buh/internal/web"
)

func newHandler(t *testing.T) http.Handler {
	t.Helper()
	// Repos are constructed with nil DB; constructors only store the pointer.
	// Handlers will fail on actual DB calls, but routing + template rendering
	// of redirect/error responses does not hit the DB.
	accts := accountant.NewRepo(nil)
	eUsers := entrepreneuruser.NewRepo(nil)
	sessions := auth.NewSessionManager(
		[]byte("test-hash-key-32-bytes-xxxxxxxxx"),
		[]byte("test-blck-key-32-bytes-xxxxxxxxx"),
	)
	return web.NewHandler(accts, eUsers, sessions, nil)
}

func get(h http.Handler, path string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodGet, path, nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

// TestPublicRoutes checks that unauthenticated requests to public endpoints
// return the expected status code without panicking.
func TestPublicRoutes(t *testing.T) {
	h := newHandler(t)

	cases := []struct {
		path string
		want int
	}{
		{"/login", http.StatusOK},
		{"/logout", http.StatusFound},
		{"/privacy", http.StatusOK},
		{"/terms", http.StatusOK},
		{"/e/register", http.StatusOK},
		// Root redirect
		{"/", http.StatusFound},
		// Unknown path → 404
		{"/does-not-exist", http.StatusNotFound},
	}
	// Note: /invite/{token} is intentionally omitted — it queries the DB
	// on every request (before auth), so it cannot be tested without a real DB.

	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			w := get(h, tc.path)
			if w.Code != tc.want {
				t.Errorf("GET %s: got %d, want %d", tc.path, w.Code, tc.want)
			}
		})
	}
}

// TestProtectedAccountantRoutes checks that unauthenticated requests to /a/*
// are redirected to /login (handled by middleware.RequireAccountant).
func TestProtectedAccountantRoutes(t *testing.T) {
	h := newHandler(t)

	paths := []string{
		"/a/",
		"/a/entrepreneurs/new",
		"/a/entrepreneurs/00000000-0000-0000-0000-000000000001",
		"/a/slips/00000000-0000-0000-0000-000000000001",
	}

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			w := get(h, path)
			if w.Code != http.StatusFound {
				t.Errorf("GET %s: got %d, want 302 (redirect to login)", path, w.Code)
			}
		})
	}
}

// TestProtectedEntrepreneurRoutes checks that unauthenticated requests to /e/*
// are redirected to /login (handled by middleware.RequireEntrepreneur).
func TestProtectedEntrepreneurRoutes(t *testing.T) {
	h := newHandler(t)

	paths := []string{
		"/e/",
		"/e/clients",
		"/e/bank-accounts",
		"/e/invoices",
		"/e/kpo/2026",
		"/e/invite-accountant",
	}

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			w := get(h, path)
			if w.Code != http.StatusFound {
				t.Errorf("GET %s: got %d, want 302 (redirect to login)", path, w.Code)
			}
		})
	}
}
