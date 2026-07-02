package invoice

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
)

var ErrNotFound = errors.New("invoice: not found")

type Type string

const (
	TypeStandard Type = "standard"
	TypeAdvance  Type = "advance"
)

// FXRates maps currency codes to their RSD equivalent (hardcoded, to be replaced with live API).
var FXRates = map[string]float64{
	"RSD": 1.0,
	"EUR": 117.2,
	"USD": 108.0,
	"CHF": 122.0,
	"GBP": 138.0,
}

func ToRSD(amount float64, currency string) float64 {
	rate, ok := FXRates[currency]
	if !ok {
		rate = 1.0
	}
	return amount * rate
}

type Invoice struct {
	ID                   uuid.UUID
	EntrepreneurUserID   uuid.UUID
	ClientID             *uuid.UUID
	ClientName           string
	InvoiceType          Type
	InvoiceNumber        string
	IssueDate            time.Time
	PeriodStart          sql.NullTime
	PeriodEnd            sql.NullTime
	DueDate              sql.NullTime
	Currency             string
	Notes                string
	TotalRSD             float64
	BankAccountID        *uuid.UUID
	CorrespondentBankID  *uuid.UUID
	Language             string // "sr" or "en"; defaults to "sr"
	NoVAT                bool   // include VAT disclaimer (Article 12)
	NoSign               bool   // include "generated without stamp" note
	CreatedAt            time.Time
}

type Item struct {
	ID          uuid.UUID
	InvoiceID   uuid.UUID
	Description string
	Quantity    float64
	UnitPrice   float64
	DiscountPct float64
	IsProduct   bool
	Position    int
}

func (it Item) LineTotal() float64 {
	gross := it.Quantity * it.UnitPrice
	return gross * (1 - it.DiscountPct/100)
}

type Repo struct{ db *sql.DB }

func NewRepo(db *sql.DB) *Repo { return &Repo{db: db} }

const invCols = `id, entrepreneur_user_id, client_id, client_name, invoice_type, invoice_number,
	issue_date, period_start, period_end, due_date, currency, notes, total_rsd,
	bank_account_id, correspondent_bank_id, language, no_vat, no_sign, created_at`

func scanInv(row interface{ Scan(...any) error }, inv *Invoice) error {
	var clientID sql.NullString
	var bankAccountID sql.NullString
	var correspondentBankID sql.NullString
	err := row.Scan(
		&inv.ID, &inv.EntrepreneurUserID, &clientID, &inv.ClientName,
		&inv.InvoiceType, &inv.InvoiceNumber,
		&inv.IssueDate, &inv.PeriodStart, &inv.PeriodEnd, &inv.DueDate,
		&inv.Currency, &inv.Notes, &inv.TotalRSD,
		&bankAccountID, &correspondentBankID,
		&inv.Language, &inv.NoVAT, &inv.NoSign,
		&inv.CreatedAt,
	)
	if err != nil {
		return err
	}
	if clientID.Valid {
		id, err := uuid.Parse(clientID.String)
		if err == nil {
			inv.ClientID = &id
		}
	}
	if bankAccountID.Valid {
		id, err := uuid.Parse(bankAccountID.String)
		if err == nil {
			inv.BankAccountID = &id
		}
	}
	if correspondentBankID.Valid {
		id, err := uuid.Parse(correspondentBankID.String)
		if err == nil {
			inv.CorrespondentBankID = &id
		}
	}
	return nil
}

func (r *Repo) Create(ctx context.Context, inv Invoice, items []Item) (Invoice, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Invoice{}, err
	}
	defer tx.Rollback()

	var clientID interface{}
	if inv.ClientID != nil {
		clientID = *inv.ClientID
	}
	var bankAccountID interface{}
	if inv.BankAccountID != nil {
		bankAccountID = *inv.BankAccountID
	}
	var correspondentBankID interface{}
	if inv.CorrespondentBankID != nil {
		correspondentBankID = *inv.CorrespondentBankID
	}

	lang := inv.Language
	if lang == "" {
		lang = "sr"
	}
	err = scanInv(tx.QueryRowContext(ctx,
		`INSERT INTO invoices (entrepreneur_user_id, client_id, client_name, invoice_type, invoice_number,
		 issue_date, period_start, period_end, due_date, currency, notes, total_rsd,
		 bank_account_id, correspondent_bank_id, language, no_vat, no_sign)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)
		 RETURNING `+invCols,
		inv.EntrepreneurUserID, clientID, inv.ClientName, string(inv.InvoiceType), inv.InvoiceNumber,
		inv.IssueDate, nullTime(inv.PeriodStart), nullTime(inv.PeriodEnd), nullTime(inv.DueDate),
		inv.Currency, inv.Notes, inv.TotalRSD,
		bankAccountID, correspondentBankID, lang, inv.NoVAT, inv.NoSign,
	), &inv)
	if err != nil {
		return Invoice{}, err
	}

	for i, it := range items {
		_, err = tx.ExecContext(ctx,
			`INSERT INTO invoice_items (invoice_id, description, quantity, unit_price, discount_pct, is_product, position)
			 VALUES ($1,$2,$3,$4,$5,$6,$7)`,
			inv.ID, it.Description, it.Quantity, it.UnitPrice, it.DiscountPct, it.IsProduct, i+1,
		)
		if err != nil {
			return Invoice{}, err
		}
	}

	return inv, tx.Commit()
}

func (r *Repo) FindByID(ctx context.Context, id uuid.UUID) (Invoice, []Item, error) {
	var inv Invoice
	err := scanInv(r.db.QueryRowContext(ctx,
		`SELECT `+invCols+` FROM invoices WHERE id=$1`, id,
	), &inv)
	if errors.Is(err, sql.ErrNoRows) {
		return Invoice{}, nil, ErrNotFound
	}
	if err != nil {
		return Invoice{}, nil, err
	}
	items, err := r.listItems(ctx, id)
	return inv, items, err
}

func (r *Repo) ListByEntrepreneurUser(ctx context.Context, entrepreneurUserID uuid.UUID) ([]Invoice, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+invCols+` FROM invoices WHERE entrepreneur_user_id=$1 ORDER BY issue_date DESC, created_at DESC`,
		entrepreneurUserID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Invoice
	for rows.Next() {
		var inv Invoice
		if err := scanInv(rows, &inv); err != nil {
			return nil, err
		}
		out = append(out, inv)
	}
	return out, rows.Err()
}

func (r *Repo) ListAdvanceByEntrepreneurUserYear(ctx context.Context, entrepreneurUserID uuid.UUID, year int) ([]Invoice, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+invCols+` FROM invoices
		 WHERE entrepreneur_user_id=$1 AND invoice_type='advance' AND EXTRACT(YEAR FROM issue_date)=$2
		 ORDER BY issue_date`,
		entrepreneurUserID, year,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Invoice
	for rows.Next() {
		var inv Invoice
		if err := scanInv(rows, &inv); err != nil {
			return nil, err
		}
		out = append(out, inv)
	}
	return out, rows.Err()
}

func (r *Repo) listItems(ctx context.Context, invoiceID uuid.UUID) ([]Item, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, invoice_id, description, quantity, unit_price, discount_pct, is_product, position
		 FROM invoice_items WHERE invoice_id=$1 ORDER BY position`,
		invoiceID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Item
	for rows.Next() {
		var it Item
		if err := rows.Scan(&it.ID, &it.InvoiceID, &it.Description, &it.Quantity,
			&it.UnitPrice, &it.DiscountPct, &it.IsProduct, &it.Position); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

func nullTime(t sql.NullTime) interface{} {
	if !t.Valid {
		return nil
	}
	return t.Time
}
