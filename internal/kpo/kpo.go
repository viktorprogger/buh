package kpo

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"

	"buh/internal/exchangerate"
)

var (
	ErrNotFound     = errors.New("kpo: not found")
	ErrPendingRates = errors.New("kpo: pending exchange rates")
)

const (
	RateStatusApplied = "applied"
	RateStatusPending = "pending"
)

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
	ID                     uuid.UUID
	KPOBookID              uuid.UUID
	OrdinalNumber          int // = position (1-based display order)
	CollectionDate         time.Time
	InvoiceNumber          string
	Description            string // client name / service description (column 2 of the official KPO form)
	ProductRevenue         float64
	ServiceRevenue         float64
	OriginalProductRevenue float64
	OriginalServiceRevenue float64
	Currency               string
	RateUnit               int
	RateMiddle             float64
	RateEffectiveDate      time.Time
	RateFetchedAt          time.Time
	RateSourceURL          string
	RateStatus             string // applied or pending
	CreatedAt              time.Time
}

func (e Entry) Total() float64       { return e.ProductRevenue + e.ServiceRevenue }
func (e Entry) HasPendingRate() bool { return e.RateStatus == RateStatusPending }

// Repo handles persistence of KPO books and entries.
type Repo struct {
	db        *sql.DB
	converter *exchangerate.Service
}

func NewRepo(db *sql.DB) *Repo {
	return NewRepoWithConverter(db, exchangerate.NewService(exchangerate.NewNBSProvider(nil), &exchangerate.PostgresCache{DB: db}))
}
func NewRepoWithConverter(db *sql.DB, converter *exchangerate.Service) *Repo {
	return &Repo{db: db, converter: converter}
}

// Convert exposes the configured historical-rate service to other domain flows
// that create KPO entries, such as invoice creation.
func (r *Repo) Convert(ctx context.Context, date time.Time, currency string, amount float64) (exchangerate.Conversion, error) {
	return r.converter.Convert(ctx, date, currency, amount)
}

func (r *Repo) convert(ctx context.Context, e *Entry) error {
	e.Currency = strings.ToUpper(strings.TrimSpace(e.Currency))
	if e.Currency == "" {
		e.Currency = "RSD"
	}
	// Callers provide original amounts. Keep this compatibility fallback for
	// older internal callers that only populate the historical RSD fields.
	if e.OriginalProductRevenue == 0 && e.OriginalServiceRevenue == 0 &&
		(e.ProductRevenue != 0 || e.ServiceRevenue != 0) {
		e.OriginalProductRevenue = e.ProductRevenue
		e.OriginalServiceRevenue = e.ServiceRevenue
	}
	if e.Currency == "RSD" {
		e.ProductRevenue = e.OriginalProductRevenue
		e.ServiceRevenue = e.OriginalServiceRevenue
		e.RateUnit = 1
		e.RateMiddle = 1
		e.RateEffectiveDate = e.CollectionDate
		e.RateFetchedAt = time.Time{}
		e.RateSourceURL = ""
		e.RateStatus = RateStatusApplied
		return nil
	}
	product, err := r.converter.Convert(ctx, e.CollectionDate, e.Currency, e.OriginalProductRevenue)
	if err != nil {
		if errors.Is(err, exchangerate.ErrUnavailable) {
			r.markRatePending(e)
			return nil
		}
		return err
	}
	service, err := r.converter.Convert(ctx, e.CollectionDate, e.Currency, e.OriginalServiceRevenue)
	if err != nil {
		if errors.Is(err, exchangerate.ErrUnavailable) {
			r.markRatePending(e)
			return nil
		}
		return err
	}
	e.ProductRevenue = math.Round(product.RSD*100) / 100
	e.ServiceRevenue = math.Round(service.RSD*100) / 100
	e.RateUnit = product.Rate.Unit
	e.RateMiddle = product.Rate.Middle
	e.RateEffectiveDate = product.Rate.EffectiveDate
	e.RateFetchedAt = product.Rate.FetchedAt
	e.RateSourceURL = product.Rate.SourceURL
	e.RateStatus = RateStatusApplied
	return nil
}

func (r *Repo) markRatePending(e *Entry) {
	e.ProductRevenue = 0
	e.ServiceRevenue = 0
	e.RateUnit = 0
	e.RateMiddle = 0
	e.RateEffectiveDate = e.CollectionDate
	e.RateFetchedAt = time.Time{}
	e.RateSourceURL = ""
	e.RateStatus = RateStatusPending
}

// RetryPendingRates attempts to resolve saved entries without changing their source amounts.
// A temporary NBS/cache failure leaves the entry pending for the next attempt.
func (r *Repo) RetryPendingRates(ctx context.Context, bookID uuid.UUID) error {
	entries, err := r.ListEntries(ctx, bookID)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.HasPendingRate() {
			continue
		}
		candidate := entry
		if err := r.convert(ctx, &candidate); err != nil {
			if errors.Is(err, exchangerate.ErrUnavailable) {
				continue
			}
			return err
		}
		if candidate.HasPendingRate() {
			continue
		}
		if _, err := r.db.ExecContext(ctx, `
			UPDATE kpo_entries
			SET product_revenue=$1, service_revenue=$2, rate_unit=$3, rate_middle=$4,
			    rate_effective_date=$5, rate_fetched_at=$6, rate_source_url=$7, rate_status=$8
			WHERE id=$9 AND kpo_book_id=$10 AND rate_status=$11`,
			candidate.ProductRevenue, candidate.ServiceRevenue, candidate.RateUnit, candidate.RateMiddle,
			candidate.RateEffectiveDate, candidate.RateFetchedAt, candidate.RateSourceURL,
			candidate.RateStatus, candidate.ID, bookID, RateStatusPending); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repo) HasPendingRates(ctx context.Context, bookID uuid.UUID) (bool, error) {
	var pending bool
	err := r.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM kpo_entries WHERE kpo_book_id=$1 AND rate_status=$2)`,
		bookID, RateStatusPending).Scan(&pending)
	return pending, err
}

func (r *Repo) HasPendingRatesForEntrepreneurUser(ctx context.Context, userID uuid.UUID) (bool, error) {
	var pending bool
	err := r.db.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM kpo_entries ke
			JOIN kpo_books kb ON kb.id=ke.kpo_book_id
			WHERE kb.entrepreneur_user_id=$1 AND ke.rate_status=$2
		)`, userID, RateStatusPending).Scan(&pending)
	return pending, err
}

func (r *Repo) HasPendingRatesForManagedEntrepreneur(ctx context.Context, entrepreneurID uuid.UUID) (bool, error) {
	var pending bool
	err := r.db.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM kpo_entries ke
			JOIN kpo_books kb ON kb.id=ke.kpo_book_id
			WHERE kb.managed_entrepreneur_id=$1 AND ke.rate_status=$2
		)`, entrepreneurID, RateStatusPending).Scan(&pending)
	return pending, err
}

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
	if err := r.convert(ctx, &e); err != nil {
		return Entry{}, err
	}
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
		`INSERT INTO kpo_entries (kpo_book_id, collection_date, invoice_number, description, product_revenue, service_revenue, original_product_revenue, original_service_revenue, currency, rate_unit, rate_middle, rate_effective_date, rate_fetched_at, rate_source_url, rate_status, position)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)
		 RETURNING id, kpo_book_id, collection_date, invoice_number, description, product_revenue, service_revenue, created_at, position`,
		e.KPOBookID, e.CollectionDate, e.InvoiceNumber, e.Description, e.ProductRevenue, e.ServiceRevenue, e.OriginalProductRevenue, e.OriginalServiceRevenue, e.Currency, e.RateUnit, e.RateMiddle, e.RateEffectiveDate, e.RateFetchedAt, e.RateSourceURL, e.RateStatus, newPos,
	).Scan(&e.ID, &e.KPOBookID, &e.CollectionDate, &e.InvoiceNumber, &e.Description, &e.ProductRevenue, &e.ServiceRevenue, &e.CreatedAt, &e.OrdinalNumber); err != nil {
		return Entry{}, err
	}

	return e, tx.Commit()
}

// UpdateEntry updates an entry's fields and repositions it to match its new date.
func (r *Repo) UpdateEntry(ctx context.Context, e Entry) error {
	if err := r.convert(ctx, &e); err != nil {
		return err
	}
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
		 SET collection_date=$1, invoice_number=$2, description=$3, product_revenue=$4, service_revenue=$5, original_product_revenue=$6, original_service_revenue=$7, currency=$8, rate_unit=$9, rate_middle=$10, rate_effective_date=$11, rate_fetched_at=$12, rate_source_url=$13, rate_status=$14, position=$15
		 WHERE id=$16`,
		e.CollectionDate, e.InvoiceNumber, e.Description, e.ProductRevenue, e.ServiceRevenue, e.OriginalProductRevenue, e.OriginalServiceRevenue, e.Currency, e.RateUnit, e.RateMiddle, e.RateEffectiveDate, e.RateFetchedAt, e.RateSourceURL, e.RateStatus, newPos, e.ID,
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
		`SELECT id,kpo_book_id,collection_date,invoice_number,description,product_revenue,service_revenue,original_product_revenue,original_service_revenue,currency,rate_unit,rate_middle,rate_effective_date,rate_fetched_at,rate_source_url,rate_status,created_at,position
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
			&e.ID, &e.KPOBookID, &e.CollectionDate, &e.InvoiceNumber, &e.Description,
			&e.ProductRevenue, &e.ServiceRevenue, &e.OriginalProductRevenue, &e.OriginalServiceRevenue, &e.Currency, &e.RateUnit, &e.RateMiddle, &e.RateEffectiveDate, &e.RateFetchedAt, &e.RateSourceURL, &e.RateStatus, &e.CreatedAt, &e.OrdinalNumber,
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
	pending, err := r.HasPendingRates(ctx, bookID)
	if err != nil {
		return err
	}
	if pending {
		return ErrPendingRates
	}
	_, err = r.db.ExecContext(ctx,
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

// RollingSumForEntrepreneurUser returns total revenue for an entrepreneur_user in [from, to].
func (r *Repo) RollingSumForEntrepreneurUser(ctx context.Context, userID uuid.UUID, from, to time.Time) (float64, error) {
	var total float64
	err := r.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(ke.product_revenue + ke.service_revenue), 0)
		FROM kpo_books kb
		JOIN kpo_entries ke ON ke.kpo_book_id = kb.id
		WHERE kb.entrepreneur_user_id = $1
		  AND ke.collection_date >= $2 AND ke.collection_date <= $3`,
		userID, from, to,
	).Scan(&total)
	return total, err
}

// RollingSumForManagedEntrepreneur returns total revenue for a managed entrepreneur in [from, to].
func (r *Repo) RollingSumForManagedEntrepreneur(ctx context.Context, managedEntrepreneurID uuid.UUID, from, to time.Time) (float64, error) {
	var total float64
	err := r.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(ke.product_revenue + ke.service_revenue), 0)
		FROM kpo_books kb
		JOIN kpo_entries ke ON ke.kpo_book_id = kb.id
		WHERE kb.managed_entrepreneur_id = $1
		  AND ke.collection_date >= $2 AND ke.collection_date <= $3`,
		managedEntrepreneurID, from, to,
	).Scan(&total)
	return total, err
}

// DailyRevenueForEntrepreneurUser returns revenue grouped by collection date from [from] onwards,
// keyed as "YYYY-MM-DD". Only dates with non-zero revenue are included.
func (r *Repo) DailyRevenueForEntrepreneurUser(ctx context.Context, userID uuid.UUID, from time.Time) (map[string]float64, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT ke.collection_date::date, SUM(ke.product_revenue + ke.service_revenue)
		FROM kpo_books kb
		JOIN kpo_entries ke ON ke.kpo_book_id = kb.id
		WHERE kb.entrepreneur_user_id = $1 AND ke.collection_date >= $2
		GROUP BY ke.collection_date::date`,
		userID, from,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[string]float64)
	for rows.Next() {
		var date time.Time
		var total float64
		if err := rows.Scan(&date, &total); err != nil {
			return nil, err
		}
		result[date.Format("2006-01-02")] = total
	}
	return result, rows.Err()
}

// SumForEntrepreneurUser returns the total revenue for the given entrepreneur_user+year.
// It does not create the book if it is absent.
func (r *Repo) SumForEntrepreneurUser(ctx context.Context, entrepreneurUserID uuid.UUID, year int) (total float64, hasEntries bool, err error) {
	var count int
	err = r.db.QueryRowContext(ctx, `
		SELECT COUNT(ke.id), COALESCE(SUM(ke.product_revenue + ke.service_revenue), 0)
		FROM kpo_books kb
		JOIN kpo_entries ke ON ke.kpo_book_id = kb.id
		WHERE kb.entrepreneur_user_id = $1 AND kb.year = $2`,
		entrepreneurUserID, year,
	).Scan(&count, &total)
	hasEntries = count > 0
	return
}

// SumForYear returns the total revenue and whether any entries exist for the given
// managed_entrepreneur+year. It does not create the book if it is absent.
// SumForYearBulk returns current-year revenue per managed entrepreneur ID.
// Entrepreneurs with no entries are absent from the map (zero value applies).
func (r *Repo) SumForYearBulk(ctx context.Context, ids []uuid.UUID, year int) (map[uuid.UUID]float64, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT kb.managed_entrepreneur_id, COALESCE(SUM(ke.product_revenue + ke.service_revenue), 0)
		FROM kpo_books kb
		JOIN kpo_entries ke ON ke.kpo_book_id = kb.id
		WHERE kb.managed_entrepreneur_id = ANY($1) AND kb.year = $2
		GROUP BY kb.managed_entrepreneur_id`,
		pq.Array(ids), year,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[uuid.UUID]float64, len(ids))
	for rows.Next() {
		var id uuid.UUID
		var total float64
		if err := rows.Scan(&id, &total); err != nil {
			return nil, err
		}
		result[id] = total
	}
	return result, rows.Err()
}

// RollingSumBulk returns rolling revenue per managed entrepreneur ID in [from, to].
// Entrepreneurs with no entries in the window are absent from the map.
func (r *Repo) RollingSumBulk(ctx context.Context, ids []uuid.UUID, from, to time.Time) (map[uuid.UUID]float64, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT kb.managed_entrepreneur_id, COALESCE(SUM(ke.product_revenue + ke.service_revenue), 0)
		FROM kpo_books kb
		JOIN kpo_entries ke ON ke.kpo_book_id = kb.id
		WHERE kb.managed_entrepreneur_id = ANY($1)
		  AND ke.collection_date >= $2 AND ke.collection_date <= $3
		GROUP BY kb.managed_entrepreneur_id`,
		pq.Array(ids), from, to,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[uuid.UUID]float64, len(ids))
	for rows.Next() {
		var id uuid.UUID
		var total float64
		if err := rows.Scan(&id, &total); err != nil {
			return nil, err
		}
		result[id] = total
	}
	return result, rows.Err()
}

// CopyToEntrepreneurUser copies all KPO books and entries from a managed entrepreneur to an
// entrepreneur user. Books for years that already exist on the entrepreneur side are skipped.
func (r *Repo) CopyToEntrepreneurUser(ctx context.Context, managedEntrepreneurID, entrepreneurUserID uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `
		WITH new_books AS (
			INSERT INTO kpo_books (entrepreneur_user_id, year, finalized_at)
			SELECT $2, year, finalized_at
			FROM kpo_books
			WHERE managed_entrepreneur_id = $1
			ON CONFLICT (entrepreneur_user_id, year) WHERE entrepreneur_user_id IS NOT NULL
			DO NOTHING
			RETURNING id, year
		)
		INSERT INTO kpo_entries (kpo_book_id, position, collection_date, invoice_number, description, product_revenue, service_revenue, original_product_revenue, original_service_revenue, currency, rate_unit, rate_middle, rate_effective_date, rate_fetched_at, rate_source_url, rate_status)
		SELECT nb.id,ke.position,ke.collection_date,ke.invoice_number,ke.description,ke.product_revenue,ke.service_revenue,ke.original_product_revenue,ke.original_service_revenue,ke.currency,ke.rate_unit,ke.rate_middle,ke.rate_effective_date,ke.rate_fetched_at,ke.rate_source_url,ke.rate_status
		FROM kpo_books sb
		JOIN kpo_entries ke ON ke.kpo_book_id = sb.id
		JOIN new_books nb ON nb.year = sb.year
		WHERE sb.managed_entrepreneur_id = $1`,
		managedEntrepreneurID, entrepreneurUserID,
	)
	return err
}

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
