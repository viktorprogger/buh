package kpo

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
)

var ErrNotFound = errors.New("kpo: not found")

// Book is a KPO ledger for one owner+year. Owner is either a managed_entrepreneur
// (accountant side) XOR an entrepreneur_user (entrepreneur side), enforced by DB constraint.
type Book struct {
	ID                    uuid.UUID
	ManagedEntrepreneurID *uuid.UUID
	EntrepreneurUserID    *uuid.UUID
	Year                  int
	FinalizedAt           *time.Time
	MergedAt              *time.Time
	CreatedAt             time.Time
}

func (b Book) IsFinalized() bool { return b.FinalizedAt != nil }
func (b Book) IsMerged() bool    { return b.MergedAt != nil }

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
	OrdinalNumber  int // = position (1-based display order)
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

const bookCols = `id, managed_entrepreneur_id, entrepreneur_user_id, year, finalized_at, merged_at, created_at`

func scanBook(row interface{ Scan(...any) error }, b *Book) error {
	var managedID sql.NullString
	var userID sql.NullString
	err := row.Scan(&b.ID, &managedID, &userID, &b.Year, &b.FinalizedAt, &b.MergedAt, &b.CreatedAt)
	if err != nil {
		return err
	}
	if managedID.Valid {
		id, err := uuid.Parse(managedID.String)
		if err == nil {
			b.ManagedEntrepreneurID = &id
		}
	}
	if userID.Valid {
		id, err := uuid.Parse(userID.String)
		if err == nil {
			b.EntrepreneurUserID = &id
		}
	}
	return nil
}

// FindOrCreateForAccountant returns (or creates) the KPO book for a managed entrepreneur+year.
func (r *Repo) FindOrCreateForAccountant(ctx context.Context, managedEntrepreneurID uuid.UUID, year int) (Book, error) {
	var b Book
	err := scanBook(r.db.QueryRowContext(ctx,
		`INSERT INTO kpo_books (managed_entrepreneur_id, year)
		 VALUES ($1, $2)
		 ON CONFLICT (managed_entrepreneur_id, year) WHERE managed_entrepreneur_id IS NOT NULL
		 DO NOTHING
		 RETURNING `+bookCols,
		managedEntrepreneurID, year,
	), &b)
	if errors.Is(err, sql.ErrNoRows) {
		return r.FindByYear(ctx, managedEntrepreneurID, year)
	}
	return b, err
}

// FindOrCreateForEntrepreneur returns (or creates) the KPO book for an entrepreneur_user+year.
func (r *Repo) FindOrCreateForEntrepreneur(ctx context.Context, entrepreneurUserID uuid.UUID, year int) (Book, error) {
	var b Book
	err := scanBook(r.db.QueryRowContext(ctx,
		`INSERT INTO kpo_books (entrepreneur_user_id, year)
		 VALUES ($1, $2)
		 ON CONFLICT (entrepreneur_user_id, year) WHERE entrepreneur_user_id IS NOT NULL
		 DO NOTHING
		 RETURNING `+bookCols,
		entrepreneurUserID, year,
	), &b)
	if errors.Is(err, sql.ErrNoRows) {
		return r.FindEntrepreneurBook(ctx, entrepreneurUserID, year)
	}
	return b, err
}

// FindByYear returns the accountant-side book for the given managed_entrepreneur+year, or ErrNotFound.
func (r *Repo) FindByYear(ctx context.Context, managedEntrepreneurID uuid.UUID, year int) (Book, error) {
	var b Book
	err := scanBook(r.db.QueryRowContext(ctx,
		`SELECT `+bookCols+`
		 FROM kpo_books WHERE managed_entrepreneur_id = $1 AND year = $2`,
		managedEntrepreneurID, year,
	), &b)
	if errors.Is(err, sql.ErrNoRows) {
		return Book{}, ErrNotFound
	}
	return b, err
}

// FindEntrepreneurBook returns the entrepreneur-side KPO book for the given user+year, or ErrNotFound.
func (r *Repo) FindEntrepreneurBook(ctx context.Context, entrepreneurUserID uuid.UUID, year int) (Book, error) {
	var b Book
	err := scanBook(r.db.QueryRowContext(ctx,
		`SELECT `+bookCols+`
		 FROM kpo_books WHERE entrepreneur_user_id = $1 AND year = $2`,
		entrepreneurUserID, year,
	), &b)
	if errors.Is(err, sql.ErrNoRows) {
		return Book{}, ErrNotFound
	}
	return b, err
}

// ListByManagedEntrepreneur returns all accountant-side KPO books for the entrepreneur, newest year first.
func (r *Repo) ListByManagedEntrepreneur(ctx context.Context, managedEntrepreneurID uuid.UUID) ([]Book, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+bookCols+`
		 FROM kpo_books WHERE managed_entrepreneur_id = $1 ORDER BY year DESC`,
		managedEntrepreneurID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Book
	for rows.Next() {
		var b Book
		if err := scanBook(rows, &b); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// ListByEntrepreneurUser returns all entrepreneur-side KPO books, newest year first.
func (r *Repo) ListByEntrepreneurUser(ctx context.Context, entrepreneurUserID uuid.UUID) ([]Book, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+bookCols+`
		 FROM kpo_books WHERE entrepreneur_user_id = $1 ORDER BY year DESC`,
		entrepreneurUserID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Book
	for rows.Next() {
		var b Book
		if err := scanBook(rows, &b); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// FindByID returns a single book or ErrNotFound.
func (r *Repo) FindByID(ctx context.Context, id uuid.UUID) (Book, error) {
	var b Book
	err := scanBook(r.db.QueryRowContext(ctx,
		`SELECT `+bookCols+` FROM kpo_books WHERE id = $1`, id,
	), &b)
	if errors.Is(err, sql.ErrNoRows) {
		return Book{}, ErrNotFound
	}
	return b, err
}

// AddEntry inserts a new entry at the date-appropriate position, shifting later entries up.
func (r *Repo) AddEntry(ctx context.Context, e Entry) (Entry, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Entry{}, err
	}
	defer tx.Rollback()

	// Lock the book row so concurrent AddEntry calls for the same book are serialized.
	if _, err = tx.ExecContext(ctx, `SELECT id FROM kpo_books WHERE id = $1 FOR UPDATE`, e.KPOBookID); err != nil {
		return Entry{}, err
	}

	var newPos int
	if err = tx.QueryRowContext(ctx,
		`SELECT COUNT(*) + 1 FROM kpo_entries WHERE kpo_book_id = $1 AND collection_date <= $2`,
		e.KPOBookID, e.CollectionDate,
	).Scan(&newPos); err != nil {
		return Entry{}, err
	}

	if _, err = tx.ExecContext(ctx,
		`UPDATE kpo_entries SET position = position + 1 WHERE kpo_book_id = $1 AND position >= $2`,
		e.KPOBookID, newPos,
	); err != nil {
		return Entry{}, err
	}

	if err = tx.QueryRowContext(ctx,
		`INSERT INTO kpo_entries (kpo_book_id, collection_date, invoice_number, product_revenue, service_revenue, position)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, kpo_book_id, collection_date, invoice_number, product_revenue, service_revenue, created_at, position`,
		e.KPOBookID, e.CollectionDate, e.InvoiceNumber, e.ProductRevenue, e.ServiceRevenue, newPos,
	).Scan(&e.ID, &e.KPOBookID, &e.CollectionDate, &e.InvoiceNumber, &e.ProductRevenue, &e.ServiceRevenue, &e.CreatedAt, &e.OrdinalNumber); err != nil {
		return Entry{}, err
	}

	return e, tx.Commit()
}

// UpdateEntry updates an entry's fields and repositions it to match its new date.
func (r *Repo) UpdateEntry(ctx context.Context, e Entry) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var bookID uuid.UUID
	var oldPos int
	if err = tx.QueryRowContext(ctx,
		`SELECT kpo_book_id, position FROM kpo_entries WHERE id = $1`,
		e.ID,
	).Scan(&bookID, &oldPos); errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	} else if err != nil {
		return err
	}
	if bookID != e.KPOBookID {
		return ErrNotFound
	}

	// Temporarily park at 0 so position sequence stays gapless during reshuffling.
	if _, err = tx.ExecContext(ctx,
		`UPDATE kpo_entries SET position = 0 WHERE id = $1`, e.ID,
	); err != nil {
		return err
	}

	// Close the gap left by the old position.
	if _, err = tx.ExecContext(ctx,
		`UPDATE kpo_entries SET position = position - 1 WHERE kpo_book_id = $1 AND position > $2`,
		bookID, oldPos,
	); err != nil {
		return err
	}

	// Find new insertion position based on updated date (excluding self which is at 0).
	var newPos int
	if err = tx.QueryRowContext(ctx,
		`SELECT COUNT(*) + 1 FROM kpo_entries WHERE kpo_book_id = $1 AND id != $2 AND collection_date <= $3`,
		bookID, e.ID, e.CollectionDate,
	).Scan(&newPos); err != nil {
		return err
	}

	// Make room at the new position.
	if _, err = tx.ExecContext(ctx,
		`UPDATE kpo_entries SET position = position + 1 WHERE kpo_book_id = $1 AND position >= $2`,
		bookID, newPos,
	); err != nil {
		return err
	}

	if _, err = tx.ExecContext(ctx,
		`UPDATE kpo_entries
		 SET collection_date = $1, invoice_number = $2, product_revenue = $3, service_revenue = $4, position = $5
		 WHERE id = $6`,
		e.CollectionDate, e.InvoiceNumber, e.ProductRevenue, e.ServiceRevenue, newPos, e.ID,
	); err != nil {
		return err
	}

	return tx.Commit()
}

// ReorderEntries assigns positions 1..N to entries in the given order.
func (r *Repo) ReorderEntries(ctx context.Context, bookID uuid.UUID, ids []uuid.UUID) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for i, id := range ids {
		if _, err = tx.ExecContext(ctx,
			`UPDATE kpo_entries SET position = $1 WHERE id = $2 AND kpo_book_id = $3`,
			i+1, id, bookID,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ListEntries returns all entries for a book ordered by position.
func (r *Repo) ListEntries(ctx context.Context, kpoBookID uuid.UUID) ([]Entry, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, kpo_book_id, collection_date, invoice_number, product_revenue, service_revenue, created_at, position
		 FROM kpo_entries WHERE kpo_book_id = $1 ORDER BY position`,
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

// DeleteEntry removes an entry (that must belong to bookID) and closes the gap in positions.
func (r *Repo) DeleteEntry(ctx context.Context, bookID, entryID uuid.UUID) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var pos int
	if err = tx.QueryRowContext(ctx,
		`DELETE FROM kpo_entries WHERE id = $1 AND kpo_book_id = $2 RETURNING position`,
		entryID, bookID,
	).Scan(&pos); errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	} else if err != nil {
		return err
	}

	if _, err = tx.ExecContext(ctx,
		`UPDATE kpo_entries SET position = position - 1 WHERE kpo_book_id = $1 AND position > $2`,
		bookID, pos,
	); err != nil {
		return err
	}

	return tx.Commit()
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

// MarkMerged sets merged_at on a book (entrepreneur-side, after accountant copies entries).
func (r *Repo) MarkMerged(ctx context.Context, bookID uuid.UUID) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE kpo_books SET merged_at = now() WHERE id = $1`,
		bookID,
	)
	return err
}

// SumForYear returns the total revenue and whether any entries exist for the given
// managed_entrepreneur+year. It does not create the book if it is absent.
func (r *Repo) SumForYear(ctx context.Context, managedEntrepreneurID uuid.UUID, year int) (total float64, hasEntries bool, err error) {
	var count int
	err = r.db.QueryRowContext(ctx, `
		SELECT COUNT(ke.id), COALESCE(SUM(ke.product_revenue + ke.service_revenue), 0)
		FROM kpo_books kb
		JOIN kpo_entries ke ON ke.kpo_book_id = kb.id
		WHERE kb.managed_entrepreneur_id = $1 AND kb.year = $2`,
		managedEntrepreneurID, year,
	).Scan(&count, &total)
	hasEntries = count > 0
	return
}
