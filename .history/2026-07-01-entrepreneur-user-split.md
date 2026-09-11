# 2026-07-01 Entrepreneur/Accountant User Split

## Problem
Implement full accountant/entrepreneur user split. Currently one user type (accountant) owns
all data. Goal: entrepreneurs register independently, own their clients/bank_accounts/invoices/KPO,
and can optionally pair with an accountant.

## Deviation from plan
Original plan Assumptions said invoices stay on managed_entrepreneurs. Overridden: invoices link
directly to entrepreneur_users (as migration 020 already said). This avoids joins for basic ops
and simplifies the entrepreneur-side routes.

## Approach
7 phases from the plan doc at ~/.claude/plans/bank-accounts-and-serialized-pinwheel.md:
1. DB migrations 016-021 (add entrepreneur_users, repoint clients/bank_accounts/invoices FKs)
2. Table rename (entrepreneurs→managed_entrepreneurs) + typed session + new packages
3. Route restructuring (/a/* for accountant, /e/* for entrepreneur)
4. Pairing/invite flow
5. KPO merge UI
6. Full entrepreneur-side routes
7. Playwright tests

Phases 1+2 deployed together to avoid broken window.

## Solution (in progress — paused 2026-07-01 ~04:31)

### Completed
- Parts 0–5 (previous session): typed session, route split, KPO fixes, settings simplification, invite/register flow, all templates for invite/register/dashboard
- Part 6: entrepreneur client CRUD (`handleEClient*`) + bank account/correspondent CRUD (`handleEBankAccount*`, `handleECorrespondent*`); `client_form.html`, `bank_account_form.html`, `correspondent_form.html` refactored to use `ActionURL`/`BackURL`/`CorrespondentBaseURL` (no more `.Entrepreneur.ID` hardcoding)
- Part 7: entrepreneur invoice create/view/PDF handlers (`handleEInvoice*`); `invoice.html` and `invoice_new.html` refactored to use `ActionURL`/`BackURL`/`ClientSearchURL`/`PDFUrl`
- Created templates: `entrepreneur_clients.html`, `entrepreneur_bank_accounts.html`, `entrepreneur_invoices.html`
- Build is clean after each part

### Parts 8–10 (completed in second session)
- Part 8: `handleEKPO` + all KPO entry CRUD handlers for entrepreneur side (`handleEKPOAddEntry`, `handleEKPOUpdateEntry`, `handleEKPODeleteEntry`, `handleEKPOReorderEntries`, `handleEKPOFinalize`, `handleEKPOUnfinalize`). Helper `eKPOBookFromPath`. Template `entrepreneur_kpo.html` (mirrors accountant KPO section with `/e/kpo/{year}/*` URLs).
- Part 9: KPO merge handlers (`handleKPOMergeView`, `handleKPOMergeCopyEntry`, `handleKPOMergeMarkDone`). Template `kpo_merge.html` shows entrepreneur-side source entries with copy buttons and the accountant's destination entries.
- Part 10: Added list routes+handlers for `/e/clients`, `/e/bank-accounts`, `/e/invoices`. Dashboard links corrected to list pages. settings.html stripped of bank-accounts/clients sections (now entrepreneur-owned). All `/entrepreneurs/` absolute links in templates replaced with `/a/entrepreneurs/`. Removed dead "New Invoice" link from accountant KPO header (invoices are entrepreneur-created); added "Преузми КПО" merge link instead.
- Build clean at every step.

### Remaining
- Tests (Playwright, unit)

### Key decisions
- Invoice PDF: served as printable HTML (no wkhtmltopdf dependency) — user can browser-print to PDF
- Client search for invoice form uses `/e/clients/search` endpoint (same JSON API pattern as the old accountant-side)
- `eOwnedBankAccount` helper centralizes auth+ownership check for all correspondent bank handlers
