package invitation

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
)

var ErrNotFound = errors.New("invitation: not found")
var ErrExpired = errors.New("invitation: expired")
var ErrAlreadyAccepted = errors.New("invitation: already accepted")

type InviterType string

const (
	InviterTypeAccountant   InviterType = "accountant"
	InviterTypeEntrepreneur InviterType = "entrepreneur"
)

type Invitation struct {
	ID                    uuid.UUID
	Token                 string
	InviterType           InviterType
	InviterID             uuid.UUID
	InviteeEmail          *string
	ManagedEntrepreneurID *uuid.UUID
	AcceptedAt            *time.Time
	ExpiresAt             time.Time
	CreatedAt             time.Time
}

type Repo struct{ db *sql.DB }

func NewRepo(db *sql.DB) *Repo { return &Repo{db: db} }

func (r *Repo) Create(ctx context.Context, inv Invitation) (Invitation, error) {
	var inviteeEmail interface{}
	if inv.InviteeEmail != nil {
		inviteeEmail = *inv.InviteeEmail
	}
	var managedEntrepreneurID interface{}
	if inv.ManagedEntrepreneurID != nil {
		managedEntrepreneurID = *inv.ManagedEntrepreneurID
	}

	err := r.db.QueryRowContext(ctx,
		`INSERT INTO invitations (inviter_type, inviter_id, invitee_email, managed_entrepreneur_id)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, token, inviter_type, inviter_id, invitee_email, managed_entrepreneur_id,
		           accepted_at, expires_at, created_at`,
		string(inv.InviterType), inv.InviterID, inviteeEmail, managedEntrepreneurID,
	).Scan(
		&inv.ID, &inv.Token, &inv.InviterType, &inv.InviterID,
		scanNullString(&inv.InviteeEmail), scanNullUUID(&inv.ManagedEntrepreneurID),
		&inv.AcceptedAt, &inv.ExpiresAt, &inv.CreatedAt,
	)
	return inv, err
}

func (r *Repo) FindByToken(ctx context.Context, token string) (Invitation, error) {
	var inv Invitation
	err := r.db.QueryRowContext(ctx,
		`SELECT id, token, inviter_type, inviter_id, invitee_email, managed_entrepreneur_id,
		        accepted_at, expires_at, created_at
		 FROM invitations WHERE token = $1`, token,
	).Scan(
		&inv.ID, &inv.Token, &inv.InviterType, &inv.InviterID,
		scanNullString(&inv.InviteeEmail), scanNullUUID(&inv.ManagedEntrepreneurID),
		&inv.AcceptedAt, &inv.ExpiresAt, &inv.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Invitation{}, ErrNotFound
	}
	return inv, err
}

func (inv Invitation) Validate() error {
	if inv.AcceptedAt != nil {
		return ErrAlreadyAccepted
	}
	if time.Now().After(inv.ExpiresAt) {
		return ErrExpired
	}
	return nil
}

func (r *Repo) Accept(ctx context.Context, id uuid.UUID) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE invitations SET accepted_at = now() WHERE id = $1 AND accepted_at IS NULL`, id,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrAlreadyAccepted
	}
	return nil
}

func (r *Repo) ListPendingForAccountant(ctx context.Context, accountantID uuid.UUID) ([]Invitation, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, token, inviter_type, inviter_id, invitee_email, managed_entrepreneur_id,
		        accepted_at, expires_at, created_at
		 FROM invitations
		 WHERE inviter_type = 'accountant' AND inviter_id = $1
		   AND accepted_at IS NULL AND expires_at > now()
		 ORDER BY created_at DESC`,
		accountantID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Invitation
	for rows.Next() {
		var inv Invitation
		if err := rows.Scan(
			&inv.ID, &inv.Token, &inv.InviterType, &inv.InviterID,
			scanNullString(&inv.InviteeEmail), scanNullUUID(&inv.ManagedEntrepreneurID),
			&inv.AcceptedAt, &inv.ExpiresAt, &inv.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, inv)
	}
	return out, rows.Err()
}

// scanNullString returns a *string destination suitable for sql.Scan.
func scanNullString(dest **string) interface{} {
	return &nullableString{p: dest}
}

type nullableString struct{ p **string }

func (n *nullableString) Scan(src interface{}) error {
	if src == nil {
		*n.p = nil
		return nil
	}
	s := ""
	switch v := src.(type) {
	case string:
		s = v
	case []byte:
		s = string(v)
	default:
		return errors.New("unexpected type for nullable string")
	}
	*n.p = &s
	return nil
}

// scanNullUUID returns a *uuid.UUID destination suitable for sql.Scan.
func scanNullUUID(dest **uuid.UUID) interface{} {
	return &nullableUUID{p: dest}
}

type nullableUUID struct{ p **uuid.UUID }

func (n *nullableUUID) Scan(src interface{}) error {
	if src == nil {
		*n.p = nil
		return nil
	}
	var s string
	switch v := src.(type) {
	case string:
		s = v
	case []byte:
		s = string(v)
	default:
		return errors.New("unexpected type for nullable uuid")
	}
	id, err := uuid.Parse(s)
	if err != nil {
		return err
	}
	*n.p = &id
	return nil
}
