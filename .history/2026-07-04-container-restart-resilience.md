# Container Restart Resilience

**Date**: 2026-07-04

## Problem

After auditing all in-memory and ephemeral state, one real issue was found:

**Session encryption keys** (gorilla/securecookie HMAC + AES keys) are randomly generated on startup when `BUH_SESSION_HASH_KEY` / `BUH_SESSION_BLOCK_KEY` env vars are not set. This logs out every user on container restart or update.

Everything else (templates, i18n, temp files) is either derived from embedded assets or properly cleaned up.

## Approach

Store session keys in a new `app_settings` DB table (key/value). On startup:
1. If env vars provided → use them (existing explicit-config path, unchanged)
2. Otherwise → try loading from `app_settings` table
3. If not in DB yet → generate random keys, persist to DB, use them

This means: first boot generates keys and saves them. Every subsequent restart reuses the same DB-stored keys. No external secret manager needed.

Rejected alternatives:
- Require env vars (operational burden, breaks dev experience)
- File-based persistence (needs a volume mount, more moving parts than DB)

## Implementation

- Migration `033_create_app_settings`: `CREATE TABLE app_settings (key TEXT PRIMARY KEY, value TEXT NOT NULL)`
- New function `loadOrGenerateSessionKeys(db *sql.DB) (hashKey, blockKey []byte, err error)` in `cmd/server.go`
- Env vars remain highest-priority override (no behavior change when set)

## Result

Session keys now survive container restarts via DB persistence. Users stay logged in across deploys.
