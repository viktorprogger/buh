package entrepreneuruser

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrNotFound           = errors.New("entrepreneur user: not found")
	ErrInvalidCredentials = errors.New("entrepreneur user: invalid credentials")
)

type User struct {
	ID           uuid.UUID
	Email        string
	PasswordHash string
	Language     string
	CreatedAt    time.Time
}

type Repo struct{ db *sql.DB }

func NewRepo(db *sql.DB) *Repo { return &Repo{db: db} }

func (r *Repo) Create(ctx context.Context, email, password string) (User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return User{}, err
	}
	var u User
	err = r.db.QueryRowContext(ctx,
		`INSERT INTO entrepreneur_users (email, password_hash)
		 VALUES ($1, $2)
		 RETURNING id, email, password_hash, created_at`,
		email, string(hash),
	).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.CreatedAt)
	return u, err
}

func (r *Repo) FindByEmail(ctx context.Context, email string) (User, error) {
	var u User
	err := r.db.QueryRowContext(ctx,
		`SELECT id, email, password_hash, language, created_at FROM entrepreneur_users WHERE email = $1`, email,
	).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Language, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return u, err
}

func (r *Repo) FindByID(ctx context.Context, id uuid.UUID) (User, error) {
	var u User
	err := r.db.QueryRowContext(ctx,
		`SELECT id, email, password_hash, language, created_at FROM entrepreneur_users WHERE id = $1`, id,
	).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Language, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return u, err
}

// SetLanguage persists the preferred UI language for an entrepreneur user.
func (r *Repo) SetLanguage(ctx context.Context, id uuid.UUID, lang string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE entrepreneur_users SET language = $1 WHERE id = $2`, lang, id)
	return err
}

// GetLanguage returns the stored language preference for an entrepreneur user.
func (r *Repo) GetLanguage(ctx context.Context, id uuid.UUID) (string, error) {
	var lang string
	err := r.db.QueryRowContext(ctx, `SELECT language FROM entrepreneur_users WHERE id = $1`, id).Scan(&lang)
	if errors.Is(err, sql.ErrNoRows) {
		return "sr", nil
	}
	return lang, err
}

func (r *Repo) UpdatePasswordHash(ctx context.Context, id uuid.UUID, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx,
		`UPDATE entrepreneur_users SET password_hash = $1 WHERE id = $2`,
		string(hash), id,
	)
	return err
}

// GetPendingNotice returns the pending notice key for the given user, or "".
func (r *Repo) GetPendingNotice(ctx context.Context, id uuid.UUID) (string, error) {
	var n string
	err := r.db.QueryRowContext(ctx, `SELECT pending_notice FROM entrepreneur_users WHERE id = $1`, id).Scan(&n)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return n, err
}

// SetPendingNotice stores a notice key for the given user.
func (r *Repo) SetPendingNotice(ctx context.Context, id uuid.UUID, key string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE entrepreneur_users SET pending_notice = $1 WHERE id = $2`, key, id)
	return err
}

// ClearPendingNotice removes the pending notice for the given user.
func (r *Repo) ClearPendingNotice(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `UPDATE entrepreneur_users SET pending_notice = '' WHERE id = $1`, id)
	return err
}

func CheckPassword(u User, password string) error {
	err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password))
	if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
		return ErrInvalidCredentials
	}
	return err
}
