package accessrequest

import (
	"bytes"
	"context"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeClient struct {
	mu     sync.Mutex
	emails []string
	err    error
}

func (f *fakeClient) Send(_ context.Context, email string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.emails = append(f.emails, email)
	return f.err
}

func TestStoreCreatesTwoColumnCSVAndKeepsDuplicates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "requests.csv")
	s := NewStore(path)
	if err := s.Append(" first@example.com "); err != nil {
		t.Fatal(err)
	}
	if err := s.Append("first@example.com"); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := "email,sent\nfirst@example.com,false\nfirst@example.com,false\n"
	if string(b) != want {
		t.Fatalf("got %q", b)
	}
}
func TestInvalidEmailDoesNotWrite(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "x.csv"))
	if err := s.Append("not email"); err == nil {
		t.Fatal("expected validation error")
	}
	if _, err := os.Stat(s.path); !os.IsNotExist(err) {
		t.Fatalf("file was created: %v", err)
	}
}

func TestMissingParentDoesNotGetCreated(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "missing")
	s := NewStore(filepath.Join(parent, "requests.csv"))
	if err := s.Append("a@example.com"); err == nil {
		t.Fatal("expected unavailable storage error")
	}
	if _, err := os.Stat(parent); !os.IsNotExist(err) {
		t.Fatalf("storage parent was created: %v", err)
	}
}

func TestMalformedExistingEmailIsRejectedBeforeDelivery(t *testing.T) {
	path := filepath.Join(t.TempDir(), "requests.csv")
	original := "email,sent\nnot-an-email,false\n"
	if err := os.WriteFile(path, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	f := &fakeClient{}
	svc := NewService(NewStore(path), f)
	svc.sweep(context.Background())
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.emails) != 0 {
		t.Fatalf("malformed row was delivered: %v", f.emails)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != original {
		t.Fatalf("malformed file changed: %q, %v", got, err)
	}
}

func TestMissingTelegramCredentialsWarnWithoutSecrets(t *testing.T) {
	var logs bytes.Buffer
	oldWriter := log.Writer()
	log.SetOutput(&logs)
	defer log.SetOutput(oldWriter)

	err := (Telegram{ChatID: "chat-secret"}).Send(context.Background(), "person@example.com")
	if err == nil {
		t.Fatal("expected missing credentials error")
	}
	if strings.Contains(logs.String(), "chat-secret") || strings.Contains(logs.String(), "person@example.com") {
		t.Fatalf("warning leaked secret or email: %q", logs.String())
	}
	if !strings.Contains(logs.String(), "credentials are not configured") {
		t.Fatalf("missing operational warning: %q", logs.String())
	}
}
func TestFailedDeliveryRemainsUnsent(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "x.csv"))
	if err := s.Append("a@example.com"); err != nil {
		t.Fatal(err)
	}
	svc := NewService(s, &fakeClient{err: context.Canceled})
	svc.sweep(context.Background())
	s.mu.Lock()
	rows, err := s.readLocked()
	s.mu.Unlock()
	if err != nil || len(rows) != 1 || rows[0].Sent {
		t.Fatalf("row changed after failed delivery: %+v %v", rows, err)
	}
}

func TestServiceMarksOnlySuccessfulDelivery(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "x.csv"))
	if err := s.Append("a@example.com"); err != nil {
		t.Fatal(err)
	}
	if err := s.Append("b@example.com"); err != nil {
		t.Fatal(err)
	}
	f := &fakeClient{}
	svc := NewService(s, f)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	svc.Start(ctx)
	svc.Notify()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		s.mu.Lock()
		rows, _ := s.readLocked()
		s.mu.Unlock()
		if len(rows) == 2 && rows[0].Sent && rows[1].Sent {
			return
		}
		time.Sleep(time.Millisecond * 5)
	}
	t.Fatal("rows were not delivered")
}
