# KPO foreign-currency conversion via NBS

## Problem

KPO amounts are stored and reported in RSD, but manual entries and invoice-generated entries can originate in a foreign currency. Fixed application rates are not valid for historical KPO dates, and silently falling back would corrupt tax-limit totals.

## Decision and approach

Use the NBS historical middle exchange-rate list for the entry date. Fetch its CSV export only when a non-RSD entry is saved, cache the requested date/currency rate and NBS effective date in PostgreSQL, and reject the save if the authoritative rate is unavailable. Persist original currency amounts and the applied rate snapshot alongside converted RSD amounts. Reuse the same service for automatic KPO entries created from foreign-currency invoices. See [ADR 007](../docs/adr/007-nbs-exchange-rate-cache.md).

## Implemented

Added an on-demand NBS historical middle-rate provider and PostgreSQL cache keyed by requested date and currency. KPO rows now retain original currency amounts, applied NBS rate metadata, and converted RSD amounts; manual and invoice-generated entries share the converter. Existing rows are migrated as unchanged RSD entries, and both KPO views expose currency selection and conversion details.

## Validation

Deterministic provider/cache/unit-conversion tests pass. `go test ./...`, `go vet ./...`, localization JSON parsing, CTX regeneration, and template parsing pass. Live NBS availability remains an external operational dependency.

## Follow-up

None.
