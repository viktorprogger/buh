# 2026-06-29 Entrepreneur detail card + paušal limit

## Problem

The entrepreneur detail page had no way to edit the entrepreneur's fields,
and there was no visual indication of the paušal turnover limit for the current year.

The list on the index page only showed Name; there was no short display label.

## Approach

1. DB migration 008: add `title TEXT NOT NULL DEFAULT ''` to entrepreneurs table.
   Title = short display label chosen by accountant (e.g. "Milan The Plumber");
   Name = full legal name from tax docs. Index page shows Title with fallback to Name.

2. Updated entrepreneur.go: added `Title` field, updated all SELECT/RETURNING/Scan
   calls, added `Update(ctx, Entrepreneur) error` method.

3. Added `SumForYear(ctx, entrepreneurID, year) (float64, bool, error)` to kpo.Repo:
   joins kpo_books + kpo_entries to get the total for a year without creating a book.

4. Paušal limit is a static function `pausalalLimitForYear(year) int64`:
   - 2027+: 8 000 000 RSD
   - otherwise: 6 000 000 RSD

5. Added `POST /entrepreneurs/{id}` handler (`handleEntrepreneurUpdate`).

6. entrepreneur.html: editable card (Title, Name, PIB + Save) + paušal limit badge in header.
   index.html: show Title (fallback Name) in the list.

## Solution implemented

All of the above. Number formatting with space thousand separators done in handler
(formatIntWithSpaces helper) to avoid template functions.
