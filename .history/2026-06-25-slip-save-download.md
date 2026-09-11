# Slip Save and Two-Step Download

**Date:** 2026-06-25

## Problem

The slip detail page had a single "Преузми PDF" button that POSTed directly to `/download` and streamed a PDF. There was no way to save edits to a slip without downloading, and the download flow didn't persist changes.

## Solution

Added `Repo.Update` to `internal/sliprecord/sliprecord.go` — updates the 8 editable fields (Payer, Purpose, Payee, PayeeAccount, Reference, PaymentCode, Amount, Currency) by ID.

Added three new HTTP endpoints in `internal/web/handler.go`:
- `POST /slips/{id}/save` — saves form fields, redirects to `GET /slips/{id}?saved=1`
- `POST /slips/{id}/download` — saves form fields, redirects to `GET /slips/{id}?saved=1&download=1`
- `GET /slips/{id}/pdf` — generates PDF from current SlipRecord and streams as attachment

Extended `handleSlip` to read `?saved` and `?download` query params, passing `Saved bool` and `Downloading bool` to template.

Updated `slip.html`:
- Removed now-unused hidden IPS fields (K, V, C, O, M, JS, RL, RP, entrepreneur_id)
- Form `action` removed (each button overrides via `formaction`)
- Two buttons: "Сачувај" (`formaction=/slips/{id}/save`) and "Преузми PDF" (`formaction=/slips/{id}/download`)
- Success `<article>` banner shown when `Saved` is true; text includes "Преузимање почиње..." and a `<script>` that sets `window.location` to `/slips/{id}/pdf` when `Downloading` is true

## Choices

- The intermediate redirect approach (save → redirect → JS download) avoids double-submit issues and gives the user visual feedback before the browser opens a download dialog.
- Used `formaction` on buttons rather than two separate forms to keep the form markup simple and avoid duplicating all input fields.
- `handleSlipPDF` reconstructs `ips.Payment` with defaults K=CodePR, V="01", C="1", O="" matching `handleSlipNewSubmit`.
