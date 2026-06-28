package auth

import (
	"net/http"
	"time"

	"github.com/gorilla/securecookie"
)

const cookieName = "buh_session"
const sessionTTL = 24 * time.Hour

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

// Set writes a session cookie containing the accountant ID.
func (s *SessionManager) Set(w http.ResponseWriter, accountantID string) error {
	value := map[string]string{"accountant_id": accountantID}
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

// Get retrieves the accountant ID from the session cookie.
// Returns ("", false) if there is no valid session.
func (s *SessionManager) Get(r *http.Request) (string, bool) {
	c, err := r.Cookie(cookieName)
	if err != nil {
		return "", false
	}
	value := map[string]string{}
	if err := s.sc.Decode(cookieName, c.Value, &value); err != nil {
		return "", false
	}
	id, ok := value["accountant_id"]
	return id, ok
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
