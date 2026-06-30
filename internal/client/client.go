package client

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
)

var ErrNotFound = errors.New("client: not found")

type Client struct {
	ID                 uuid.UUID
	EntrepreneurID     uuid.UUID
	Name               string
	PIB                string
	RegistrationNumber string
	Email              string
	Address            string
	IsForeign          bool
	CreatedAt          time.Time
}

type Repo struct{ db *sql.DB }

func NewRepo(db *sql.DB) *Repo { return &Repo{db: db} }

const selectCols = `id, entrepreneur_id, name, pib, registration_number, email, address, is_foreign, created_at`

func scan(row interface{ Scan(...any) error }, c *Client) error {
	return row.Scan(&c.ID, &c.EntrepreneurID, &c.Name, &c.PIB,
		&c.RegistrationNumber, &c.Email, &c.Address, &c.IsForeign, &c.CreatedAt)
}

func (r *Repo) Create(ctx context.Context, c Client) (Client, error) {
	err := scan(r.db.QueryRowContext(ctx,
		`INSERT INTO clients (entrepreneur_id, name, pib, registration_number, email, address, is_foreign)
		 VALUES ($1,$2,$3,$4,$5,$6,$7)
		 RETURNING `+selectCols,
		c.EntrepreneurID, c.Name, c.PIB, c.RegistrationNumber, c.Email, c.Address, c.IsForeign,
	), &c)
	return c, err
}

func (r *Repo) Update(ctx context.Context, c Client) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE clients SET name=$1, pib=$2, registration_number=$3, email=$4, address=$5, is_foreign=$6
		 WHERE id=$7`,
		c.Name, c.PIB, c.RegistrationNumber, c.Email, c.Address, c.IsForeign, c.ID,
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

func (r *Repo) Delete(ctx context.Context, id uuid.UUID) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM clients WHERE id=$1`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repo) FindByID(ctx context.Context, id uuid.UUID) (Client, error) {
	var c Client
	err := scan(r.db.QueryRowContext(ctx,
		`SELECT `+selectCols+` FROM clients WHERE id=$1`, id,
	), &c)
	if errors.Is(err, sql.ErrNoRows) {
		return Client{}, ErrNotFound
	}
	return c, err
}

func (r *Repo) ListByEntrepreneur(ctx context.Context, entrepreneurID uuid.UUID) ([]Client, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+selectCols+` FROM clients WHERE entrepreneur_id=$1 ORDER BY name`, entrepreneurID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Client
	for rows.Next() {
		var c Client
		if err := scan(rows, &c); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *Repo) Search(ctx context.Context, entrepreneurID uuid.UUID, q string) ([]Client, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+selectCols+` FROM clients
		 WHERE entrepreneur_id=$1 AND name ILIKE '%'||$2||'%'
		 ORDER BY name LIMIT 20`,
		entrepreneurID, q,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Client
	for rows.Next() {
		var c Client
		if err := scan(rows, &c); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
