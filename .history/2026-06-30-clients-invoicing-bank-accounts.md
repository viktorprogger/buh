# 2026-06-30 Clients, Invoicing & Bank Accounts

## Problem
Implement a full Clients & Invoicing module, then add structured bank account management.

## Solution

### Phase 1: Clients & Invoicing
- Migrations 011-013: address/bank_account on entrepreneurs, clients table, invoices + invoice_items tables
- `internal/client/`: CRUD repo with ILIKE search
- `internal/invoice/`: CRUD repo, FX rates (hardcoded), PDF generation via gofpdf
- Settings page: entrepreneur profile (address, bank_account) + client CRUD
- Invoice creation form: split-screen with live JS preview, client autocomplete, currency selector, dynamic line items
- KPO integration: standard invoices auto-create KPO entries; advance invoices shown muted in KPO table

### Phase 2: Bank Accounts
- Migrations 014-015: bank_accounts (local/foreign), correspondent_banks, FK columns on invoices
- `internal/bankaccount/`: CRUD for accounts and correspondent banks
- Settings page: bank accounts table with edit/delete, correspondents manageable from account edit page
- Invoice form: bank account select, correspondent select (AJAX/JS from preloaded data), live preview shows account+correspondent
- Invoice detail: shows bank account and correspondent bank if set
- PDF: prints structured bank account info (IBAN/SWIFT for foreign, account number for local, correspondent bank if set)

### KPO test fixes
- `FindByYear` → `FindOrCreate` on page load so `#year-select` always renders for new entrepreneurs
- Fixed hidden year form (tests needed to click "+ Додај годину" before filling)
- Fixed date format: tests used ISO `YYYY-MM-DD` but inputs expect `ДД.ММ`

## Outcome
42 Playwright tests (11 KPO + 17 invoicing + 14 bank accounts), all passing.
