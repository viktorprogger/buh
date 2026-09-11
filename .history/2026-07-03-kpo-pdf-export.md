# KPO PDF Export

**Date**: 2026-07-03
**Status**: complete

## Problem
Users need to download the KPO book as a printable PDF document following the official Serbian KPO form layout.

## Solution implemented
- Created `internal/kpo/pdf.go` with `EntrepreneurInfo` struct and `GeneratePDF`
- Added `handleKPOPDF` handlers in both `entrepreneur/kpo.go` and `accountant/kpo.go`
- Registered routes in both handler `Routes()` methods
- Added download buttons to `entrepreneur_kpo.html` and `entrepreneur.html`

## 2026-07-03 Update: align with official KPO form (kpo-requirements-pausalac.md)
- **PDF title** changed to "KNJIGA O OSTVARENOM PROMETU PAUSALNO OPOREZOVANIH OBVEZNIKA (KPO)"
- **5-column layout** per official form (was 6): serial, date+description, product revenue, service revenue, total
- **Header fields** now include: PIB, Obveznik/Firma-radnje, MB, Sediste, Sifra poreskog obveznika, Sifra delatnosti
- **New DB fields**: `taxpayer_code` + `activity_code` on `managed_entrepreneurs` (migration 030); `description` on `kpo_entries` (migration 031)
- **EntrepreneurInfo**: added `TaxpayerCode`, `ActivityCode`
- **kpo.Entry**: added `Description` (client name / service type per column 2 of official form)
- **UI**: added Опис column to KPO tables in both accountant and entrepreneur views; added Шифра пореског обвезника + Шифра делатности to entrepreneur card, new form, and profile editing
- **Date label** changed from "Датум наплате" to "Датум" (entries are by invoice issue date, not payment date)
