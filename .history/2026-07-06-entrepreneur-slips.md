# Entrepreneur Slip Management

**Date:** 2026-07-06  
**Status:** Implemented

---

## Problem Statement

Entrepreneurs who self-register or get invited by an accountant currently have no way to manage their own payment slips (уплатнице). The entire slip workflow lives in the accountant area and is inaccessible from `/e/*` routes.

Entrepreneurs need full CRUD over their own slips — including PDF upload — independently of whether they have a paired accountant. The key constraint: an entrepreneur may only touch slips that belong to them.

When an entrepreneur and accountant are paired and both have slips for the same year, those sets must eventually be reconciled into one canonical set. This is the "slip merge" flow, modeled after the existing KPO merge feature.

---

## Current State

### `slip_records` schema

```
id              UUID    PK
entrepreneur_id UUID    NOT NULL REFERENCES managed_entrepreneurs(id) ON DELETE CASCADE
payment_code    TEXT    NOT NULL DEFAULT ''
amount          TEXT    NOT NULL DEFAULT ''
currency        TEXT    NOT NULL DEFAULT ''
purpose         TEXT    NOT NULL DEFAULT ''
payee_account   TEXT    NOT NULL DEFAULT ''
reference       TEXT    NOT NULL DEFAULT ''
payee           TEXT    NOT NULL DEFAULT ''
payer           TEXT    NOT NULL DEFAULT ''
year            INT     NOT NULL DEFAULT 0
advance         BOOLEAN NOT NULL DEFAULT FALSE
generated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
```

No UNIQUE constraint in the DB. The logical upsert key `(entrepreneur_id, purpose, advance)` is enforced in code only (`FindOrUpdateByPurpose`).

### What's missing

- No `entrepreneur_user_id` FK — standalone entrepreneurs have nowhere to attach slips
- No merge tracking table for slip reconciliation
- No `/e/slips` routes or handlers
- No unpair feature exists anywhere in the codebase (so copy-on-unpair is also new)

---

## Agreed Design Decisions

### 1. XOR ownership on `slip_records`

Add `entrepreneur_user_id UUID REFERENCES entrepreneur_users(id)` to `slip_records`.  
Add an XOR constraint: exactly one of `managed_entrepreneur_id` / `entrepreneur_user_id` must be non-NULL.  
Rename existing `entrepreneur_id` column to `managed_entrepreneur_id` for clarity.

This mirrors the pattern used by `kpo_books`.

### 2. Upsert key change

The current code key `(entrepreneur_id, purpose, advance)` becomes `(owner_id, purpose, year, advance)` where `owner_id` is whichever FK is set. This change applies to **both** the entrepreneur import pipeline (new) and the existing accountant import pipeline.

Rationale: the same purpose text can repeat across different tax years; `year` is required to distinguish them correctly.

### 3. Merge mechanics

**Trigger:** merge runs per year, on demand (accountant initiates from the entrepreneur detail page), similar to KPO merge.

**Conflict detection:** a conflict exists when the same `(purpose, year, advance)` tuple is present on both sides with at least one differing field value.

**Auto-merge:** if a year has zero conflicts, the merge completes immediately without accountant intervention:
- Slips only on the accountant side → stay as-is
- Slips only on the entrepreneur side → moved under `managed_entrepreneur_id`
- Same key on both sides, identical fields → one copy kept under `managed_entrepreneur_id`, duplicate deleted

**Manual merge:** if conflicts exist, the accountant sees a side-by-side comparison per conflicting slip and picks which version wins (or edits inline). After all conflicts are resolved, the accountant confirms → year is marked merged.

**Post-merge state:** all slips for the merged year live under `managed_entrepreneur_id`. The entrepreneur accesses them via the join `managed_entrepreneurs WHERE entrepreneur_user_id = session.UserID`. Both parties see the same set and can edit it.

**Merge tracking table:**

```sql
CREATE TABLE slip_merges (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    managed_entrepreneur_id UUID NOT NULL REFERENCES managed_entrepreneurs(id) ON DELETE CASCADE,
    year                    INT  NOT NULL,
    merged_at               TIMESTAMPTZ NOT NULL DEFAULT now(),
    merged_by               UUID NOT NULL,  -- accountant_id
    UNIQUE (managed_entrepreneur_id, year)
);
```

A row here signals that a given year is fully merged. It does not prevent new slips from being added after the fact (re-merge is always possible).

### 4. Unpair + copy

Unpair is a new feature (does not currently exist). When an accountant unlinks an entrepreneur:
1. All `slip_records` with `managed_entrepreneur_id = <this managed entrepreneur>` are copied into new rows with `entrepreneur_user_id = managed_entrepreneur.entrepreneur_user_id`
2. The original `managed_entrepreneur_id` rows remain (accountant keeps their records)
3. `slip_merges` rows for this pairing are deleted (the merged state is no longer meaningful once the link is broken)
4. `managed_entrepreneurs.entrepreneur_user_id` is set to NULL; `paired_at` is cleared

The same copy logic applies to `kpo_books` when they exist (out of scope for this task — noted here for future reference).

### 5. PDF upload for entrepreneurs

- Entrepreneurs can upload PDFs via the same extractor pipeline used by accountants
- **PIB gate:** if the entrepreneur's profile has no PIB set, the upload form is disabled with a prompt to set their PIB first
- **Validation:** after extraction, the PIB extracted from the PDF is compared to `entrepreneur_users` profile PIB. Mismatch → reject with a clear error ("This PDF belongs to a different entrepreneur")
- On match → standard import pipeline with `entrepreneur_user_id` as the owner

### 6. Advance payment detection

Advance detection (multi-year PDF logic in `importer.go`) is preserved for the entrepreneur upload flow.

---

## Scope of Work

### Phase 1 — Database migration

**Migration N:** rename + XOR

```sql
ALTER TABLE slip_records RENAME COLUMN entrepreneur_id TO managed_entrepreneur_id;
ALTER TABLE slip_records ADD COLUMN entrepreneur_user_id UUID REFERENCES entrepreneur_users(id) ON DELETE CASCADE;
ALTER TABLE slip_records ADD CONSTRAINT slip_records_owner_xor CHECK (
    (managed_entrepreneur_id IS NOT NULL AND entrepreneur_user_id IS NULL) OR
    (managed_entrepreneur_id IS NULL AND entrepreneur_user_id IS NOT NULL)
);
```

**Migration N+1:** merge tracking table

```sql
CREATE TABLE slip_merges (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    managed_entrepreneur_id UUID NOT NULL REFERENCES managed_entrepreneurs(id) ON DELETE CASCADE,
    year                    INT  NOT NULL,
    merged_at               TIMESTAMPTZ NOT NULL DEFAULT now(),
    merged_by               UUID NOT NULL,
    UNIQUE (managed_entrepreneur_id, year)
);
```

Both migrations go in `cmd/server.go` (runMigrations) **and** `db/migrations/`.

### Phase 2 — Repository layer

**`internal/sliprecord/sliprecord.go`**

- Rename all references to `entrepreneur_id` → `managed_entrepreneur_id` in struct tags, SQL, and scan targets
- Add `EntrepreneurUserID uuid.UUID` field to `SlipRecord` struct (nullable, use `*uuid.UUID`)
- `ListByEntrepreneurUser(ctx, entrepreneurUserID uuid.UUID) ([]SlipRecord, error)` — queries `WHERE entrepreneur_user_id = $1`
- `ListByManagedEntrepreneur(ctx, managedEntrepreneurID uuid.UUID) ([]SlipRecord, error)` — rename from current `ListByEntrepreneur`
- Update `FindOrUpdateByPurpose` lookup to `WHERE owner_col = $1 AND purpose = $2 AND year = $3 AND advance = $4` — takes the owner value + which column to use
- `SaveForEntrepreneurUser(ctx, s SlipRecord) (SlipRecord, error)` — insert with `entrepreneur_user_id`
- `CopyToEntrepreneurUser(ctx, managedEntrepreneurID, entrepreneurUserID uuid.UUID) error` — bulk copy for unpair
- `MoveToManagedEntrepreneur(ctx, slipID, managedEntrepreneurID uuid.UUID) error` — used during merge to move entrepreneur-side slips

**New `internal/slipmerge/slipmerge.go`**

```go
type Repo struct { db *sql.DB }

func (r *Repo) IsMerged(ctx, managedEntrepreneurID uuid.UUID, year int) (bool, error)
func (r *Repo) MarkMerged(ctx, managedEntrepreneurID uuid.UUID, year int, mergedBy uuid.UUID) error
func (r *Repo) DeleteForPairing(ctx, managedEntrepreneurID uuid.UUID) error  // used on unpair
```

### Phase 3 — Entrepreneur CRUD routes

New file: `internal/web/entrepreneur/slip.go`

Routes (added to `internal/web/entrepreneur/handler.go`):

```
GET  /e/slips                  list all slips (grouped by year)
GET  /e/slips/new              manual creation form
POST /e/slips/new              create slip
GET  /e/slips/{id}             slip detail + history
POST /e/slips/{id}/save        update slip fields
GET  /e/slips/{id}/pdf         download PDF
POST /e/slips/{id}/download    save + download PDF
POST /e/slips/{id}/delete      delete slip
GET  /e/slips/upload           upload PDF form
POST /e/slips/upload           process uploaded PDF
```

**Access control pattern:**

Every handler calls a helper analogous to accountant's `findOwnedSlip`:

```go
func (h *Handler) findOwnedSlip(w http.ResponseWriter, r *http.Request, slipID uuid.UUID) (sliprecord.SlipRecord, bool) {
    userID, ok := h.entrepreneurUserFromSession(r)
    // ...
    slip, err := h.slips.FindByID(ctx, slipID)
    // check slip.EntrepreneurUserID == userID
    // OR: slip.ManagedEntrepreneurID matches a managed_entrepreneur where entrepreneur_user_id == userID
}
```

The second branch of the access check covers post-merge slips that live under `managed_entrepreneur_id` but still belong to this entrepreneur.

**List page:** slips grouped by year (same `GroupSlipsByYear` helper). For merged years, a "Merged with accountant" badge is shown. Post-merge slips remain fully editable by the entrepreneur.

**PDF upload:** upload form is gated by PIB presence. On submit: extract → validate PIB match → import via `importer.ProcessFileData` adapted to accept `entrepreneurUserID` instead of `accountantID + managed entrepreneur`.

### Phase 4 — Importer adaptation

`internal/importer/importer.go`:

- `ProcessFileData` currently takes `(ctx, accountantID uuid.UUID, filename string, data []byte)`
- Introduce an owner type (or separate function) that accepts either `managedEntrepreneurID` or `entrepreneurUserID` and sets the right FK when saving slips
- Update `FindOrUpdateByPurpose` call site to pass `year` as part of the key

`internal/sliprecord/sliprecord.go` — `FindOrUpdateByPurpose` signature change propagates to:
- `internal/importer/importer.go`
- `internal/web/accountant/slip.go` (handleSlipNewSubmit currently calls it indirectly)

### Phase 5 — Merge UI (accountant side)

New routes in `internal/web/accountant/`:

```
GET  /a/entrepreneurs/{id}/slips/merge/{year}       conflict review page
POST /a/entrepreneurs/{id}/slips/merge/{year}        resolve one conflict (pick side)
POST /a/entrepreneurs/{id}/slips/merge/{year}/done   confirm + finalize merge
```

**Merge page layout:** for each conflicting `(purpose, year, advance)` tuple, show accountant's version and entrepreneur's version side-by-side with a radio "keep this one" per slip. Non-conflicting slips are listed separately as "will be merged automatically."

**Auto-merge trigger:** when accountant loads the merge page and there are zero conflicts, the merge runs immediately and the page shows a success confirmation.

**Entry point:** on the accountant entrepreneur detail page, per-year slip summary shows a "Merge" button when the entrepreneur is paired and has unmerged slips for that year.

### Phase 6 — Unpair feature

New route in `internal/web/accountant/`:

```
POST /a/entrepreneurs/{id}/unpair
```

**Flow:**
1. Verify accountant owns this `managed_entrepreneur`
2. Verify a pairing exists (`entrepreneur_user_id IS NOT NULL`)
3. Copy all `slip_records` WHERE `managed_entrepreneur_id = id` to new rows with `entrepreneur_user_id` (bulk INSERT SELECT)
4. Delete `slip_merges` rows for this pairing
5. Set `managed_entrepreneurs.entrepreneur_user_id = NULL`, `paired_at = NULL`
6. Redirect back to entrepreneur detail page with a confirmation banner

**UI:** "Unlink entrepreneur account" button on the accountant's entrepreneur detail page, behind a confirmation dialog. Clearly states: "The entrepreneur will keep a copy of all slips. Your records are unchanged."

---

## Open Questions

### What happens to merged years on unpair?

When unpairing, slips are copied from `managed_entrepreneur_id` to `entrepreneur_user_id`. The `slip_merges` row is deleted. Effectively the year goes back to "unmerged" on the entrepreneur side with the full copied set.

This is already covered in the design above but worth calling out explicitly: **the entrepreneur gets all slips, not just "their original" ones from before the merge**.

### Slip history attribution for merged slips

**Resolved: log a `"merged"` history event for every slip that moves during merge.**

`slip_history` uses `actor_type` + `actor_id`. When a slip moves ownership during merge (`MoveToManagedEntrepreneur`), the merge handler logs a history entry:

- `event = "merged"`
- `actor_type = "accountant"`, `actor_id = accountant_id`
- `changes = {"from": "entrepreneur_user_id:<uuid>", "to": "managed_entrepreneur_id:<uuid>"}`

This applies to:
- Slips moved from the entrepreneur side to `managed_entrepreneur_id` (both auto-merge and conflict resolution)
- Slips on the accountant side that "win" a conflict (log records which entrepreneur version was discarded)

**Required addition to `slipmerge` package:** `LogMerge` is not a separate method — the merge handler calls `sliphistory.Repo.Log(...)` directly after each `MoveToManagedEntrepreneur` call, same pattern as other slip history logging.

---

## Implementation Order

1. Phase 1 (migration) — unblocks everything else
2. Phase 2 (repository) — unblocks phases 3, 4, 5
3. Phase 4 (importer adaptation) — small but needed before testing upload
4. Phase 3 (entrepreneur CRUD, manual form first, then upload)
5. Phase 5 (merge UI)
6. Phase 6 (unpair)

Phases 5 and 6 can be built in parallel once Phase 2 is done.

---

## Implementation Summary

All 6 phases implemented and compiling clean.

### What was done

**Phase 1 — DB migrations** (035, 036)
- `slip_records.entrepreneur_id` → `managed_entrepreneur_id`; added `entrepreneur_user_id` FK with XOR CHECK constraint
- New `slip_merges` table tracking per-(managed_entrepreneur, year) merge completions

**Phase 2 — Repository layer** (`internal/sliprecord/sliprecord.go`)
- `SlipRecord.EntrepreneurID` → `ManagedEntrepreneurID` + new `EntrepreneurUserID *uuid.UUID`
- New methods: `ListByEntrepreneurUser`, `ListAccessibleByEntrepreneurUser`, `CopyToEntrepreneurUser`, `MoveToManagedEntrepreneur`
- `FindOrUpdateByPurpose` key now includes `year`
- New `internal/slipmerge/slipmerge.go`: `IsMerged`, `MarkMerged`, `DeleteForPairing`, `ListMergedYears`

**Phase 3 — Compilation fixes**
- `internal/importer/importer.go`: `EntrepreneurID` → `ManagedEntrepreneurID`
- `internal/web/accountant/slip.go`: three field references updated
- `internal/web/accountant/entrepreneur.go`: `ListByEntrepreneur` → `ListByManagedEntrepreneur`

**Phase 4 — Importer adaptation**
- Added `ProcessFileDataForEntrepreneurUser` to importer — validates PIB from PDF, saves with `EntrepreneurUserID`, logs with `actor_type="entrepreneur"`

**Phase 5 — Entrepreneur CRUD routes** (`internal/web/entrepreneur/`)
- `handler.go` rewritten with full slip routes under `/e/slips/*`
- `slip.go` (NEW): list, view, save, download, delete, PDF, new-form, upload-form, upload-submit
- `findOwnedSlip` handles both direct ownership and post-merge access via paired managed entrepreneur
- Upload gated by PIB presence from paired managed_entrepreneur record
- Templates: `entrepreneur_slips.html`, `entrepreneur_slip.html`, `entrepreneur_slip_new.html`, `entrepreneur_slip_upload.html`

**Phase 6 — Merge UI + unpair** (`internal/web/accountant/slip_merge.go`, `handleUnpair`)
- Auto-merge when no conflicts; conflict resolution UI per-slip-pair
- `slip_merge_slips.html` template for side-by-side conflict display
- `handleUnpair`: copies managed slips to entrepreneur user, deletes merge records, clears pairing FK

### Difficulties
- Migration order: initially inserted 035/036 before 034 — caught and fixed before any data was at risk
- `slip_merge.go` imported `slipmerge` package unnecessarily (type accessed only via handler field) — removed
- Unused `year` variable in `handleSlipMergePickSide` (URL value used only for validation, not logic) — changed to `if _, err :=`
