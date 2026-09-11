# Client Local vs Foreign Fields — Invoice Views

## Problem
Client form, invoice detail, PDF, and live preview all show PIB + MB unconditionally regardless of client origin. Serbian regulatory requirements differ: local clients need PIB + Matični Broj (Serbian Tax ID + Registration Number), while foreign clients need only a generic Tax ID / Reg No. field.

## Approach
No DB migration needed — `is_foreign`, `pib`, `registration_number` columns already exist on `clients` table.

Changes:
1. `client_form.html` — JS-driven conditional fields: local shows PIB + MB with helper text, foreign shows single Tax ID/Reg No. field + address country helper
2. `entrepreneur/client.go` — add `is_foreign` to client search JSON; add backend validation (PIB + MB required for local; Reg No. required for foreign)
3. `invoice/pdf.go` — add `IsForeign` to `ClientInfo`; add `TaxIDLabel` to `pdfLabels`; conditional `clientLines` construction
4. `entrepreneur/invoice.go` — pass `IsForeign` from client to `ClientInfo`
5. `invoice.html` — conditional PIB/MB vs Tax ID labels in detail view
6. `invoice_new.html` — add `taxId` label to `PREVIEW_LABELS`; update client preview section to be `IsForeign`-aware

## Decisions
- Foreign client data goes in `RegistrationNumber`, `PIB` stays empty — keeps semantic clarity
- For local clients: both PIB and MB are required (non-empty); for foreign: only RegistrationNumber required
- PDF label for foreign clients: `labelsSR.TaxIDLabel = "Tax ID / Reg. br.:"`, `labelsEN.TaxIDLabel = "Tax ID / Reg. No.:"`

## Result
All views (form, web detail, PDF) conditionally render identification fields based on `IsForeign`.
