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
		`SELECT id, email, password_hash, created_at FROM entrepreneur_users WHERE email = $1`, email,
	).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return u, err
}

func (r *Repo) FindByID(ctx context.Context, id uuid.UUID) (User, error) {
	var u User
	err := r.db.QueryRowContext(ctx,
		`SELECT id, email, password_hash, created_at FROM entrepreneur_users WHERE id = $1`, id,
	).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return u, err
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

func CheckPassword(u User, password string) error {
	err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password))
	if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
		return ErrInvalidCredentials
	}
	return err
}
