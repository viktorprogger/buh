// Package sliprecord handles DB persistence of generated payment slip records.
// It is named sliprecord (not slip) to avoid conflict with internal/slip, which
// handles PDF generation.
package sliprecord

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
)

// ErrNotFound is returned when no slip record matches the query.
var ErrNotFound = errors.New("sliprecord: not found")

// UpsertStatus describes the outcome of a FindOrUpdateByPurpose call.
type UpsertStatus int

const (
	UpsertCreated   UpsertStatus = iota // new record inserted
	UpsertUpdated                       // existing record fields changed
	UpsertUnchanged                     // existing record identical, no write
)

// SlipRecord is a persisted record of a generated payment slip.
type SlipRecord struct {
	ID             uuid.UUID
	EntrepreneurID uuid.UUID
	PaymentCode    string
	Amount         string
	Currency       string
	Purpose        string
	PayeeAccount   string
	Reference      string
	Payee          string
	Payer          string
	GeneratedAt    time.Time
}

// Repo handles persistence of slip records.
type Repo struct {
	db *sql.DB
}

// NewRepo creates a new Repo backed by the given database connection.
func NewRepo(db *sql.DB) *Repo {
	return &Repo{db: db}
}

// Save inserts a new SlipRecord. The ID and GeneratedAt fields are set by the database.
func (r *Repo) Save(ctx context.Context, s SlipRecord) (SlipRecord, error) {
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO slip_records
		 (entrepreneur_id, payment_code, amount, currency, purpose, payee_account, reference, payee, payer)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		 RETURNING id, entrepreneur_id, payment_code, amount, currency, purpose, payee_account, reference, payee, payer, generated_at`,
		s.EntrepreneurID, s.PaymentCode, s.Amount, s.Currency, s.Purpose, s.PayeeAccount, s.Reference, s.Payee, s.Payer,
	).Scan(
		&s.ID, &s.EntrepreneurID, &s.PaymentCode, &s.Amount, &s.Currency,
		&s.Purpose, &s.PayeeAccount, &s.Reference, &s.Payee, &s.Payer, &s.GeneratedAt,
	)
	if err != nil {
		return SlipRecord{}, err
	}
	return s, nil
}

// ListByEntrepreneur returns all slip records for the given entrepreneur, newest first.
func (r *Repo) ListByEntrepreneur(ctx context.Context, entrepreneurID uuid.UUID) ([]SlipRecord, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, entrepreneur_id, payment_code, amount, currency, purpose, payee_account, reference, payee, payer, generated_at
		 FROM slip_records
		 WHERE entrepreneur_id = $1
		 ORDER BY generated_at DESC`,
		entrepreneurID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []SlipRecord
	for rows.Next() {
		var s SlipRecord
		if err := rows.Scan(
			&s.ID, &s.EntrepreneurID, &s.PaymentCode, &s.Amount, &s.Currency,
			&s.Purpose, &s.PayeeAccount, &s.Reference, &s.Payee, &s.Payer, &s.GeneratedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// Update overwrites the editable fields of an existing SlipRecord by ID.
func (r *Repo) Update(ctx context.Context, s SlipRecord) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE slip_records
		 SET payer=$1, purpose=$2, payee=$3, payee_account=$4, reference=$5, payment_code=$6, amount=$7, currency=$8
		 WHERE id=$9`,
		s.Payer, s.Purpose, s.Payee, s.PayeeAccount, s.Reference, s.PaymentCode, s.Amount, s.Currency, s.ID,
	)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// FindOrUpdateByPurpose looks up a slip by (entrepreneur_id, purpose).
// If found and unchanged it returns UpsertUnchanged; if fields differ it updates and returns UpsertUpdated.
// If not found it inserts and returns UpsertCreated.
func (r *Repo) FindOrUpdateByPurpose(ctx context.Context, s SlipRecord) (SlipRecord, UpsertStatus, error) {
	var existing SlipRecord
	err := r.db.QueryRowContext(ctx,
		`SELECT id, entrepreneur_id, payment_code, amount, currency, purpose, payee_account, reference, payee, payer, generated_at
		 FROM slip_records
		 WHERE entrepreneur_id = $1 AND purpose = $2
		 LIMIT 1`,
		s.EntrepreneurID, s.Purpose,
	).Scan(
		&existing.ID, &existing.EntrepreneurID, &existing.PaymentCode, &existing.Amount,
		&existing.Currency, &existing.Purpose, &existing.PayeeAccount, &existing.Reference,
		&existing.Payee, &existing.Payer, &existing.GeneratedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		saved, err := r.Save(ctx, s)
		return saved, UpsertCreated, err
	}
	if err != nil {
		return SlipRecord{}, 0, err
	}

	if existing.PaymentCode == s.PaymentCode &&
		existing.Amount == s.Amount &&
		existing.Currency == s.Currency &&
		existing.PayeeAccount == s.PayeeAccount &&
		existing.Reference == s.Reference &&
		existing.Payee == s.Payee &&
		existing.Payer == s.Payer {
		return existing, UpsertUnchanged, nil
	}

	s.ID = existing.ID
	s.GeneratedAt = existing.GeneratedAt
	if err := r.Update(ctx, s); err != nil {
		return SlipRecord{}, 0, err
	}
	return s, UpsertUpdated, nil
}

// Delete removes the slip record with the given ID.
func (r *Repo) Delete(ctx context.Context, id uuid.UUID) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM slip_records WHERE id = $1`, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// FindByID returns the slip record with the given ID, or ErrNotFound.
func (r *Repo) FindByID(ctx context.Context, id uuid.UUID) (SlipRecord, error) {
	var s SlipRecord
	err := r.db.QueryRowContext(ctx,
		`SELECT id, entrepreneur_id, payment_code, amount, currency, purpose, payee_account, reference, payee, payer, generated_at
		 FROM slip_records
		 WHERE id = $1`,
		id,
	).Scan(
		&s.ID, &s.EntrepreneurID, &s.PaymentCode, &s.Amount, &s.Currency,
		&s.Purpose, &s.PayeeAccount, &s.Reference, &s.Payee, &s.Payer, &s.GeneratedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return SlipRecord{}, ErrNotFound
	}
	return s, err
}
