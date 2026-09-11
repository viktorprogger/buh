## Problem
Implement the full product roadmap (plan-overview.md):
1. Docker Compose + PostgreSQL setup
2. Accountant entity with sessions and login page
3. Entrepreneur name/ID extraction from PDFs
4. Entrepreneur entity + DB persistence of slips
5. Pico CSS UI (login, entrepreneur list, slip list, slip page, nav, footer)

## Approach
Orchestrated via sub-agents per task group. Sequential dependency chain:
- Tasks 1 & 3 are independent (run in parallel)
- Task 2 blocked by task 1 (needs DB config)
- Task 4 blocked by tasks 1 + 2 + 3
- Task 5 blocked by task 4

History and ADRs handled by orchestrator; all implementation delegated.

## Progress

### Task 1: Docker Compose + PostgreSQL (done)
- Created `compose.yml` with `db` (postgres:16-alpine) and `app` services
- `db` has healthcheck; `app` depends_on db healthy
- Created `.env.example` (documented placeholders) and `.env` (working local dev values)
- Env vars use `BUH_` prefix — see `docs/adr/001-env-var-prefix.md`
- Session keys: `BUH_SESSION_HASH_KEY` (HMAC), `BUH_SESSION_BLOCK_KEY` (AES)

### Task 3: Entrepreneur extraction (done)
- `ExtractEntrepreneurInfo` in internal/extractor using pdftotext + regex
- Extracts ПОСЛОВНО СЕДИШТЕ (firm name) and ЈИПД from PDF text
- Pre-fills payer field in slip view; graceful degradation if not found

### Task 2: Accountant auth (done)
- `internal/accountant/`, `internal/auth/`, `internal/middleware/`
- gorilla/securecookie sessions, bcrypt passwords, lib/pq
- DB migration 001_create_accountants.sql
- Default seed: admin@localhost / pausal with WARNING log
- ADR 002 documents the auth approach

### Task 4: Entrepreneur entity + DB persistence (done)
- `internal/entrepreneur/`, `internal/sliprecord/`
- Migrations 002, 003; FindOrCreate with ON CONFLICT; SlipRecord saved on download
- entrepreneur_id threaded through form hidden field
- ADR 003 documents sliprecord package separation from internal/slip

### Task 5: Pico CSS UI (done)
- Embedded templates via embed.FS; all pages: login, index, entrepreneur, results, slip, placeholder
- Nav (Предузетници / Одјава), footer (Политика приватности / Услови коришћења)
- ADR 004 documents embedded template approach

### Fixes applied by orchestrator after browser verification
- Templates used `{{define "base"}}` so `tmpl.Execute()` returned empty body; fixed with `tmpl.ExecuteTemplate("base", data)`
- Index page was missing upload form (chicken-and-egg with entrepreneur creation); added upload form directly on index
- Dockerfile: golang:1.22 → golang:1.25-bookworm (go.mod requires 1.25); added `server` subcommand to ENTRYPOINT; proper go.sum COPY
- compose.yml port conflicts resolved: app 8081:8081, db 5433:5432

## Status
Complete — all 5 roadmap tasks implemented and verified in browser.
