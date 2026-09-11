# Manual Slip Creation Form

**Date:** 2026-06-25

## Problem

Users needed to create payment slips manually (without uploading a PDF) from the entrepreneur page.

## Approach

Add two new routes under `/entrepreneurs/{id}/slips/new` (GET for form, POST for submit), a new `slip_new.html` template, and a "Нова уплатница" button on the entrepreneur detail page.

## Solution Implemented

- `internal/web/templates/slip_new.html` — new form template extending base.html; fields: S, N, R (with dash format hint), RO, SF (default 253), amount+currency (RSD/EUR select), P (pre-filled with entrepreneur name)
- `internal/web/handler.go`:
  - Added `slipNew *template.Template` to `templates` struct
  - Parsed `slip_new.html` in `parseTemplates()`
  - Registered `GET /entrepreneurs/{id}/slips/new` → `handleSlipNewForm`
  - Registered `POST /entrepreneurs/{id}/slips/new` → `handleSlipNewSubmit`
  - `handleSlipNewForm` — loads entrepreneur, renders form with defaults (SF=253, currency=RSD, P=entrepreneur name)
  - `handleSlipNewSubmit` — validates required fields, strips dashes from account number, builds `ips.Payment` with K=PR/V=01/C=1, generates temp PDF via `slip.GeneratePDF`, saves `sliprecord.SlipRecord`, redirects to `/slips/{id}`
- `internal/web/templates/entrepreneur.html` — added "Нова уплатница" button above the slips table

## Notes

- Account number entered as `XXX-XXXXXXXXXXXXX-XX` is normalized by stripping dashes before storing and using in IPS payment
- PDF is generated to validate the payment (temp file, discarded after save); the slip detail page handles re-download separately
- Validation errors re-render the form with the filled values and an error message
