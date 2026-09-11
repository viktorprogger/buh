# 2026-07-02 Entrepreneur Create CLI Command

## Problem
No way to create entrepreneur login users from CLI.

## Solution
- Added `UpdatePasswordHash` to `internal/entrepreneuruser/entrepreneuruser.go` (mirrors accountant.Repo)
- Created `cmd/entrepreneur_create.go` with `entre-create` cobra command
- Reuses `accountant.GeneratePassword` and `runMigrations` from same cmd package
- Same flags as `accountant-create`: `--email`, `--restore`, `--password`, `--password-length`
