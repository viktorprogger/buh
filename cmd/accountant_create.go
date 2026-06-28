package cmd

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	_ "github.com/lib/pq"
	"github.com/spf13/cobra"

	"buh/internal/accountant"
	"buh/internal/config"
)

var accountantCreateCmd = &cobra.Command{
	Use:   "accountant-create",
	Short: "Create an accountant and print the password",
	RunE:  runAccountantCreate,
}

func init() {
	accountantCreateCmd.Flags().String("email", "", "accountant email (required)")
	accountantCreateCmd.Flags().Bool("restore", false, "reset password if the accountant already exists")
	accountantCreateCmd.Flags().String("password", "", "password (generated if empty)")
	accountantCreateCmd.Flags().Int("password-length", 16, "generated password length")
	rootCmd.AddCommand(accountantCreateCmd)
}

func runAccountantCreate(cmd *cobra.Command, args []string) error {
	email, _ := cmd.Flags().GetString("email")
	if email == "" {
		return fmt.Errorf("--email is required")
	}
	restore, _ := cmd.Flags().GetBool("restore")
	password, _ := cmd.Flags().GetString("password")
	passwordLen, _ := cmd.Flags().GetInt("password-length")

	if password == "" {
		p, err := accountant.GeneratePassword(passwordLen)
		if err != nil {
			return fmt.Errorf("generate password: %w", err)
		}
		password = p
	}

	cfg := config.Load()
	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer db.Close()
	if err := db.PingContext(context.Background()); err != nil {
		return fmt.Errorf("ping db: %w", err)
	}
	if err := runMigrations(db); err != nil {
		return fmt.Errorf("migrations: %w", err)
	}

	repo := accountant.NewRepo(db)
	ctx := context.Background()

	existing, err := repo.FindByEmail(ctx, email)
	if err != nil && !errors.Is(err, accountant.ErrNotFound) {
		return fmt.Errorf("find accountant: %w", err)
	}

	if existing != nil {
		if !restore {
			return fmt.Errorf("accountant %s already exists; use --restore to reset the password", email)
		}
		if err := repo.UpdatePasswordHash(ctx, existing.ID, password); err != nil {
			return fmt.Errorf("update password: %w", err)
		}
		fmt.Printf("password updated\nemail:    %s\npassword: %s\n", email, password)
		return nil
	}

	if _, err := repo.Create(ctx, email, password); err != nil {
		return fmt.Errorf("create accountant: %w", err)
	}
	fmt.Printf("accountant created\nemail:    %s\npassword: %s\n", email, password)
	return nil
}
