# ADR 008: Searchable NBS currency picker and pending historical rates

## Context

KPO and invoice forms need to expose the complete currency set used by the NBS middle-rate list while remaining usable on a keyboard. Historical NBS data may be temporarily unavailable. Rejecting the user's KPO/invoice data in that case loses input, while substituting an application rate would corrupt RSD totals.

## Alternatives considered

1. Keep native `<select>` elements or use a datalist. These provide some browser keyboard behavior, but do not provide the required exact-match filtering, controlled active-option focus, or deterministic scroll reset consistently across browsers.
2. Keep rejecting saves until NBS responds. This preserves numeric integrity, but loses the user's source amounts and prevents a later retry.
3. Persist a fabricated/legacy rate when NBS is unavailable. This preserves form flow but violates the authoritative-rate requirement and can silently corrupt tax totals.
4. Use a custom combobox and persist an explicit pending-rate state. The original currency amounts remain durable; conversion is retried when the relevant data is viewed or otherwise revisited, and no RSD value is presented as authoritative until a real NBS rate is available.

## Decision

Use one shared accessible custom currency combobox for KPO and invoice forms. It orders RSD, EUR, and USD first, then the remaining NBS currency codes alphabetically. Filtering is exact and case-insensitive; the search input owns `aria-activedescendant`, so the top visible option is the active keyboard option without preventing continued text editing. Arrow keys move the active option and Enter commits it.

Persist `applied` versus `pending` rate status on KPO entries and invoice RSD totals. On an unavailable NBS/cache operation, save the source data with pending status and zero/unknown converted output rather than a fallback. Retry pending conversions on later KPO/invoice reads and update the stored RSD snapshot only after a successful NBS conversion.

## Consequences

- Existing RSD rows remain unchanged and are marked applied.
- Pending rows must be visibly distinguished from a real zero RSD amount and must not be finalized or included as authoritative totals until resolved.
- The static currency-code list must be updated if NBS changes the published list.
- No legacy fixed-rate preview is used for persisted or projected RSD values.
