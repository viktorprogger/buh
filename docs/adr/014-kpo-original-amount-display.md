# ADR 014 — KPO web display of original foreign-currency amounts

## Context

KPO entries retain both the original amount entered in a foreign currency and the converted RSD amount. The web tables currently show only the converted amount, while the official KPO PDF must remain an RSD-only representation. The table is already width-constrained, so adding separate source-currency columns would make the primary ledger harder to scan.

## Alternatives considered

### Option A — Add separate original-currency columns (rejected)

This would make both values explicit, but would widen every KPO table, duplicate the existing product/service/total structure, and degrade the accountant's primary RSD view.

### Option B — Hide original values behind a tooltip only (rejected)

This would preserve width, but important source values would be undiscoverable and inaccessible without interaction. It also makes comparison and auditing slower.

### Option C — Show a muted second line in each foreign-currency amount cell (chosen)

Keep the converted RSD value as the primary line and show the original value with its currency code beneath it. Use one concise information tooltip beside the original total to explain that the original amount is not included in the RSD-only PDF. RSD rows remain single-line.

## Decision

Use the existing KPO web templates and localized strings to render original foreign-currency amounts as muted secondary lines beneath the converted RSD values. Qualify all monetary headers with `, RSD`, prevent monetary values from wrapping, and add one localized original-amount/PDF tooltip per foreign row beside the total. Reuse existing persisted fields and leave conversion, persistence, totals, and PDF generation unchanged.
