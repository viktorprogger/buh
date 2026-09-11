# Import flow: save-on-upload + results summary

## Problem
Current `/process` parses PDFs and shows a preview page; the DB save only happens when the user clicks "Download". This makes it impossible to show a meaningful "what was created/updated" summary, and results in PDFs being saved to DB from the download action rather than the upload action.

## Approach
1. `entrepreneur.FindOrCreate` → returns `(Entrepreneur, bool isNew, error)` 
2. `sliprecord.FindOrUpdateByPurpose` → upserts by `(entrepreneur_id, purpose)` returning `UpsertStatus` (Created/Updated/Unchanged)
3. New `internal/importer` package encapsulates all PDF→DB logic, returns `Result{Entrepreneurs, Errors}`
4. `handleProcess` calls importer, passes `Result` to rewritten `results.html`
5. Remove `handleDownload` (old flow); PDF generation now served from `/slips/{id}/pdf`

## Natural key decision
Uplatnica natural key: `(entrepreneur_id, purpose)` — application-level only (no DB constraint) so manual slips can have duplicate purposes without breaking.

## Removed
- `slipView` struct, `paymentToView`, `splitAmount`, `joinNonEmpty`, `formatAccount` from handler.go
- `POST /download` route and `handleDownload`
- `nl2br` template func and `slip_styles` include from results template
