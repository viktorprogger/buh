package accessrequest

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/mail"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

var ErrInvalidEmail = errors.New("invalid email address")

// ValidateEmail trims and validates an email address.
func ValidateEmail(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", ErrInvalidEmail
	}
	a, err := mail.ParseAddress(s)
	if err != nil || a.Address != s || !strings.Contains(s, "@") {
		return "", ErrInvalidEmail
	}
	return s, nil
}

type Row struct {
	Email string
	Sent  bool
}

type Store struct {
	path string
	mu   sync.Mutex
}

func NewStore(path string) *Store { return &Store{path: path} }
func (s *Store) Append(email string) error {
	email, err := ValidateEmail(email)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.readLocked()
	if err != nil {
		return err
	}
	rows = append(rows, Row{Email: email})
	return s.writeLocked(rows)
}
func (s *Store) readLocked() ([]Row, error) {
	f, err := os.Open(s.path)
	if os.IsNotExist(err) {
		return []Row{}, s.writeLocked([]Row{})
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	header, err := r.Read()
	if err != nil || len(header) != 2 || header[0] != "email" || header[1] != "sent" {
		return nil, fmt.Errorf("invalid access request CSV header")
	}
	var rows []Row
	for {
		rec, e := r.Read()
		if e == io.EOF {
			break
		}
		if e != nil || len(rec) != 2 || rec[0] == "" {
			return nil, fmt.Errorf("invalid access request CSV row")
		}
		if _, e := ValidateEmail(rec[0]); e != nil {
			return nil, fmt.Errorf("invalid access request email")
		}
		sent, e := strconv.ParseBool(rec[1])
		if e != nil {
			return nil, fmt.Errorf("invalid access request sent value")
		}
		rows = append(rows, Row{Email: rec[0], Sent: sent})
	}
	return rows, nil
}
func (s *Store) writeLocked(rows []Row) error {
	parent := filepath.Dir(s.path)
	info, err := os.Stat(parent)
	if err != nil {
		return fmt.Errorf("access request storage parent is unavailable: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("access request storage parent is not a directory")
	}
	tmp, err := os.CreateTemp(parent, ".accountant-access-requests-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if err := tmp.Chmod(0600); err != nil {
		tmp.Close()
		return err
	}
	w := csv.NewWriter(tmp)
	if err = w.Write([]string{"email", "sent"}); err != nil {
		tmp.Close()
		return err
	}
	for _, row := range rows {
		if err = w.Write([]string{row.Email, strconv.FormatBool(row.Sent)}); err != nil {
			tmp.Close()
			return err
		}
	}
	w.Flush()
	if err = w.Error(); err != nil {
		tmp.Close()
		return err
	}
	if err = tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, s.path)
}

// TelegramClient sends one operator notification.
type TelegramClient interface {
	Send(ctx context.Context, email string) error
}
type Telegram struct {
	Token, ChatID, ThreadID string
	HTTPClient              *http.Client
}

func (t Telegram) Send(ctx context.Context, email string) error {
	if t.Token == "" || t.ChatID == "" {
		log.Printf("WARN access-request Telegram notifications unavailable: credentials are not configured")
		return errors.New("telegram is not configured")
	}
	body := map[string]string{"chat_id": t.ChatID, "text": "Accountant access request: " + email}
	if t.ThreadID != "" {
		body["message_thread_id"] = t.ThreadID
	}
	b, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.telegram.org/bot"+t.Token+"/sendMessage", strings.NewReader(string(b)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	c := t.HTTPClient
	if c == nil {
		c = &http.Client{Timeout: 10 * time.Second}
	}
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("telegram HTTP status %d", resp.StatusCode)
	}
	var out struct {
		OK bool `json:"ok"`
	}
	if json.NewDecoder(resp.Body).Decode(&out) != nil || !out.OK {
		return errors.New("telegram API rejected message")
	}
	return nil
}

// Service owns asynchronous notification retries.
type Service struct {
	Store              *Store
	Client             TelegramClient
	wake               chan struct{}
	once               sync.Once
}

func NewService(store *Store, client TelegramClient) *Service {
	if telegramCredentialsMissing(client) {
		log.Printf("WARN access-request Telegram notifications unavailable: credentials are not configured")
	}
	return &Service{Store: store, Client: client, wake: make(chan struct{}, 1)}
}

func telegramCredentialsMissing(client TelegramClient) bool {
	switch t := client.(type) {
	case Telegram:
		return t.Token == "" || t.ChatID == ""
	case *Telegram:
		return t == nil || t.Token == "" || t.ChatID == ""
	default:
		return false
	}
}
func (s *Service) Notify() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}
func (s *Service) sweep(ctx context.Context) {
	if s.Client == nil {
		return
	}
	s.Store.mu.Lock()
	defer s.Store.mu.Unlock()
	rows, err := s.Store.readLocked()
	if err != nil {
		return
	}
	for i := range rows {
		if rows[i].Sent {
			continue
		}
		if err := s.Client.Send(ctx, rows[i].Email); err != nil {
			continue
		}
		rows[i].Sent = true
		if err := s.Store.writeLocked(rows); err != nil {
			return
		}
	}
}
func (s *Service) Start(ctx context.Context) { s.start(ctx, time.Hour) }

// StartWithInterval is useful for deterministic tests and local operation; production uses Start.
func (s *Service) StartWithInterval(ctx context.Context, interval time.Duration) {
	s.start(ctx, interval)
}

func (s *Service) start(ctx context.Context, interval time.Duration) {
	s.once.Do(func() {
		go func() {
			s.sweep(ctx)
			ticker := time.NewTicker(interval)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-s.wake:
					s.sweep(ctx)
				case <-ticker.C:
					s.sweep(ctx)
				}
			}
		}()
	})
}
