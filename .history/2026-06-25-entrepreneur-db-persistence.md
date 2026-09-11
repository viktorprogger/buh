## Problem
Implement Entrepreneur entity and DB persistence of payment slips:
- Entrepreneur struct with FindOrCreate, ListByAccountant, FindByID
- SlipRecord struct with Save, ListByEntrepreneur, FindByID
- DB migrations 002 and 003
- Wire into web handler: upsert entrepreneur on /process, save slip on /download
- Pass entrepreneur_id through hidden form field
- Update migration runner to handle all migrations
- Add github.com/google/uuid dependency

## Approach
- New package `internal/entrepreneur/` for entrepreneur entity + repo
- New package `internal/sliprecord/` for slip DB record (avoids conflict with `internal/slip` PDF generator)
- Inline migration runner in cmd/server.go extended with new migrations
- Handler receives *sql.DB directly (simpler than passing two repos, matches existing pattern for extensibility)
- entrepreneur_id passed as hidden form field from /process results to /download form

## Solution

### Packages created
- `internal/entrepreneur/entrepreneur.go` — Entrepreneur struct, Repo with FindOrCreate/ListByAccountant/FindByID
- `internal/sliprecord/sliprecord.go` — SlipRecord struct, Repo with Save/ListByEntrepreneur/FindByID

### Migrations added (inline in cmd/server.go + files in db/migrations/)
- `002_create_entrepreneurs` — entrepreneurs table (id, accountant_id FK, name, jipd, created_at; UNIQUE on jipd+accountant_id)
- `003_create_slip_records` — slip_records table (id, entrepreneur_id FK, payment_code, amount, currency, purpose, payee_account, reference, payee, generated_at)

### Handler changes
- `NewHandler` now also takes `*sql.DB`
- `handleProcess`: calls `entrepreneur.FindOrCreate`; passes `EntrepreneurID` to template
- `handleDownload`: reads `entrepreneur_id` from form, saves SlipRecord to DB
- resultsTmpl: hidden `entrepreneur_id` input added to each slip form

### cmd/server.go
- Passes `db` to `web.NewHandler`

## ADR
- ADR 003: separate sliprecord package vs reusing slip package name
