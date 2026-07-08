// Package slipmerge tracks which slip years have been merged between an accountant
// and an entrepreneur user.
package slipmerge

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
)

// Repo persists slip merge records.
type Repo struct{ db *sql.DB }

// NewRepo creates a new Repo backed by the given database connection.
func NewRepo(db *sql.DB) *Repo { return &Repo{db: db} }

// IsMerged returns true when the given (managedEntrepreneurID, year) has been merged.
func (r *Repo) IsMerged(ctx context.Context, managedEntrepreneurID uuid.UUID, year int) (bool, error) {
	var exists bool
	err := r.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM slip_merges WHERE managed_entrepreneur_id = $1 AND year = $2)`,
		managedEntrepreneurID, year,
	).Scan(&exists)
	return exists, err
}

// MarkMerged records that a given year has been fully merged. Idempotent on repeated calls.
func (r *Repo) MarkMerged(ctx context.Context, managedEntrepreneurID uuid.UUID, year int, mergedBy uuid.UUID) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO slip_merges (managed_entrepreneur_id, year, merged_by)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (managed_entrepreneur_id, year) DO UPDATE SET merged_at = now(), merged_by = EXCLUDED.merged_by`,
		managedEntrepreneurID, year, mergedBy,
	)
	return err
}

// DeleteForPairing removes all merge records for a pairing. Called on unpair.
func (r *Repo) DeleteForPairing(ctx context.Context, managedEntrepreneurID uuid.UUID) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM slip_merges WHERE managed_entrepreneur_id = $1`,
		managedEntrepreneurID,
	)
	return err
}

// ListMergedYears returns all years that have been merged for a managed entrepreneur.
func (r *Repo) ListMergedYears(ctx context.Context, managedEntrepreneurID uuid.UUID) ([]int, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT year FROM slip_merges WHERE managed_entrepreneur_id = $1 ORDER BY year DESC`,
		managedEntrepreneurID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var years []int
	for rows.Next() {
		var y int
		if err := rows.Scan(&y); err != nil {
			return nil, err
		}
		years = append(years, y)
	}
	return years, rows.Err()
}

// ErrNotFound is returned when no merge record matches.
var ErrNotFound = errors.New("slipmerge: not found")

// UnmarkMerged removes a specific merge record (for re-merge scenarios).
func (r *Repo) UnmarkMerged(ctx context.Context, managedEntrepreneurID uuid.UUID, year int) error {
	res, err := r.db.ExecContext(ctx,
		`DELETE FROM slip_merges WHERE managed_entrepreneur_id = $1 AND year = $2`,
		managedEntrepreneurID, year,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
