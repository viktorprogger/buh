# 2026-07-01 Security & correctness bug fixes from review.json

## Problem
Six bugs identified in review.json:
1. KPO DeleteEntry/UpdateEntry allow cross-book entry manipulation (no book ownership check)
2. ToRSD silently uses rate=1.0 for unknown/empty currency, storing wildly wrong TotalRSD
3. findOwnedSlip returns 403 Forbidden on transient DB error loading entrepreneur
4. FindOrCreateForAccountant pollutes DB with empty kpo_books rows on every GET (DO UPDATE vs DO NOTHING)
5. Entrepreneur-side client/bank-account handlers return 404 on transient DB errors
6. AddEntry computes position with SELECT COUNT(*)+1 with no row lock, enabling concurrent position collisions

## Approach

1. **Bug 1**: Add `bookID` param to `DeleteEntry` (used in `AND kpo_book_id=$2` clause). In `UpdateEntry`, compare fetched bookID to `e.KPOBookID`; handler sets `e.KPOBookID = book.ID`. Returns ErrNotFound if mismatch.

2. **Bug 2**: Validate currency in `handleEInvoiceCreate` against `invoice.FXRates` before building the invoice; return 400 on unknown currency.

3. **Bug 3**: Separate `if err != nil` branch (→ 500) from `if e.AccountantID != accountantID` branch (→ 403) in `findOwnedSlip`.

4. **Bug 4a**: Change `DO UPDATE SET year = EXCLUDED.year` to `DO NOTHING` in both `FindOrCreateForAccountant` and `FindOrCreateForEntrepreneur`. **Bug 4b**: Replace `FindOrCreateForAccountant` with `FindByYear` in `handleEntrepreneur` GET handler; handle ErrNotFound as empty book (no entries yet).

5. **Bug 5**: In `handleEClientDelete`, `handleEBankAccountEditForm`, `handleEBankAccountUpdate`, `handleEBankAccountDelete`: separate `err != nil && !errors.Is(err, ErrNotFound)` (→ 500) from not-found/ownership check (→ 404).

6. **Bug 6**: Add `SELECT id FROM kpo_books WHERE id=$1 FOR UPDATE` at the start of `AddEntry` transaction to serialize concurrent inserts for the same book.

## Solution implemented

All fixes applied to `internal/kpo/kpo.go` and `internal/web/handler.go`. Build passes.
