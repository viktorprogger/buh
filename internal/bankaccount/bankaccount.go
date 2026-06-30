package bankaccount

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
)

var ErrNotFound = errors.New("bank account not found")

type Type string

const (
	TypeLocal   Type = "local"
	TypeForeign Type = "foreign"
)

type BankAccount struct {
	ID             uuid.UUID
	EntrepreneurID uuid.UUID
	AccountType    Type
	BankName       string
	AccountNumber  string
	IBAN           string
	SWIFT          string
	CreatedAt      time.Time

	CorrespondentBanks []CorrespondentBank
}

func (a BankAccount) DisplayLabel() string {
	if a.AccountType == TypeForeign {
		s := a.IBAN
		if a.BankName != "" {
			s = a.BankName + " — " + s
		}
		return s
	}
	s := a.AccountNumber
	if a.BankName != "" {
		s = a.BankName + " — " + s
	}
	return s
}

type CorrespondentBank struct {
	ID            uuid.UUID
	BankAccountID uuid.UUID
	BankName      string
	SWIFT         string
	BankAddress   string
	CreatedAt     time.Time
}

type Repo struct{ db *sql.DB }

func NewRepo(db *sql.DB) *Repo { return &Repo{db: db} }

func (r *Repo) Create(ctx context.Context, a BankAccount) (BankAccount, error) {
	a.ID = uuid.New()
	a.CreatedAt = time.Now()
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO bank_accounts (id, entrepreneur_id, account_type, bank_name, account_number, iban, swift, created_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		a.ID, a.EntrepreneurID, string(a.AccountType), a.BankName, a.AccountNumber, a.IBAN, a.SWIFT, a.CreatedAt,
	)
	return a, err
}

func (r *Repo) Update(ctx context.Context, a BankAccount) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE bank_accounts SET account_type=$1, bank_name=$2, account_number=$3, iban=$4, swift=$5 WHERE id=$6`,
		string(a.AccountType), a.BankName, a.AccountNumber, a.IBAN, a.SWIFT, a.ID,
	)
	return err
}

func (r *Repo) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM bank_accounts WHERE id=$1`, id)
	return err
}

func (r *Repo) FindByID(ctx context.Context, id uuid.UUID) (BankAccount, error) {
	var a BankAccount
	var at string
	err := r.db.QueryRowContext(ctx,
		`SELECT id, entrepreneur_id, account_type, bank_name, account_number, iban, swift, created_at
		 FROM bank_accounts WHERE id=$1`, id,
	).Scan(&a.ID, &a.EntrepreneurID, &at, &a.BankName, &a.AccountNumber, &a.IBAN, &a.SWIFT, &a.CreatedAt)
	if err == sql.ErrNoRows {
		return a, ErrNotFound
	}
	a.AccountType = Type(at)
	return a, err
}

func (r *Repo) ListByEntrepreneur(ctx context.Context, entrepreneurID uuid.UUID) ([]BankAccount, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, entrepreneur_id, account_type, bank_name, account_number, iban, swift, created_at
		 FROM bank_accounts WHERE entrepreneur_id=$1 ORDER BY created_at`, entrepreneurID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var accounts []BankAccount
	for rows.Next() {
		var a BankAccount
		var at string
		if err := rows.Scan(&a.ID, &a.EntrepreneurID, &at, &a.BankName, &a.AccountNumber, &a.IBAN, &a.SWIFT, &a.CreatedAt); err != nil {
			return nil, err
		}
		a.AccountType = Type(at)
		accounts = append(accounts, a)
	}
	return accounts, rows.Err()
}

func (r *Repo) CreateCorrespondent(ctx context.Context, cb CorrespondentBank) (CorrespondentBank, error) {
	cb.ID = uuid.New()
	cb.CreatedAt = time.Now()
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO correspondent_banks (id, bank_account_id, bank_name, swift, bank_address, created_at)
		 VALUES ($1,$2,$3,$4,$5,$6)`,
		cb.ID, cb.BankAccountID, cb.BankName, cb.SWIFT, cb.BankAddress, cb.CreatedAt,
	)
	return cb, err
}

func (r *Repo) UpdateCorrespondent(ctx context.Context, cb CorrespondentBank) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE correspondent_banks SET bank_name=$1, swift=$2, bank_address=$3 WHERE id=$4`,
		cb.BankName, cb.SWIFT, cb.BankAddress, cb.ID,
	)
	return err
}

func (r *Repo) DeleteCorrespondent(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM correspondent_banks WHERE id=$1`, id)
	return err
}

func (r *Repo) FindCorrespondentByID(ctx context.Context, id uuid.UUID) (CorrespondentBank, error) {
	var cb CorrespondentBank
	err := r.db.QueryRowContext(ctx,
		`SELECT id, bank_account_id, bank_name, swift, bank_address, created_at FROM correspondent_banks WHERE id=$1`, id,
	).Scan(&cb.ID, &cb.BankAccountID, &cb.BankName, &cb.SWIFT, &cb.BankAddress, &cb.CreatedAt)
	if err == sql.ErrNoRows {
		return cb, ErrNotFound
	}
	return cb, err
}

func (r *Repo) ListCorrespondentsByAccount(ctx context.Context, bankAccountID uuid.UUID) ([]CorrespondentBank, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, bank_account_id, bank_name, swift, bank_address, created_at
		 FROM correspondent_banks WHERE bank_account_id=$1 ORDER BY created_at`, bankAccountID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cbs []CorrespondentBank
	for rows.Next() {
		var cb CorrespondentBank
		if err := rows.Scan(&cb.ID, &cb.BankAccountID, &cb.BankName, &cb.SWIFT, &cb.BankAddress, &cb.CreatedAt); err != nil {
			return nil, err
		}
		cbs = append(cbs, cb)
	}
	return cbs, rows.Err()
}
