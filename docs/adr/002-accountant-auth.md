# ADR 002 — Accountant Auth: gorilla/securecookie + lib/pq

## Context

The original server accepted a plain password via CLI flag and stored it verbatim in a session cookie. This needs replacing with a real auth model: accountants stored in PostgreSQL with bcrypt passwords, sessions managed via encrypted cookies.

## Alternatives Considered

### A. JWT tokens stored client-side
- Pro: stateless, no server-side session store needed
- Con: requires token refresh logic, harder to invalidate immediately, more moving parts for a single-accountant tool

### B. gorilla/sessions (file store or cookie store)
- Pro: higher-level API with flash messages etc.
- Con: heavier abstraction; the hrmatch reference already shows securecookie directly is clean and sufficient

### C. gorilla/securecookie directly (chosen)
- Pro: minimal dependency, same pattern as the hrmatch reference, HMAC-signed + AES-encrypted cookie, no server-side state
- Con: sessions cannot be server-side invalidated (acceptable for this use case; logout clears the cookie client-side)

## Decision

Use `gorilla/securecookie` for session management. Session keys (`BUH_SESSION_HASH_KEY`, `BUH_SESSION_BLOCK_KEY`) are base64-encoded and read from environment. Missing keys fall back to random keys with a warning (sessions survive only per process lifetime — fine for dev).

PostgreSQL driver: `lib/pq` (pure Go, no CGo).

Migrations: inline in Go code (tracked in `schema_migrations` table) instead of a separate migration tool — avoids adding another dependency for a small schema.

Seed: if no accountants exist, create `admin@localhost` / `pausal` with a loud log warning.

## Rationale

Matches the hrmatch reference pattern. Minimal new dependencies. Suitable for a single-accountant internal tool.
