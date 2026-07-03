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
	Year           int
	Advance        bool
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

const selectCols = `id, entrepreneur_id, payment_code, amount, currency, purpose, payee_account, reference, payee, payer, year, advance, generated_at`

func scanSlip(row interface {
	Scan(...any) error
}, s *SlipRecord) error {
	return row.Scan(
		&s.ID, &s.EntrepreneurID, &s.PaymentCode, &s.Amount, &s.Currency,
		&s.Purpose, &s.PayeeAccount, &s.Reference, &s.Payee, &s.Payer,
		&s.Year, &s.Advance, &s.GeneratedAt,
	)
}

// Save inserts a new SlipRecord. The ID and GeneratedAt fields are set by the database.
func (r *Repo) Save(ctx context.Context, s SlipRecord) (SlipRecord, error) {
	err := scanSlip(r.db.QueryRowContext(ctx,
		`INSERT INTO slip_records
		 (entrepreneur_id, payment_code, amount, currency, purpose, payee_account, reference, payee, payer, year, advance)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		 RETURNING `+selectCols,
		s.EntrepreneurID, s.PaymentCode, s.Amount, s.Currency, s.Purpose,
		s.PayeeAccount, s.Reference, s.Payee, s.Payer, s.Year, s.Advance,
	), &s)
	if err != nil {
		return SlipRecord{}, err
	}
	return s, nil
}

// ListByEntrepreneur returns all slip records for the given entrepreneur, newest first.
func (r *Repo) ListByEntrepreneur(ctx context.Context, entrepreneurID uuid.UUID) ([]SlipRecord, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+selectCols+`
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
		if err := scanSlip(rows, &s); err != nil {
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
		 SET payer=$1, purpose=$2, payee=$3, payee_account=$4, reference=$5, payment_code=$6, amount=$7, currency=$8, year=$9, advance=$10
		 WHERE id=$11`,
		s.Payer, s.Purpose, s.Payee, s.PayeeAccount, s.Reference,
		s.PaymentCode, s.Amount, s.Currency, s.Year, s.Advance, s.ID,
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

// FindOrUpdateByPurpose looks up a slip by (entrepreneur_id, purpose, advance).
// If found and unchanged it returns UpsertUnchanged; if fields differ it updates and returns UpsertUpdated.
// If not found it inserts and returns UpsertCreated.
// The advance field is part of the key because the same purpose text can appear on both a main slip
// and an advance slip from different tax-year resolutions.
// Returns (old, result, status, err); old is zero for UpsertCreated, equals result for UpsertUnchanged.
func (r *Repo) FindOrUpdateByPurpose(ctx context.Context, s SlipRecord) (old SlipRecord, result SlipRecord, status UpsertStatus, err error) {
	var existing SlipRecord
	scanErr := scanSlip(r.db.QueryRowContext(ctx,
		`SELECT `+selectCols+`
		 FROM slip_records
		 WHERE entrepreneur_id = $1 AND purpose = $2 AND advance = $3
		 LIMIT 1`,
		s.EntrepreneurID, s.Purpose, s.Advance,
	), &existing)
	if errors.Is(scanErr, sql.ErrNoRows) {
		saved, saveErr := r.Save(ctx, s)
		return SlipRecord{}, saved, UpsertCreated, saveErr
	}
	if scanErr != nil {
		return SlipRecord{}, SlipRecord{}, 0, scanErr
	}

	if existing.PaymentCode == s.PaymentCode &&
		existing.Amount == s.Amount &&
		existing.Currency == s.Currency &&
		existing.PayeeAccount == s.PayeeAccount &&
		existing.Reference == s.Reference &&
		existing.Payee == s.Payee &&
		existing.Payer == s.Payer &&
		existing.Year == s.Year {
		return existing, existing, UpsertUnchanged, nil
	}

	s.ID = existing.ID
	s.GeneratedAt = existing.GeneratedAt
	if updateErr := r.Update(ctx, s); updateErr != nil {
		return SlipRecord{}, SlipRecord{}, 0, updateErr
	}
	return existing, s, UpsertUpdated, nil
}

// Snapshot returns all editable fields of a SlipRecord as a flat map (for history logging).
func Snapshot(s SlipRecord) map[string]any {
	return map[string]any{
		"amount":       s.Amount,
		"currency":     s.Currency,
		"purpose":      s.Purpose,
		"payee":        s.Payee,
		"payeeAccount": s.PayeeAccount,
		"reference":    s.Reference,
		"payer":        s.Payer,
		"paymentCode":  s.PaymentCode,
		"year":         s.Year,
		"advance":      s.Advance,
	}
}

type fieldDiff struct {
	Old any `json:"old"`
	New any `json:"new"`
}

// Diff returns only the fields that differ between old and new as {field: {old, new}} (for history logging).
// Returns nil if there are no differences.
func Diff(old, new SlipRecord) map[string]any {
	out := map[string]any{}
	if old.Amount != new.Amount {
		out["amount"] = fieldDiff{Old: old.Amount, New: new.Amount}
	}
	if old.Currency != new.Currency {
		out["currency"] = fieldDiff{Old: old.Currency, New: new.Currency}
	}
	if old.Purpose != new.Purpose {
		out["purpose"] = fieldDiff{Old: old.Purpose, New: new.Purpose}
	}
	if old.Payee != new.Payee {
		out["payee"] = fieldDiff{Old: old.Payee, New: new.Payee}
	}
	if old.PayeeAccount != new.PayeeAccount {
		out["payeeAccount"] = fieldDiff{Old: old.PayeeAccount, New: new.PayeeAccount}
	}
	if old.Reference != new.Reference {
		out["reference"] = fieldDiff{Old: old.Reference, New: new.Reference}
	}
	if old.Payer != new.Payer {
		out["payer"] = fieldDiff{Old: old.Payer, New: new.Payer}
	}
	if old.PaymentCode != new.PaymentCode {
		out["paymentCode"] = fieldDiff{Old: old.PaymentCode, New: new.PaymentCode}
	}
	if old.Year != new.Year {
		out["year"] = fieldDiff{Old: old.Year, New: new.Year}
	}
	if old.Advance != new.Advance {
		out["advance"] = fieldDiff{Old: old.Advance, New: new.Advance}
	}
	if len(out) == 0 {
		return nil
	}
	return out
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
	err := scanSlip(r.db.QueryRowContext(ctx,
		`SELECT `+selectCols+` FROM slip_records WHERE id = $1`,
		id,
	), &s)
	if errors.Is(err, sql.ErrNoRows) {
		return SlipRecord{}, ErrNotFound
	}
	return s, err
}
