## Problem
Extract entrepreneur name and ID from Serbian APR/ePorezi паушал PDF files and pre-fill the payer (P) field on the payment slip.

## Approach
Run `pdftotext` on the uploaded PDF (poppler-utils already available), then use regex to locate:
- `ЈИПД:\n<number>` — the entrepreneur's unique tax ID (value on next line due to PDF text flow)
- `ПОСЛОВНО СЕДИШТЕ: <name>` — the firm's Latin-script business name (value on same line)

The firm name (ПОСЛОВНО СЕДИШТЕ) is used for pre-filling rather than the personal name (ОБВЕЗНИКУ) because the firm name is what appears on official payment slips.

## Solution
- Added `ExtractEntrepreneurInfo(pdfPath string) (EntrepreneurInfo, error)` to `internal/extractor/extractor.go`
- `EntrepreneurInfo` holds `Name` (firm name, Latin) and `JIPD` (numeric ID string)
- Errors from pdftotext propagate; missing fields result in empty strings (graceful degradation)
- `handleProcess` in `internal/web/handler.go` calls extraction per uploaded file, then sets `pay.P` if it was empty and a name was found

## Findings
- Both sample PDFs have ЈИПД on a separate line from its value (hence `\n?\s*` in regex)
- ПОСЛОВНО СЕДИШТЕ value is on the same line as the label
- The JIPD in PAUS-RESDOP and PAUS-RESPOR differ (one per tax type) but the firm name is the same
