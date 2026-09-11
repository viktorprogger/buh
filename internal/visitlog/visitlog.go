// Package visitlog observes authenticated web requests and writes visit diagnostics.
package visitlog

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"buh/internal/accountant"
	"buh/internal/auth"
	"buh/internal/entrepreneuruser"

	"github.com/google/uuid"
)

const resolveTimeout = 3 * time.Second

var ErrNotLoggable = errors.New("visitlog: identity is not loggable")

type Resolver interface {
	Resolve(context.Context, auth.Session) (string, error)
}

type Logger interface {
	Printf(string, ...any)
}

// RepositoryResolver resolves only the two supported authenticated account types.
type RepositoryResolver struct {
	Accountants   *accountant.Repo
	Entrepreneurs *entrepreneuruser.Repo
}

func (r RepositoryResolver) Resolve(ctx context.Context, s auth.Session) (string, error) {
	switch s.UserType {
	case auth.UserTypeAccountant:
		a, err := r.Accountants.FindByID(ctx, s.UserID)
		if err != nil {
			return "", err
		}
		return a.Email, nil
	case auth.UserTypeEntrepreneur:
		id, err := uuid.Parse(s.UserID)
		if err != nil {
			return "", ErrNotLoggable
		}
		u, err := r.Entrepreneurs.FindByID(ctx, id)
		if err != nil {
			return "", err
		}
		return u.Email, nil
	default:
		return "", ErrNotLoggable
	}
}

// Middleware logs qualifying requests while leaving the wrapped response untouched.
func Middleware(sessions *auth.SessionManager, resolver Resolver, logger Logger) func(http.Handler) http.Handler {
	if logger == nil {
		logger = log.Default()
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/static" && r.URL.Path != "/static/" && !strings.HasPrefix(r.URL.Path, "/static/") {
				if session, ok := sessions.Get(r); ok && isSupportedUserType(session.UserType) {
					// Capture request data before detaching: the request and its URL may
					// be reused or mutated after the handler returns.
					method := r.Method
					requestURI := r.URL.RequestURI()
					go notifyVisit(resolver, logger, session, method, requestURI)
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

func notifyVisit(resolver Resolver, logger Logger, session auth.Session, method, requestURI string) {
	ctx, cancel := context.WithTimeout(context.Background(), resolveTimeout)
	defer cancel()

	login, err := resolver.Resolve(ctx, session)
	if err != nil {
		logger.Printf("visit logging identity resolution failed: %v", err)
		return
	}
	message := fmt.Sprintf("#buh #visited %s\n%s %s", login, method, requestURI)
	logger.Printf("%s", message)
}

func isSupportedUserType(userType auth.UserType) bool {
	return userType == auth.UserTypeAccountant || userType == auth.UserTypeEntrepreneur
}
