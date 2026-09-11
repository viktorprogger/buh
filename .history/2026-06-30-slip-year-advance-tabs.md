# Slip Year/Advance Fields + Tabbed UI

## Problem
Slip records had no year or advance flag. A PDF can contain two slips (older year = main, newer year = advance for next period). UI listed all slips in a flat table.

## Approach
- DB migration 010: add `year INT NOT NULL DEFAULT 0` and `advance BOOLEAN NOT NULL DEFAULT FALSE`
- In PDF import: extract purpose-year per QR code; if 2 distinct years in one PDF, minYear = decision year for ALL slips in that PDF (both get stored year=minYear), slip with max purpose-year gets advance=true
- FindOrUpdateByPurpose uniqueness key changed from (entrepreneur_id, purpose) to (entrepreneur_id, purpose, advance) to avoid collision when 2025-advance and 2026-main share the same purpose text
- Forms: year (number, prefill=current year) + advance (checkbox) with "?" tooltip icons
- entrepreneur.html slips section: two tabs — Latest (latest non-advance year: regular list + advance accordion) and Previous (each older year: year subheader + 2 accordions)
- Tab data computed in handler via slipYearGroup helper struct

## Key decisions
- Stored year = decision year (minYear from PDF), not purpose year — so both slips from same PDF share year, making grouping natural
- FindOrUpdateByPurpose now also matches on advance to distinguish same-purpose slips with different advance status
- Accordions use native `<details>/<summary>` — no JS needed
- Tabs use simple JS show/hide — no dependency
