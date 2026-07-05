package uploadqueue

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
)

type Job struct {
	ID           uuid.UUID
	BatchID      uuid.UUID
	AccountantID uuid.UUID
	Filename     string
	Data         []byte
}

type JobResult struct {
	EntrepreneurName   string
	EntrepreneurID     uuid.UUID
	IsNewEntrepreneur  bool
	SlipsCreated       int
	SlipsUpdated       int
}

type FailedFile struct {
	Filename  string
	ErrorText string
}

type DoneFile struct {
	Filename           string
	EntrepreneurName   string
	EntrepreneurID     uuid.UUID
	HasEntrepreneur    bool
	IsNewEntrepreneur  bool
	SlipsCreated       int
	SlipsUpdated       int
}

type BatchSummary struct {
	BatchID uuid.UUID
	Pending int
	Failed  []FailedFile
	Done    []DoneFile
}

type Repo struct {
	db *sql.DB
}

func NewRepo(db *sql.DB) *Repo {
	return &Repo{db: db}
}

func (r *Repo) CreateBatch(ctx context.Context, accountantID uuid.UUID) (uuid.UUID, error) {
	var id uuid.UUID
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO upload_batches (accountant_id) VALUES ($1) RETURNING id`,
		accountantID,
	).Scan(&id)
	return id, err
}

func (r *Repo) Enqueue(ctx context.Context, batchID, accountantID uuid.UUID, filename string, data []byte) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO upload_queue (batch_id, accountant_id, filename, file_data) VALUES ($1, $2, $3, $4)`,
		batchID, accountantID, filename, data,
	)
	return err
}

// Claim atomically picks the oldest pending job and marks it processing.
// Returns ok=false when the queue is empty.
func (r *Repo) Claim(ctx context.Context) (Job, bool, error) {
	var j Job
	err := r.db.QueryRowContext(ctx, `
		UPDATE upload_queue
		SET status = 'processing', claimed_at = now()
		WHERE id = (
			SELECT id FROM upload_queue
			WHERE status = 'pending'
			ORDER BY created_at
			LIMIT 1
			FOR UPDATE SKIP LOCKED
		)
		RETURNING id, batch_id, accountant_id, filename, file_data`,
	).Scan(&j.ID, &j.BatchID, &j.AccountantID, &j.Filename, &j.Data)
	if err == sql.ErrNoRows {
		return Job{}, false, nil
	}
	if err != nil {
		return Job{}, false, fmt.Errorf("claim: %w", err)
	}
	return j, true, nil
}

// Complete records the result and deletes the queue row atomically.
func (r *Repo) Complete(ctx context.Context, job Job, result JobResult) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var entrepreneurID interface{}
	if result.EntrepreneurID != uuid.Nil {
		entrepreneurID = result.EntrepreneurID
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO upload_results
			(batch_id, filename, entrepreneur_name, entrepreneur_id, is_new_entrepreneur, slips_created, slips_updated)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		job.BatchID, job.Filename,
		result.EntrepreneurName, entrepreneurID, result.IsNewEntrepreneur,
		result.SlipsCreated, result.SlipsUpdated,
	)
	if err != nil {
		return fmt.Errorf("insert result: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM upload_queue WHERE id = $1`, job.ID); err != nil {
		return fmt.Errorf("delete queue row: %w", err)
	}
	return tx.Commit()
}

// Fail marks a job as failed and clears its file_data to free space.
func (r *Repo) Fail(ctx context.Context, jobID uuid.UUID, errText string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE upload_queue SET status = 'failed', error_text = $2, file_data = NULL WHERE id = $1`,
		jobID, errText,
	)
	return err
}

// ResetStale resets any 'processing' rows back to 'pending'.
// Call on startup: no worker was running before this process started.
func (r *Repo) ResetStale(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE upload_queue SET status = 'pending', claimed_at = NULL WHERE status = 'processing'`,
	)
	return err
}

func (r *Repo) BatchSummary(ctx context.Context, batchID uuid.UUID) (BatchSummary, error) {
	sum := BatchSummary{BatchID: batchID}

	// Count pending+processing
	err := r.db.QueryRowContext(ctx,
		`SELECT count(*) FROM upload_queue WHERE batch_id = $1 AND status IN ('pending','processing')`,
		batchID,
	).Scan(&sum.Pending)
	if err != nil {
		return sum, err
	}

	// Failed files
	rows, err := r.db.QueryContext(ctx,
		`SELECT filename, error_text FROM upload_queue WHERE batch_id = $1 AND status = 'failed' ORDER BY created_at`,
		batchID,
	)
	if err != nil {
		return sum, err
	}
	defer rows.Close()
	for rows.Next() {
		var f FailedFile
		if err := rows.Scan(&f.Filename, &f.ErrorText); err != nil {
			return sum, err
		}
		sum.Failed = append(sum.Failed, f)
	}
	if err := rows.Err(); err != nil {
		return sum, err
	}

	// Done files
	doneRows, err := r.db.QueryContext(ctx, `
		SELECT filename, entrepreneur_name, entrepreneur_id,
		       is_new_entrepreneur, slips_created, slips_updated
		FROM upload_results WHERE batch_id = $1 ORDER BY processed_at`,
		batchID,
	)
	if err != nil {
		return sum, err
	}
	defer doneRows.Close()
	for doneRows.Next() {
		var d DoneFile
		var eid *uuid.UUID
		if err := doneRows.Scan(&d.Filename, &d.EntrepreneurName, &eid,
			&d.IsNewEntrepreneur, &d.SlipsCreated, &d.SlipsUpdated); err != nil {
			return sum, err
		}
		if eid != nil {
			d.EntrepreneurID = *eid
			d.HasEntrepreneur = true
		}
		sum.Done = append(sum.Done, d)
	}
	return sum, doneRows.Err()
}
