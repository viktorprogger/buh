# 2026-06-26 Accountant Create CLI Command

## Problem
No way to create accountants from CLI — only via default seed on server start.

## Approach
Follow hrmatch pattern: cobra command with --email, --restore, --password, --password-length flags.
- Place command in `cmd/accountant_create.go` (same package as other commands, added via init())
- Add `GeneratePassword` to `internal/accountant/password.go`
- Add `UpdatePasswordHash` to accountant.Repo for --restore support
- Reuse existing `runMigrations` (same cmd package) so command is self-contained

## Chosen over
- Separate `internal/cli/` deps layer (hrmatch has it; buh is simpler, doesn't need it)
- Skipping migrations in the command (bad UX if run before server)
