package cmd

import (
	"context"
	"database/sql"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"os"

	_ "github.com/lib/pq"
	"github.com/spf13/cobra"

	"buh/internal/accountant"
	"buh/internal/auth"
	"buh/internal/config"
	"buh/internal/entrepreneuruser"
	"buh/internal/web"
)

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start the web UI server for processing paušal payment slips",
	RunE:  runServer,
}

func init() {
	serverCmd.Flags().StringP("port", "p", "", "Port to listen on (overrides BUH_ADDR)")
	rootCmd.AddCommand(serverCmd)
}

func runServer(cmd *cobra.Command, args []string) error {
	cfg := config.Load()

	// Allow --port flag to override BUH_ADDR for convenience.
	if p, _ := cmd.Flags().GetString("port"); p != "" {
		cfg.Addr = ":" + p
	}

	// Open database connection.
	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer db.Close()
	if err := db.PingContext(context.Background()); err != nil {
		return fmt.Errorf("ping db: %w", err)
	}

	// Run migrations.
	if err := runMigrations(db); err != nil {
		return fmt.Errorf("migrations: %w", err)
	}

	// Set up accountant repository.
	accountantRepo := accountant.NewRepo(db)

	// Seed default accountant if the table is empty.
	if err := seedDefaultAccountant(accountantRepo); err != nil {
		return fmt.Errorf("seed: %w", err)
	}

	// Decode session keys from base64.
	var hashKey, blockKey []byte
	if cfg.SessionHashKey != "" {
		b, err := base64.StdEncoding.DecodeString(cfg.SessionHashKey)
		if err != nil {
			return fmt.Errorf("decode BUH_SESSION_HASH_KEY: %w", err)
		}
		hashKey = b
	}
	if cfg.SessionBlockKey != "" {
		b, err := base64.StdEncoding.DecodeString(cfg.SessionBlockKey)
		if err != nil {
			return fmt.Errorf("decode BUH_SESSION_BLOCK_KEY: %w", err)
		}
		blockKey = b
	}
	if cfg.SessionHashKey == "" || cfg.SessionBlockKey == "" {
		log.Println("WARNING: session keys not configured; sessions will be invalidated on restart")
	}

	sessions := auth.NewSessionManager(hashKey, blockKey)
	entrepreneurUserRepo := entrepreneuruser.NewRepo(db)
	handler := web.NewHandler(accountantRepo, entrepreneurUserRepo, sessions, db)

	log.Printf("Server running at http://localhost%s", cfg.Addr)
	return http.ListenAndServe(cfg.Addr, handler)
}

// runMigrations applies all SQL migration files embedded in db/migrations/.
func runMigrations(db *sql.DB) error {
	migrations := []struct {
		name string
		sql  string
	}{
		{
			name: "001_create_accountants",
			sql: `CREATE EXTENSION IF NOT EXISTS "pgcrypto";
CREATE TABLE IF NOT EXISTS accountants (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    email         TEXT        NOT NULL UNIQUE,
    password_hash TEXT        NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);`,
		},
		{
			name: "002_create_entrepreneurs",
			sql: `CREATE TABLE IF NOT EXISTS entrepreneurs (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    accountant_id UUID        NOT NULL REFERENCES accountants(id) ON DELETE CASCADE,
    name          TEXT        NOT NULL,
    jipd          TEXT        NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (accountant_id, jipd)
);`,
		},
		{
			name: "003_create_slip_records",
			sql: `CREATE TABLE IF NOT EXISTS slip_records (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    entrepreneur_id UUID        NOT NULL REFERENCES entrepreneurs(id) ON DELETE CASCADE,
    payment_code    TEXT        NOT NULL DEFAULT '',
    amount          TEXT        NOT NULL DEFAULT '',
    currency        TEXT        NOT NULL DEFAULT '',
    purpose         TEXT        NOT NULL DEFAULT '',
    payee_account   TEXT        NOT NULL DEFAULT '',
    reference       TEXT        NOT NULL DEFAULT '',
    payee           TEXT        NOT NULL DEFAULT '',
    generated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);`,
		},
		{
			name: "004_add_payer_to_slip_records",
			sql:  `ALTER TABLE slip_records ADD COLUMN IF NOT EXISTS payer TEXT NOT NULL DEFAULT '';`,
		},
		{
			name: "005_rename_jipd_to_pib",
			sql:  `ALTER TABLE entrepreneurs RENAME COLUMN jipd TO pib;`,
		},
		{
			name: "006_create_kpo_books",
			sql: `CREATE TABLE IF NOT EXISTS kpo_books (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    entrepreneur_id UUID        NOT NULL REFERENCES entrepreneurs(id) ON DELETE CASCADE,
    year            INTEGER     NOT NULL,
    finalized_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(entrepreneur_id, year)
);`,
		},
		{
			name: "007_create_kpo_entries",
			sql: `CREATE TABLE IF NOT EXISTS kpo_entries (
    id              UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    kpo_book_id     UUID          NOT NULL REFERENCES kpo_books(id) ON DELETE CASCADE,
    collection_date DATE          NOT NULL,
    invoice_number  TEXT          NOT NULL DEFAULT '',
    product_revenue NUMERIC(15,2) NOT NULL DEFAULT 0,
    service_revenue NUMERIC(15,2) NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ   NOT NULL DEFAULT now()
);`,
		},
		{
			name: "008_add_title_to_entrepreneurs",
			sql:  `ALTER TABLE entrepreneurs ADD COLUMN IF NOT EXISTS title TEXT NOT NULL DEFAULT '';`,
		},
		{
			name: "009_add_position_to_kpo_entries",
			sql: `ALTER TABLE kpo_entries ADD COLUMN IF NOT EXISTS position INTEGER;
UPDATE kpo_entries e
SET position = sub.rn
FROM (
  SELECT id,
         ROW_NUMBER() OVER (PARTITION BY kpo_book_id ORDER BY collection_date, created_at) AS rn
  FROM kpo_entries
) sub
WHERE e.id = sub.id AND e.position IS NULL;
ALTER TABLE kpo_entries ALTER COLUMN position SET NOT NULL;`,
		},
		{
			name: "010_add_year_advance_to_slip_records",
			sql: `ALTER TABLE slip_records ADD COLUMN IF NOT EXISTS year INT NOT NULL DEFAULT 0;
ALTER TABLE slip_records ADD COLUMN IF NOT EXISTS advance BOOLEAN NOT NULL DEFAULT FALSE;`,
		},
		{
			name: "011_add_address_bank_to_entrepreneurs",
			sql: `ALTER TABLE entrepreneurs ADD COLUMN IF NOT EXISTS address TEXT NOT NULL DEFAULT '';
ALTER TABLE entrepreneurs ADD COLUMN IF NOT EXISTS bank_account TEXT NOT NULL DEFAULT '';`,
		},
		{
			name: "012_create_clients",
			sql: `CREATE TABLE IF NOT EXISTS clients (
    id                  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    entrepreneur_id     UUID        NOT NULL REFERENCES entrepreneurs(id) ON DELETE CASCADE,
    name                TEXT        NOT NULL,
    pib                 TEXT        NOT NULL DEFAULT '',
    registration_number TEXT        NOT NULL DEFAULT '',
    email               TEXT        NOT NULL DEFAULT '',
    address             TEXT        NOT NULL DEFAULT '',
    is_foreign          BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS clients_entrepreneur_id_idx ON clients(entrepreneur_id);`,
		},
		{
			name: "013_create_invoices",
			sql: `CREATE TABLE IF NOT EXISTS invoices (
    id              UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    entrepreneur_id UUID          NOT NULL REFERENCES entrepreneurs(id) ON DELETE CASCADE,
    client_id       UUID          REFERENCES clients(id) ON DELETE SET NULL,
    client_name     TEXT          NOT NULL DEFAULT '',
    invoice_type    TEXT          NOT NULL DEFAULT 'standard',
    invoice_number  TEXT          NOT NULL DEFAULT '',
    issue_date      DATE          NOT NULL,
    period_start    DATE,
    period_end      DATE,
    due_date        DATE,
    currency        TEXT          NOT NULL DEFAULT 'RSD',
    notes           TEXT          NOT NULL DEFAULT '',
    total_rsd       NUMERIC(15,2) NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ   NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS invoice_items (
    id           UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    invoice_id   UUID          NOT NULL REFERENCES invoices(id) ON DELETE CASCADE,
    description  TEXT          NOT NULL DEFAULT '',
    quantity     NUMERIC(10,2) NOT NULL DEFAULT 1,
    unit_price   NUMERIC(15,2) NOT NULL DEFAULT 0,
    discount_pct NUMERIC(5,2)  NOT NULL DEFAULT 0,
    is_product   BOOLEAN       NOT NULL DEFAULT FALSE,
    position     INTEGER       NOT NULL DEFAULT 0
);`,
		},
		{
			name: "014_create_bank_accounts",
			sql: `CREATE TABLE IF NOT EXISTS bank_accounts (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    entrepreneur_id UUID        NOT NULL REFERENCES entrepreneurs(id) ON DELETE CASCADE,
    account_type    TEXT        NOT NULL DEFAULT 'local',
    bank_name       TEXT        NOT NULL DEFAULT '',
    account_number  TEXT        NOT NULL DEFAULT '',
    iban            TEXT        NOT NULL DEFAULT '',
    swift           TEXT        NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS bank_accounts_entrepreneur_id_idx ON bank_accounts(entrepreneur_id);
CREATE TABLE IF NOT EXISTS correspondent_banks (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    bank_account_id UUID        NOT NULL REFERENCES bank_accounts(id) ON DELETE CASCADE,
    bank_name       TEXT        NOT NULL DEFAULT '',
    swift           TEXT        NOT NULL DEFAULT '',
    bank_address    TEXT        NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);`,
		},
		{
			name: "015_add_bank_account_to_invoices",
			sql: `ALTER TABLE invoices ADD COLUMN IF NOT EXISTS bank_account_id UUID REFERENCES bank_accounts(id) ON DELETE SET NULL;
ALTER TABLE invoices ADD COLUMN IF NOT EXISTS correspondent_bank_id UUID REFERENCES correspondent_banks(id) ON DELETE SET NULL;`,
		},
		{
			name: "016_create_entrepreneur_users",
			sql: `CREATE TABLE IF NOT EXISTS entrepreneur_users (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    email         TEXT        UNIQUE,
    password_hash TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);`,
		},
		{
			name: "017_add_entrepreneur_user_id_to_managed",
			sql: `ALTER TABLE entrepreneurs
    ADD COLUMN IF NOT EXISTS entrepreneur_user_id UUID
    REFERENCES entrepreneur_users(id) ON DELETE RESTRICT;`,
		},
		{
			name: "018_repoint_clients_to_entrepreneur_users",
			sql: `DELETE FROM clients;
ALTER TABLE clients DROP CONSTRAINT IF EXISTS clients_entrepreneur_id_fkey;
ALTER TABLE clients DROP COLUMN IF EXISTS entrepreneur_id;
ALTER TABLE clients ADD COLUMN IF NOT EXISTS entrepreneur_user_id UUID NOT NULL
    REFERENCES entrepreneur_users(id) ON DELETE CASCADE;`,
		},
		{
			name: "019_repoint_bank_accounts_to_entrepreneur_users",
			sql: `DELETE FROM bank_accounts;
ALTER TABLE bank_accounts DROP CONSTRAINT IF EXISTS bank_accounts_entrepreneur_id_fkey;
ALTER TABLE bank_accounts DROP COLUMN IF EXISTS entrepreneur_id;
ALTER TABLE bank_accounts ADD COLUMN IF NOT EXISTS entrepreneur_user_id UUID NOT NULL
    REFERENCES entrepreneur_users(id) ON DELETE CASCADE;`,
		},
		{
			name: "020_repoint_invoices_to_entrepreneur_users",
			sql: `DELETE FROM invoices;
ALTER TABLE invoices DROP CONSTRAINT IF EXISTS invoices_entrepreneur_id_fkey;
ALTER TABLE invoices DROP COLUMN IF EXISTS entrepreneur_id;
ALTER TABLE invoices ADD COLUMN IF NOT EXISTS entrepreneur_user_id UUID NOT NULL
    REFERENCES entrepreneur_users(id) ON DELETE CASCADE;`,
		},
		{
			name: "021_rebuild_indexes_for_entrepreneur_users",
			sql: `DROP INDEX IF EXISTS clients_entrepreneur_id_idx;
DROP INDEX IF EXISTS bank_accounts_entrepreneur_id_idx;
CREATE INDEX IF NOT EXISTS clients_entrepreneur_user_id_idx ON clients(entrepreneur_user_id);
CREATE INDEX IF NOT EXISTS bank_accounts_entrepreneur_user_id_idx ON bank_accounts(entrepreneur_user_id);
CREATE INDEX IF NOT EXISTS invoices_entrepreneur_user_id_idx ON invoices(entrepreneur_user_id);`,
		},
		{
			name: "022_rename_entrepreneurs_to_managed_entrepreneurs",
			sql:  `ALTER TABLE entrepreneurs RENAME TO managed_entrepreneurs;`,
		},
		{
			name: "023_add_paired_at_to_managed_entrepreneurs",
			sql:  `ALTER TABLE managed_entrepreneurs ADD COLUMN IF NOT EXISTS paired_at TIMESTAMPTZ;`,
		},
		{
			name: "024_create_invitations",
			sql: `CREATE TABLE IF NOT EXISTS invitations (
    id                      UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    token                   TEXT        NOT NULL UNIQUE DEFAULT encode(gen_random_bytes(32), 'hex'),
    inviter_type            TEXT        NOT NULL CHECK (inviter_type IN ('accountant','entrepreneur')),
    inviter_id              UUID        NOT NULL,
    invitee_email           TEXT,
    managed_entrepreneur_id UUID        REFERENCES managed_entrepreneurs(id) ON DELETE CASCADE,
    accepted_at             TIMESTAMPTZ,
    expires_at              TIMESTAMPTZ NOT NULL DEFAULT now() + INTERVAL '7 days',
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS invitations_token_idx ON invitations(token);`,
		},
		{
			name: "025_split_kpo_books_owner",
			sql: `ALTER TABLE kpo_books
    ADD COLUMN IF NOT EXISTS managed_entrepreneur_id UUID REFERENCES managed_entrepreneurs(id) ON DELETE CASCADE,
    ADD COLUMN IF NOT EXISTS entrepreneur_user_id    UUID REFERENCES entrepreneur_users(id)    ON DELETE CASCADE;

UPDATE kpo_books SET managed_entrepreneur_id = entrepreneur_id WHERE managed_entrepreneur_id IS NULL;

ALTER TABLE kpo_books ADD CONSTRAINT kpo_books_owner_xor CHECK (
    (managed_entrepreneur_id IS NOT NULL AND entrepreneur_user_id IS NULL) OR
    (managed_entrepreneur_id IS NULL     AND entrepreneur_user_id IS NOT NULL)
);

ALTER TABLE kpo_books DROP CONSTRAINT IF EXISTS kpo_books_entrepreneur_id_year_key;
ALTER TABLE kpo_books DROP COLUMN IF EXISTS entrepreneur_id;

CREATE UNIQUE INDEX IF NOT EXISTS kpo_books_managed_year ON kpo_books(managed_entrepreneur_id, year)
    WHERE managed_entrepreneur_id IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS kpo_books_user_year ON kpo_books(entrepreneur_user_id, year)
    WHERE entrepreneur_user_id IS NOT NULL;`,
		},
		{
			name: "026_add_merged_at_to_kpo_books",
			sql:  `ALTER TABLE kpo_books ADD COLUMN IF NOT EXISTS merged_at TIMESTAMPTZ;`,
		},
		{
			name: "027_add_language_to_invoices",
			sql:  `ALTER TABLE invoices ADD COLUMN IF NOT EXISTS language TEXT NOT NULL DEFAULT 'sr';`,
		},
		{
			name: "028_add_mb_and_invoice_flags",
			sql: `ALTER TABLE managed_entrepreneurs ADD COLUMN IF NOT EXISTS mb TEXT NOT NULL DEFAULT '';
ALTER TABLE invoices ADD COLUMN IF NOT EXISTS no_vat BOOLEAN NOT NULL DEFAULT TRUE;
ALTER TABLE invoices ADD COLUMN IF NOT EXISTS no_sign BOOLEAN NOT NULL DEFAULT TRUE;`,
		},
	}

	// Create migrations tracking table.
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		name TEXT PRIMARY KEY,
		applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
	)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	for _, m := range migrations {
		var exists bool
		err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE name = $1)`, m.name).Scan(&exists)
		if err != nil {
			return fmt.Errorf("check migration %s: %w", m.name, err)
		}
		if exists {
			continue
		}
		if _, err := db.Exec(m.sql); err != nil {
			return fmt.Errorf("apply migration %s: %w", m.name, err)
		}
		if _, err := db.Exec(`INSERT INTO schema_migrations (name) VALUES ($1)`, m.name); err != nil {
			return fmt.Errorf("record migration %s: %w", m.name, err)
		}
		log.Printf("migration applied: %s", m.name)
	}
	return nil
}

// seedDefaultAccountant creates admin@localhost / pausal if no accountants exist.
func seedDefaultAccountant(repo *accountant.Repo) error {
	n, err := repo.Count(context.Background())
	if err != nil {
		return err
	}
	if n > 0 {
		return nil
	}

	defaultEmail := "admin@localhost"
	defaultPassword := "pausal"

	if v := os.Getenv("BUH_DEFAULT_EMAIL"); v != "" {
		defaultEmail = v
	}
	if v := os.Getenv("BUH_DEFAULT_PASSWORD"); v != "" {
		defaultPassword = v
	}

	if _, err := repo.Create(context.Background(), defaultEmail, defaultPassword); err != nil {
		return fmt.Errorf("create default accountant: %w", err)
	}
	log.Printf("WARNING: no accountants found — created default accountant email=%s password=%s  Change this immediately!", defaultEmail, defaultPassword)
	return nil
}
