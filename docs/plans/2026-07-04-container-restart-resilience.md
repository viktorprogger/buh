# Plan: Container Restart Resilience

**Status**: implementing

## Audit findings

After a full audit of the codebase, there is exactly **one** place that does not survive container restart:

### Session encryption keys (HIGH)

`cmd/server.go:80-82` — when `BUH_SESSION_HASH_KEY` / `BUH_SESSION_BLOCK_KEY` env vars are not set, `internal/auth/session.go:34-39` generates random keys. Every restart invalidates all existing session cookies, logging out all users.

Everything else is safe:
- Templates and i18n: rebuilt from embedded assets on startup
- Temporary files: all cleaned up with `defer os.Remove/RemoveAll`
- PDF generation: in-memory buffers (invoice, kpo) or properly cleaned temp files (slip)
- No in-memory caches, rate limiters, or accumulating state found

## Solution

Add a `app_settings` DB table to persist session keys. On startup, keys are loaded from DB (or generated and saved if missing). Env vars remain the highest-priority override.

## Implementation steps

1. **Migration 033** — add `app_settings(key TEXT PK, value TEXT NOT NULL)` table
2. **`loadOrGenerateSessionKeys(db)`** — load keys from DB; if missing, generate via `securecookie.GenerateRandomKey`, persist, return
3. **Wire into `runServer`** — replace the current env-var-only path with: env vars → DB → generate+persist

## Files changed

- `cmd/server.go` — add migration, add helper function, update startup logic
- `db/migrations/033_create_app_settings.sql` — reference SQL file
