# KPO original amount display

## Problem

KPO web tables currently show converted amounts in RSD, while entries can retain original foreign-currency amounts. The display must make both values understandable to accountants without widening the table or implying that the original amount belongs in the official KPO PDF.

## Decision and approach

Show the converted RSD amount as the primary line and the original foreign-currency amount as a smaller muted line directly beneath it. Add `, RSD` to all three monetary column headers, prevent monetary values from wrapping, and place one information icon beside the original total with a concise explanation that the PDF contains only the RSD amount. Apply the same display contract to entrepreneur, accountant, and KPO merge web tables. Keep persistence, conversion, totals, and PDF export unchanged.

## Implemented

The entrepreneur, accountant, and both KPO merge web tables now keep converted RSD as the primary amount and show the original foreign-currency amount as a muted secondary line with its currency code. Monetary headers explicitly identify RSD, monetary values use no-wrap styling, and one localized tooltip beside the original total states that the KPO PDF contains only the RSD amount. Pending-rate rows preserve source values without presenting an unconfirmed RSD amount. PDF output, persistence, conversion, totals, forms, and currency-picker behavior remain unchanged.

## Validation

- `go test ./...` passed.
- `go vet ./...` passed.
- `go test ./internal/i18n ./internal/web/...` passed during implementation.
- Locale JSON/key-parity validation, Tailwind CSS validation, and `git diff --check` passed.
- Playwright test listing passed and the deterministic fixture uses the current managed-entrepreneur schema without live NBS calls.
- Full browser execution remains unverified because the application server/database configuration was unavailable in the local environment.

## Follow-up

Run the focused KPO Playwright test once the application server is available at the configured base URL.
