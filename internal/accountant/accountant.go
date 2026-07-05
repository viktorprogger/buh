package accountant

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// ErrNotFound is returned when no accountant matches the query.
var ErrNotFound = errors.New("accountant: not found")

// ErrInvalidCredentials is returned on wrong email/password.
var ErrInvalidCredentials = errors.New("accountant: invalid credentials")

// Accountant is the user entity in this application.
type Accountant struct {
	ID           string
	Email        string
	PasswordHash string
	Language     string
	CreatedAt    time.Time
}

// Repo handles persistence of accountants.
type Repo struct {
	db *sql.DB
}

// NewRepo creates a new Repo backed by the given database connection.
func NewRepo(db *sql.DB) *Repo {
	return &Repo{db: db}
}

// Create inserts a new accountant, hashing the given plaintext password.
func (r *Repo) Create(ctx context.Context, email, password string) (*Accountant, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	var a Accountant
	err = r.db.QueryRowContext(ctx,
		`INSERT INTO accountants (email, password_hash)
		 VALUES ($1, $2)
		 RETURNING id, email, password_hash, created_at`,
		email, string(hash),
	).Scan(&a.ID, &a.Email, &a.PasswordHash, &a.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// FindByEmail returns the accountant with the given email, or ErrNotFound.
func (r *Repo) FindByEmail(ctx context.Context, email string) (*Accountant, error) {
	var a Accountant
	err := r.db.QueryRowContext(ctx,
		`SELECT id, email, password_hash, language, created_at FROM accountants WHERE email = $1`,
		email,
	).Scan(&a.ID, &a.Email, &a.PasswordHash, &a.Language, &a.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// Count returns the total number of accountants in the database.
func (r *Repo) Count(ctx context.Context) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM accountants`).Scan(&n)
	return n, err
}

// SetLanguage persists the preferred UI language for an accountant.
func (r *Repo) SetLanguage(ctx context.Context, id, lang string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE accountants SET language = $1 WHERE id = $2`, lang, id)
	return err
}

// GetLanguage returns the stored language preference for an accountant.
func (r *Repo) GetLanguage(ctx context.Context, id string) (string, error) {
	var lang string
	err := r.db.QueryRowContext(ctx, `SELECT language FROM accountants WHERE id = $1`, id).Scan(&lang)
	if errors.Is(err, sql.ErrNoRows) {
		return "sr", nil
	}
	return lang, err
}

// UpdatePasswordHash replaces the stored password for the given accountant ID.
func (r *Repo) UpdatePasswordHash(ctx context.Context, id, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx,
		`UPDATE accountants SET password_hash = $1 WHERE id = $2`,
		string(hash), id,
	)
	return err
}

// CheckPassword verifies the plaintext password against the stored bcrypt hash.
// Returns ErrInvalidCredentials on mismatch.
func CheckPassword(a *Accountant, password string) error {
	err := bcrypt.CompareHashAndPassword([]byte(a.PasswordHash), []byte(password))
	if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
		return ErrInvalidCredentials
	}
	return err
}
