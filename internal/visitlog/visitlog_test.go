package visitlog

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"buh/internal/auth"
)

type fakeResolver struct {
	login string
	err   error
}

func (f fakeResolver) Resolve(context.Context, auth.Session) (string, error) { return f.login, f.err }

type fakeLogger struct {
	mu        sync.Mutex
	messages  []string
	started   chan struct{}
	called    chan struct{}
	release   chan struct{}
	startOnce sync.Once
}

func (f *fakeLogger) Printf(format string, args ...any) {
	message := fmt.Sprintf(format, args...)
	f.mu.Lock()
	f.messages = append(f.messages, message)
	started := f.started
	release := f.release
	called := f.called
	f.mu.Unlock()
	if started != nil {
		f.startOnce.Do(func() { close(started) })
	}
	if called != nil {
		called <- struct{}{}
	}
	if release != nil {
		<-release
	}
}

func (f *fakeLogger) messagesSnapshot() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.messages...)
}

func requestWithSession(t *testing.T, sessions *auth.SessionManager, path string, sess auth.Session) *http.Request {
	t.Helper()
	r := httptest.NewRequest(http.MethodGet, path, nil)
	w := httptest.NewRecorder()
	if err := sessions.Set(w, sess); err != nil {
		t.Fatal(err)
	}
	r.Header.Set("Cookie", w.Header().Get("Set-Cookie"))
	return r
}

func TestMiddlewareLogsExactMessageAndPreservesResponse(t *testing.T) {
	sessions := auth.NewSessionManager([]byte("12345678901234567890123456789012"), []byte("abcdefghijklmnopqrstuvwxzy123456"))
	started := make(chan struct{})
	logger := &fakeLogger{started: started}
	h := Middleware(sessions, fakeResolver{login: "a@example.test"}, logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte("unchanged"))
	}))
	r := requestWithSession(t, sessions, "/report?x=1", auth.Session{UserType: auth.UserTypeAccountant, UserID: "id"})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusTeapot || w.Body.String() != "unchanged" {
		t.Fatalf("response changed: %d %q", w.Code, w.Body.String())
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("visit was not logged")
	}
	if messages := logger.messagesSnapshot(); len(messages) != 1 || messages[0] != "#buh #visited a@example.test\nGET /report?x=1" {
		t.Fatalf("messages: %#v", messages)
	}
}

func TestMiddlewareRedactsAccessRequestQueryFromVisitLog(t *testing.T) {
	sessions := auth.NewSessionManager([]byte("12345678901234567890123456789012"), []byte("abcdefghijklmnopqrstuvwxzy123456"))
	started := make(chan struct{})
	logger := &fakeLogger{started: started}
	h := Middleware(sessions, fakeResolver{login: "a@example.test"}, logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))

	r := requestWithSession(t, sessions, "/a/access-request?email=submitted@example.test", auth.Session{UserType: auth.UserTypeAccountant, UserID: "id"})
	h.ServeHTTP(httptest.NewRecorder(), r)
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("visit was not logged")
	}

	if messages := logger.messagesSnapshot(); len(messages) != 1 || messages[0] != "#buh #visited a@example.test\nGET /a/access-request" {
		t.Fatalf("messages: %#v", messages)
	}
	if strings.Contains(logger.messagesSnapshot()[0], "submitted@example.test") {
		t.Fatal("visit log contains submitted access-request email")
	}
}

func TestMiddlewareDoesNotWaitForLogging(t *testing.T) {
	sessions := auth.NewSessionManager([]byte("12345678901234567890123456789012"), []byte("abcdefghijklmnopqrstuvwxzy123456"))
	started := make(chan struct{})
	release := make(chan struct{})
	logger := &fakeLogger{started: started, release: release}
	h := Middleware(sessions, fakeResolver{login: "a@example.test"}, logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	r := requestWithSession(t, sessions, "/report", auth.Session{UserType: auth.UserTypeAccountant, UserID: "id"})
	done := make(chan struct{})
	go func() {
		h.ServeHTTP(httptest.NewRecorder(), r)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("request waited for logging")
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("visit was not logged")
	}
	close(release)
}

func TestMiddlewareSkipsStaticAndUnauthenticatedRequests(t *testing.T) {
	sessions := auth.NewSessionManager([]byte("12345678901234567890123456789012"), []byte("abcdefghijklmnopqrstuvwxzy123456"))
	resolver := &countingResolver{done: make(chan struct{})}
	logger := &fakeLogger{called: make(chan struct{}, 2)}
	h := Middleware(sessions, resolver, logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	for _, path := range []string{"/static", "/static/", "/static/app.css"} {
		h.ServeHTTP(httptest.NewRecorder(), requestWithSession(t, sessions, path, auth.Session{UserType: auth.UserTypeAccountant, UserID: "id"}))
	}
	for _, r := range []*http.Request{
		httptest.NewRequest(http.MethodGet, "/public", nil),
	} {
		h.ServeHTTP(httptest.NewRecorder(), r)
	}
	bad := httptest.NewRequest(http.MethodGet, "/public", nil)
	bad.Header.Set("Cookie", "buh_session=invalid")
	h.ServeHTTP(httptest.NewRecorder(), bad)

	// Supported authenticated requests are still logged; the assertions below
	// make sure the filtering does not accidentally suppress them as well.
	for range 2 {
		h.ServeHTTP(httptest.NewRecorder(), requestWithSession(t, sessions, "/private", auth.Session{UserType: auth.UserTypeAccountant, UserID: "id"}))
	}
	select {
	case <-resolver.done:
	case <-time.After(time.Second):
		t.Fatal("supported sessions were not resolved")
	}
	for range 2 {
		select {
		case <-logger.called:
		case <-time.After(time.Second):
			t.Fatal("supported session was not logged")
		}
	}
	if resolver.callCount() != 2 || len(logger.messagesSnapshot()) != 2 {
		t.Fatalf("resolver calls=%d messages=%d", resolver.callCount(), len(logger.messagesSnapshot()))
	}
}

func TestMiddlewareLogsIdentityResolutionFailure(t *testing.T) {
	sessions := auth.NewSessionManager([]byte("12345678901234567890123456789012"), []byte("abcdefghijklmnopqrstuvwxzy123456"))
	started := make(chan struct{})
	logger := &fakeLogger{started: started}
	h := Middleware(sessions, fakeResolver{err: fmt.Errorf("database unavailable")}, logger)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	h.ServeHTTP(httptest.NewRecorder(), requestWithSession(t, sessions, "/report", auth.Session{UserType: auth.UserTypeAccountant, UserID: "id"}))

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("identity failure was not logged")
	}
	messages := logger.messagesSnapshot()
	if len(messages) != 1 || !strings.Contains(messages[0], "visit logging identity resolution failed: database unavailable") {
		t.Fatalf("messages: %#v", messages)
	}
}

type countingResolver struct {
	mu    sync.Mutex
	calls int
	done  chan struct{}
}

func (r *countingResolver) Resolve(context.Context, auth.Session) (string, error) {
	r.mu.Lock()
	r.calls++
	calls := r.calls
	done := r.done
	r.mu.Unlock()
	if calls == 2 && done != nil {
		close(done)
	}
	return "user@example.test", nil
}

func (r *countingResolver) callCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.calls
}
