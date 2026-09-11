# PAUS PDF amount extraction

## Problem
Tax Authority (Poreska uprava) QR codes in PAUS decision PDFs always carry `I:RSD0,00` — the amount field is intentionally blank (a template). The importer was saving slips with amount `0,00`.

## Finding
Monthly amounts are clearly stated in the decision body text:
- RESPOR: one amount ("износи 10.512,42 динара"), one bank account (`840-711122843-32`)
- RESDOP: three amounts with three distinct bank accounts (PIO, ZDRO, NZS)

Each amount can be unambiguously matched to its slip via the payee bank account, which appears in both the text and the QR code's `R` field.

## Solution
Added `extractor.ExtractAmountsByAccount(pdfPath)` that:
1. Runs `pdftotext`
2. Finds all `\d{3}-\d{9}-\d{2}` bank account patterns
3. For each, looks backward 1500 bytes for the most recent `износи X,XX динара` pattern
4. Returns a map of 18-digit normalized account → IPS amount string (e.g. `"RSD10512,42"`)

Added `normalizeAccount("840-711122843-32")` → `"840000071112284332"` (insert 4 zeros after 3-digit bank code).

`importer.processFile` now calls `ExtractAmountsByAccount` and overrides `pay.I` for any payment with `I == "" || I == "RSD0,00"`.

## Key pitfall
The lookback window must be 1500 bytes, not 400. Cyrillic UTF-8 text uses 2 bytes per character, so a ~700-character gap (body text between amount mention and account mention) expands to ~1400 bytes. A 400-byte window missed the amount entirely.

## Why last match in window
RESDOP contains both the annual total (`302.757,60`) and the monthly amount (`25.229,80`) before the PIO account. Taking the last `износи` match in the lookback window always gives the monthly amount (closest to the account), not the annual total.
