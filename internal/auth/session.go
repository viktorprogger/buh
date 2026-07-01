package auth

import (
	"net/http"
	"time"

	"github.com/gorilla/securecookie"
)

const cookieName = "buh_session"
const sessionTTL = 24 * time.Hour

type UserType string

const (
	UserTypeAccountant   UserType = "accountant"
	UserTypeEntrepreneur UserType = "entrepreneur"
)

type Session struct {
	UserType UserType
	UserID   string
}

// SessionManager manages secure, encrypted session cookies.
type SessionManager struct {
	sc *securecookie.SecureCookie
}

// NewSessionManager creates a SessionManager from HMAC and AES keys.
// If either key is empty, random keys are generated (sessions will not
// survive restarts — suitable only for dev).
func NewSessionManager(hashKey, blockKey []byte) *SessionManager {
	if len(hashKey) == 0 {
		hashKey = securecookie.GenerateRandomKey(32)
	}
	if len(blockKey) == 0 {
		blockKey = securecookie.GenerateRandomKey(32)
	}
	return &SessionManager{
		sc: securecookie.New(hashKey, blockKey),
	}
}

// Set writes a session cookie containing the typed session.
func (s *SessionManager) Set(w http.ResponseWriter, sess Session) error {
	value := map[string]string{
		"user_type": string(sess.UserType),
		"user_id":   sess.UserID,
	}
	encoded, err := s.sc.Encode(cookieName, value)
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    encoded,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(sessionTTL),
	})
	return nil
}

// Get retrieves the session from the cookie.
// Returns zero Session and false if there is no valid session.
func (s *SessionManager) Get(r *http.Request) (Session, bool) {
	c, err := r.Cookie(cookieName)
	if err != nil {
		return Session{}, false
	}
	value := map[string]string{}
	if err := s.sc.Decode(cookieName, c.Value, &value); err != nil {
		return Session{}, false
	}
	userType, ok1 := value["user_type"]
	userID, ok2 := value["user_id"]
	if !ok1 || !ok2 || userID == "" {
		return Session{}, false
	}
	return Session{UserType: UserType(userType), UserID: userID}, true
}

// Clear expires the session cookie.
func (s *SessionManager) Clear(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
}
