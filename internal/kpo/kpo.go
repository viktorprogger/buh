package kpo

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
)

var ErrNotFound = errors.New("kpo: not found")

// Book is a KPO ledger for one entrepreneur+year.
type Book struct {
	ID             uuid.UUID
	EntrepreneurID uuid.UUID
	Year           int
	FinalizedAt    *time.Time
	CreatedAt      time.Time
}

func (b Book) IsFinalized() bool { return b.FinalizedAt != nil }

// FinalizedAtStr returns the finalization date formatted for display, or "".
func (b Book) FinalizedAtStr() string {
	if b.FinalizedAt == nil {
		return ""
	}
	return b.FinalizedAt.Format("02.01.2006.")
}

// Entry is one line in a KPO book.
type Entry struct {
	ID             uuid.UUID
	KPOBookID      uuid.UUID
	OrdinalNumber  int
	CollectionDate time.Time
	InvoiceNumber  string
	ProductRevenue float64
	ServiceRevenue float64
	CreatedAt      time.Time
}

func (e Entry) Total() float64 { return e.ProductRevenue + e.ServiceRevenue }

// Repo handles persistence of KPO books and entries.
type Repo struct {
	db *sql.DB
}

func NewRepo(db *sql.DB) *Repo { return &Repo{db: db} }

// FindOrCreate returns the KPO book for the given entrepreneur+year, creating it if absent.
func (r *Repo) FindOrCreate(ctx context.Context, entrepreneurID uuid.UUID, year int) (Book, error) {
	var b Book
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO kpo_books (entrepreneur_id, year)
		 VALUES ($1, $2)
		 ON CONFLICT (entrepreneur_id, year) DO UPDATE SET year = EXCLUDED.year
		 RETURNING id, entrepreneur_id, year, finalized_at, created_at`,
		entrepreneurID, year,
	).Scan(&b.ID, &b.EntrepreneurID, &b.Year, &b.FinalizedAt, &b.CreatedAt)
	return b, err
}

// ListByEntrepreneur returns all KPO books for the entrepreneur, newest year first.
func (r *Repo) ListByEntrepreneur(ctx context.Context, entrepreneurID uuid.UUID) ([]Book, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, entrepreneur_id, year, finalized_at, created_at
		 FROM kpo_books WHERE entrepreneur_id = $1 ORDER BY year DESC`,
		entrepreneurID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Book
	for rows.Next() {
		var b Book
		if err := rows.Scan(&b.ID, &b.EntrepreneurID, &b.Year, &b.FinalizedAt, &b.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// FindByID returns a single book or ErrNotFound.
func (r *Repo) FindByID(ctx context.Context, id uuid.UUID) (Book, error) {
	var b Book
	err := r.db.QueryRowContext(ctx,
		`SELECT id, entrepreneur_id, year, finalized_at, created_at
		 FROM kpo_books WHERE id = $1`, id,
	).Scan(&b.ID, &b.EntrepreneurID, &b.Year, &b.FinalizedAt, &b.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Book{}, ErrNotFound
	}
	return b, err
}

// AddEntry inserts a new entry into the book.
func (r *Repo) AddEntry(ctx context.Context, e Entry) (Entry, error) {
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO kpo_entries (kpo_book_id, collection_date, invoice_number, product_revenue, service_revenue)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, kpo_book_id, collection_date, invoice_number, product_revenue, service_revenue, created_at`,
		e.KPOBookID, e.CollectionDate, e.InvoiceNumber, e.ProductRevenue, e.ServiceRevenue,
	).Scan(&e.ID, &e.KPOBookID, &e.CollectionDate, &e.InvoiceNumber, &e.ProductRevenue, &e.ServiceRevenue, &e.CreatedAt)
	return e, err
}

// ListEntries returns all entries for a book in chronological order with ordinal numbers.
func (r *Repo) ListEntries(ctx context.Context, kpoBookID uuid.UUID) ([]Entry, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, kpo_book_id, collection_date, invoice_number, product_revenue, service_revenue, created_at,
		        ROW_NUMBER() OVER (ORDER BY created_at, id) AS ordinal
		 FROM kpo_entries WHERE kpo_book_id = $1 ORDER BY created_at, id`,
		kpoBookID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Entry
	for rows.Next() {
		var e Entry
		if err := rows.Scan(
			&e.ID, &e.KPOBookID, &e.CollectionDate, &e.InvoiceNumber,
			&e.ProductRevenue, &e.ServiceRevenue, &e.CreatedAt, &e.OrdinalNumber,
		); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// DeleteEntry removes an entry by ID.
func (r *Repo) DeleteEntry(ctx context.Context, entryID uuid.UUID) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM kpo_entries WHERE id = $1`, entryID)
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

// Finalize marks a book as finalized (read-only). No-op if already finalized.
func (r *Repo) Finalize(ctx context.Context, bookID uuid.UUID) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE kpo_books SET finalized_at = now() WHERE id = $1 AND finalized_at IS NULL`,
		bookID,
	)
	return err
}

// Unfinalize removes the finalization mark so entries can be edited again.
func (r *Repo) Unfinalize(ctx context.Context, bookID uuid.UUID) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE kpo_books SET finalized_at = NULL WHERE id = $1`,
		bookID,
	)
	return err
}
