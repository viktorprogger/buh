package accountant

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"

	"buh/internal/accessrequest"
	"buh/internal/auth"
	"buh/internal/web/shared"
)

func newAccessRequestHandler(t *testing.T) (*Handler, string, uuid.UUID) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "requests.csv")
	service := accessrequest.NewService(accessrequest.NewStore(path), nil)
	sessions := auth.NewSessionManager(
		[]byte("test-hash-key-32-bytes-xxxxxxxxx"),
		[]byte("test-blck-key-32-bytes-xxxxxxxxx"),
	)
	h := NewHandler(sessions, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, shared.Templates{}, service)
	accountantID := uuid.New()
	return h, path, accountantID
}

func authenticatedAccessRequest(t *testing.T, h *Handler, accountantID uuid.UUID, target string, body string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, target, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	cookieResponse := httptest.NewRecorder()
	if err := h.sessions.Set(cookieResponse, auth.Session{UserType: auth.UserTypeAccountant, UserID: accountantID.String()}); err != nil {
		t.Fatal(err)
	}
	r.AddCookie(cookieResponse.Result().Cookies()[0])
	w := httptest.NewRecorder()
	h.handleAccessRequest(w, r)
	return w
}

func TestShouldShowAccessRequestBannerOnlyForHardcodedEmail(t *testing.T) {
	if !shouldShowAccessRequestBanner("viktorprogger@gmail.com") {
		t.Fatal("expected the hardcoded accountant email to show the banner")
	}
	if shouldShowAccessRequestBanner("someone@example.com") {
		t.Fatal("unexpected banner for another email")
	}
}

func TestHandleAccessRequestRejectsQueryOnlyEmail(t *testing.T) {
	h, path, accountantID := newAccessRequestHandler(t)
	w := authenticatedAccessRequest(t, h, accountantID, "/a/access-request?email=query@example.com", "")

	if w.Code != http.StatusBadRequest {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusBadRequest)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("query-only email was persisted: %v", err)
	}
}

func TestHandleAccessRequestAcceptsBodyEmail(t *testing.T) {
	h, path, accountantID := newAccessRequestHandler(t)
	w := authenticatedAccessRequest(t, h, accountantID, "/a/access-request", url.Values{"email": {"body@example.com"}}.Encode())

	if w.Code != http.StatusSeeOther {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusSeeOther)
	}
	if got := w.Header().Get("Location"); got != "/a/?access_submitted=1" {
		t.Fatalf("got redirect %q", got)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := "email,sent\nbody@example.com,false\n"; string(contents) != want {
		t.Fatalf("got persisted data %q, want %q", contents, want)
	}
}
