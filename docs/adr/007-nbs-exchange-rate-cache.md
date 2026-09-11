# ADR 007 — NBS exchange-rate cache for KPO

## Context

KPO values must be recorded in Serbian dinars even when the underlying revenue is entered in a foreign currency. The National Bank of Serbia publishes historical middle exchange-rate lists, including the currency unit (for example, 100 HUF), so using a fixed application-wide rate is not acceptable for historical entries.

The exchange rate must be obtained only when a non-RSD amount is saved, and the result must remain reproducible if the NBS archive or rates change later.

## Alternatives considered

### Option A — Keep a fixed rate table in application code (rejected)

This is already used for invoice previews. It is simple and available offline, but it is inaccurate for historical dates and cannot provide an auditable source for KPO values.

### Option B — Request NBS for every conversion without persistence (rejected)

This would use the authoritative historical list, but makes old entries dependent on network availability and on later changes in the external archive. It also repeats requests for the same date and currency.

### Option C — Fetch the NBS historical CSV on demand and cache it in the database (chosen)

On saving a non-RSD KPO entry, the application requests the NBS middle-rate list for that date, downloads its CSV export, extracts the currency rate and unit, and stores the requested date, effective list date, rate, unit, and source URL in `exchange_rates`. Subsequent conversions for the same date and currency use the cached row.

## Decision

Use the NBS historical middle exchange rate. Store both the original KPO amounts and the converted RSD amounts together with the applied rate snapshot and effective NBS date. Existing KPO rows are treated as RSD rows during migration. If NBS cannot provide a required rate, the entry is rejected rather than silently using a fallback rate.

The same conversion service is used when a foreign-currency invoice automatically creates a KPO entry, so that automatic and manual entries follow one historical-rate rule.
