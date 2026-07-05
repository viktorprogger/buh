# ADR 006 — KPO Book Design

## Context

Paušal entrepreneurs in Serbia must maintain a KPO ("Knjiga prihoda i ostalih prihoda") ledger per calendar year. Accountants managing 10+ years of clients need quick access to any year, keyboard-first data entry, and a finalization step that marks a year as submitted to the tax authority.

## Options considered

### Option A — Flat table with year column (rejected)
Store all entries in one `kpo_entries` table with a `year` column. Simpler schema, but finalization state lives on the entry level or requires a separate flags table, which complicates the finalize/unfinalize logic.

### Option B — Separate `kpo_books` + `kpo_entries` (chosen)
`kpo_books` holds one record per entrepreneur+year and carries `finalized_at`. `kpo_entries` references `kpo_books` via FK. Finalization is a single-field update on the book row. The ON DELETE CASCADE on both FKs keeps cleanup simple.

## Decision

Option B. The extra indirection is worth it: the book row is the natural home for year-level metadata (finalization date, potentially future fields like "submitted to NBS"). The unique constraint `(entrepreneur_id, year)` on `kpo_books` enforces at the DB level that only one book per year exists.

## Year navigation

The entrepreneur page uses a `?year=YYYY` query parameter. Viewing any year auto-creates the KPO book via INSERT … ON CONFLICT DO UPDATE (effectively an upsert). This eliminates the need for an explicit "create book" step.

## Form-in-table pattern

The new-entry row is rendered inside `<tbody>` but the `<form>` element lives outside the table. HTML5 `form="id"` attributes on each `<input>` associate them with the outer form without breaking the table's DOM validity.
