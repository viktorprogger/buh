# 2026-07-03 HTMX Pagination & Sorting for All Lists

## Problem
All list views load the full data set with no sorting or pagination. As data grows, this degrades UX.

## Scope — lists to update (slips list excluded per user)
1. `/a/` — accountant entrepreneur list (`index.html`)
2. `/e/clients` — client list (`entrepreneur_clients.html`)
3. `/e/invoices` — invoice list (`entrepreneur_invoices.html`)
4. `/e/bank-accounts` — bank account list (`entrepreneur_bank_accounts.html`)

## Implemented

All four lists updated and building clean.

## Approach
- HTMX 2.0.4 embedded in `/static/htmx.min.js`; script added to both base templates
- Each list template defines a named sub-template (`"xxx-list"`) wrapping the table+pagination div with a stable `id`
- Handlers check `HX-Request: true` header → `RenderNamedTemplate` returns just the sub-template; otherwise full page
- Sort: in-memory (sort.Slice after fetching all); page: in-memory (`PageSlice`)
- `shared/pagination.go` — `ListState`, `ParseListParams`, `PageSlice`, `LessStr`/`LessFloat`
- `shared/render.go` — added `RenderNamedTemplate`
- Sort params: `?sort=<col>&dir=asc|desc&page=<n>`; clicking active column toggles direction; new column resets to page 1
- Page size: 20

## Sortable columns
- Entrepreneurs: `name`, `pib`, `pausal`, `vat`
- Clients: `name`, `pib`, `email`, `type`
- Invoices: `number`, `client`, `type`, `date` (default desc), `amount`
- Bank accounts: `bank`, `type`, `number`
