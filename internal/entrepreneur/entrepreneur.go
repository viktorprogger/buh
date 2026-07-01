package entrepreneur

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
)

// ErrNotFound is returned when no entrepreneur matches the query.
var ErrNotFound = errors.New("entrepreneur: not found")

// Entrepreneur represents a paušal entrepreneur tracked under a specific accountant.
type Entrepreneur struct {
	ID                   uuid.UUID
	Title                string // short display label chosen by the accountant; falls back to Name in lists
	Name                 string
	PIB                  string
	Address              string
	BankAccount          string
	AccountantID         uuid.UUID
	EntrepreneurUserID   *uuid.UUID // NULL until paired with an entrepreneur_user
	PairedAt             *time.Time
	CreatedAt            time.Time
}

// ProfileComplete returns true when all fields needed for invoice PDF/SEF are filled.
func (e Entrepreneur) ProfileComplete() bool {
	return e.PIB != "" && e.Address != "" && e.BankAccount != ""
}

// IsPaired returns true if this managed entrepreneur is linked to an entrepreneur_user.
func (e Entrepreneur) IsPaired() bool { return e.EntrepreneurUserID != nil }

// Repo handles persistence of managed entrepreneurs.
type Repo struct {
	db *sql.DB
}

// NewRepo creates a new Repo backed by the given database connection.
func NewRepo(db *sql.DB) *Repo {
	return &Repo{db: db}
}

const selectCols = `id, name, pib, accountant_id, created_at, title, address, bank_account,
	entrepreneur_user_id, paired_at`

func scanRow(row interface{ Scan(...any) error }, e *Entrepreneur) error {
	var entrepreneurUserID sql.NullString
	var pairedAt sql.NullTime
	err := row.Scan(
		&e.ID, &e.Name, &e.PIB, &e.AccountantID, &e.CreatedAt,
		&e.Title, &e.Address, &e.BankAccount,
		&entrepreneurUserID, &pairedAt,
	)
	if err != nil {
		return err
	}
	if entrepreneurUserID.Valid {
		id, err := uuid.Parse(entrepreneurUserID.String)
		if err == nil {
			e.EntrepreneurUserID = &id
		}
	}
	if pairedAt.Valid {
		e.PairedAt = &pairedAt.Time
	}
	return nil
}

// FindOrCreate returns the existing entrepreneur with the given PIB+accountantID,
// or inserts a new one with the provided name and returns it.
// The bool return value is true when a new entrepreneur was inserted.
func (r *Repo) FindOrCreate(ctx context.Context, accountantID uuid.UUID, pib, name string) (Entrepreneur, bool, error) {
	var e Entrepreneur
	err := scanRow(r.db.QueryRowContext(ctx,
		`SELECT `+selectCols+` FROM managed_entrepreneurs WHERE pib = $1 AND accountant_id = $2`,
		pib, accountantID,
	), &e)
	if err == nil {
		return e, false, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return Entrepreneur{}, false, err
	}

	err = scanRow(r.db.QueryRowContext(ctx,
		`INSERT INTO managed_entrepreneurs (accountant_id, pib, name)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (accountant_id, pib) DO UPDATE SET name = EXCLUDED.name
		 RETURNING `+selectCols,
		accountantID, pib, name,
	), &e)
	if err != nil {
		return Entrepreneur{}, false, err
	}
	return e, true, nil
}

// ListByAccountant returns all entrepreneurs belonging to the given accountant.
func (r *Repo) ListByAccountant(ctx context.Context, accountantID uuid.UUID) ([]Entrepreneur, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+selectCols+` FROM managed_entrepreneurs WHERE accountant_id = $1 ORDER BY name`,
		accountantID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Entrepreneur
	for rows.Next() {
		var e Entrepreneur
		if err := scanRow(rows, &e); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// Update saves Name, PIB, Title, Address, and BankAccount for the given entrepreneur.
func (r *Repo) Update(ctx context.Context, e Entrepreneur) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE managed_entrepreneurs SET name=$1, pib=$2, title=$3, address=$4, bank_account=$5 WHERE id=$6`,
		e.Name, e.PIB, e.Title, e.Address, e.BankAccount, e.ID,
	)
	return err
}

// FindByID returns the entrepreneur with the given ID, or ErrNotFound.
func (r *Repo) FindByID(ctx context.Context, id uuid.UUID) (Entrepreneur, error) {
	var e Entrepreneur
	err := scanRow(r.db.QueryRowContext(ctx,
		`SELECT `+selectCols+` FROM managed_entrepreneurs WHERE id = $1`, id,
	), &e)
	if errors.Is(err, sql.ErrNoRows) {
		return Entrepreneur{}, ErrNotFound
	}
	return e, err
}

// FindByEntrepreneurUserID returns the managed entrepreneur linked to the given entrepreneur_user,
// or ErrNotFound.
func (r *Repo) FindByEntrepreneurUserID(ctx context.Context, userID uuid.UUID) (Entrepreneur, error) {
	var e Entrepreneur
	err := scanRow(r.db.QueryRowContext(ctx,
		`SELECT `+selectCols+` FROM managed_entrepreneurs WHERE entrepreneur_user_id = $1`, userID,
	), &e)
	if errors.Is(err, sql.ErrNoRows) {
		return Entrepreneur{}, ErrNotFound
	}
	return e, err
}

// Pair links a managed entrepreneur to an entrepreneur_user and records the pairing time.
func (r *Repo) Pair(ctx context.Context, managedID, entrepreneurUserID uuid.UUID) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE managed_entrepreneurs
		 SET entrepreneur_user_id = $1, paired_at = now()
		 WHERE id = $2 AND entrepreneur_user_id IS NULL`,
		entrepreneurUserID, managedID,
	)
	return err
}
