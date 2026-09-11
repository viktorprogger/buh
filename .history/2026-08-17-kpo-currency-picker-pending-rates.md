# KPO currency picker and pending historical rates

## Problem

The KPO currency controls only exposed a few currencies and native selects could not provide the requested exact-match search and keyboard focus behavior. A failed historical NBS lookup also rejected otherwise valid user data, while a fallback rate would make RSD totals unauthoritative.

## Decision and approach

Use a shared custom combobox ordered as RSD/EUR/USD followed by the NBS codes alphabetically. Persist an explicit pending-rate state for KPO/invoice conversions instead of rejecting user source data or substituting a fixed rate. Retry pending conversions on later relevant reads and only publish RSD totals after a successful NBS/cache lookup. See ADR 008.

## Implemented

Pending-rate persistence, retry behavior, all NBS currency validation/listing, and the shared keyboard-search currency picker are implemented in the affected domain, handlers, templates, migration, and localization files.

## Validation

`go test ./...`, `go vet ./...`, locale JSON parsing/key parity, Node syntax validation, migration parity, and Tailwind v4 CSS generation pass. An isolated headless Playwright interaction test covers exact filtering, case-insensitive matching, arrow navigation, Enter selection, and active-option reset behavior. Live NBS availability and authenticated PostgreSQL/browser flows remain external gates.
