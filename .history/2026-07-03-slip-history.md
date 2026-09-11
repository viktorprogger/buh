# Slip History / Audit Log

## Problem
Need to track who changed a slip, what changed, and when — covering created, updated, and deleted events. Must support accountants now, entrepreneurs soon, and future enterprise (multiple accountants per entrepreneur data).

## Approach
- New `slip_history` table: `slip_id UUID NOT NULL` (no FK — history must survive deletion), `event TEXT`, `actor_type TEXT`, `actor_id UUID`, `changes JSONB`, `changed_at TIMESTAMPTZ`
- No FK on `slip_id` so history rows outlive the slip record
- `actor_type` ('accountant'/'entrepreneur') + `actor_id` UUID: forward-compatible with enterprise (new actor types just add new `actor_type` values)
- `changes` JSONB: snapshot for created/deleted events; field-level diff `{field: {old: X, new: Y}}` for updated events
- History logging at handler level (not repo level) so actor info is available
- Import path (PDF upload) also tracked — accountant who triggered it is the actor
- `FindOrUpdateByPurpose` return signature extended to return `(old, result SlipRecord, status, err)` so importer can compute diffs
- New package `internal/sliphistory/`; `Snapshot`/`Diff` helpers added to `internal/sliprecord`
- Timeline shown on slip detail page (accountant view)

## Decisions
- No FK on slip_id (vs ON DELETE SET NULL): audit log should be a self-contained record; we don't want nulled-out references for deleted slips — the `deleted` event has a full snapshot
- No `actor_name` column: join-on-demand for display; if actor deleted, show type+ID only (acceptable edge case)
- History failures do not abort the main operation (slip save/delete) — consistency via best-effort, not transactions
