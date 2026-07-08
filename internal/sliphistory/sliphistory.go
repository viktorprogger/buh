package sliphistory

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Event string

const (
	EventCreated  Event = "created"
	EventImported Event = "imported"
	EventUpdated  Event = "updated"
	EventDeleted  Event = "deleted"
	EventMerged   Event = "merged"
)

// Record is one entry in the slip audit log.
type Record struct {
	ID        uuid.UUID
	SlipID    uuid.UUID
	Event     Event
	ActorType string
	ActorID   uuid.UUID
	Changes   map[string]any
	ChangedAt time.Time
}

// Repo persists and retrieves slip history entries.
type Repo struct {
	db *sql.DB
}

func NewRepo(db *sql.DB) *Repo { return &Repo{db: db} }

// Log records one history event. Errors are non-fatal to callers — the main
// slip operation has already committed; history is best-effort.
func (r *Repo) Log(ctx context.Context, slipID uuid.UUID, event Event, actorType string, actorID uuid.UUID, changes map[string]any) error {
	data, err := json.Marshal(changes)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx,
		`INSERT INTO slip_history (slip_id, event, actor_type, actor_id, changes)
		 VALUES ($1, $2, $3, $4, $5)`,
		slipID, string(event), actorType, actorID, data,
	)
	return err
}

// ListBySlip returns history entries for a slip, newest first.
func (r *Repo) ListBySlip(ctx context.Context, slipID uuid.UUID) ([]Record, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, slip_id, event, actor_type, actor_id, changes, changed_at
		 FROM slip_history WHERE slip_id = $1 ORDER BY changed_at DESC`,
		slipID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Record
	for rows.Next() {
		var rec Record
		var raw []byte
		if err := rows.Scan(&rec.ID, &rec.SlipID, &rec.Event, &rec.ActorType, &rec.ActorID, &raw, &rec.ChangedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(raw, &rec.Changes); err != nil {
			rec.Changes = map[string]any{}
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}
