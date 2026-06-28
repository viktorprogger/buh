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
	handler := web.NewHandler(accountantRepo, sessions, db)

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
