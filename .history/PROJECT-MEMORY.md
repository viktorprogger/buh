# Project Memory

## Shared accountant access requests

- **Current rule:** Only the authenticated accountant whose stored login equals the hardcoded `viktorprogger@gmail.com` in `internal/web/accountant/handler.go` sees the voluntary access-request banner. Accepted emails are stored as distinct rows in `/data/accountant-access-requests.csv` with exactly `email,sent`; storage is process-locally locked, atomically rewritten, and fail-closed when the mount is missing or malformed. Telegram notification is asynchronous and at-least-once, with startup, immediate, and hourly retries; `sent` changes only after successful delivery.
- **Why:** The shared login cannot reliably identify individuals, while requests must survive Telegram outages and container recreation without changing existing access or automatically creating accounts/copying data.
- **Authority:** [ADR 013 — accountant access request persistence and notification](../docs/adr/013-accountant-access-requests.md), [`internal/accessrequest`](../internal/accessrequest), [`../buh_prod/compose.yaml`](../../buh_prod/compose.yaml), and the [production migration runbook](../../buh_prod/docs/production-migration-runbook.md).
- **History:** [Accountant access requests](2026-10-25-accountant-access-requests.md).
- **Revisit:** Reconsider the CSV and single-replica boundary if volume, replication, exactly-once delivery, or automated account/data ownership becomes a requirement. Keep account creation and data-copy decisions manual until the ownership policy is explicit.

## Production deployment orchestration

- **Current rule:** The application workflow publishes the immutable production image and participates in GitHub Actions automatic reconciliation: weekday eligibility is `20:00–21:59 Europe/Belgrade`, weekend pushes are unrestricted, and `workflow_dispatch` is immediate. Telegram reports attempted deployment outcomes without changing workflow conclusions.
- **Why:** Deployment timing and effective-state deduplication are owned by the production workflow/server boundary while image publication remains in this repository.
- **Authority:** [CI and CD workflow](../.github/workflows/ci-cd.yml) and the production repository's deployment ADR.
- **History:** The deployment reconciliation record is maintained in `buh_prod/.history/`.
- **Revisit:** Reconsider if deployment ownership moves to a single cross-repository coordinator or exact-time scheduling becomes required.

## Payment slip download filenames

- **Current rule:** Single slip PDFs and PDFs inside bulk archives use `uplatnica-<official-entrepreneur-slug>-<purpose-slug>.pdf`; bulk responses use `uplatnice-<official-entrepreneur-slug>.zip`. Managed entrepreneur `Name` is authoritative, with payer/`slip` fallback for direct unpaired slips.
- **Why:** Downloaded files must remain recognizable when separated from the application, while one HTTP response cannot reliably deliver multiple independent browser downloads.
- **Authority:** [ADR 001 — bulk slip download format](../docs/adr/001-bulk-slip-download-format.md), [`internal/web/shared/slip_archive.go`](../internal/web/shared/slip_archive.go), and the accountant/entrepreneur slip handlers.
- **History:** [Uplatnica download filenames](2026-09-11-uplatnica-download-filenames.md), [Bulk Slip Download](2026-07-08-bulk-slip-download.md).
- **Revisit:** Reconsider separate browser downloads only with an explicit client-side delivery design and browser compatibility testing.

## Container visit logging

- **Current rule:** When explicitly enabled, outer middleware logs non-static requests with valid accountant or entrepreneur sessions using the authoritative account email and the exact `#buh #visited <email>\\n<method> <RequestURI>` message through the standard Go logger collected by Docker.
- **Why:** Visit diagnostics must be available through `docker logs` without an external delivery dependency or delivery credentials.
- **Authority:** [`internal/visitlog`](../internal/visitlog) and [`compose.yml`](../compose.yml).
- **History:** [Container output for visit logging](2026-09-11-container-visit-logging.md).
- **Revisit:** Reassess per-visit goroutines only if traffic volume or delivery durability becomes material.

## In-app updates

- **Current rule:** Updates are manually added through migrations with Serbian, English, and Russian title/body fields; bodies may contain trusted semantic HTML authored only in reviewed migrations, while titles remain escaped plain text. Users see only rows whose `published_at` is strictly after their account `created_at`, and opening `/updates` records per-user read state while retaining read rows in subdued presentation.
- **Why:** Publication must be independent from migration time, registration cutoffs must be durable, and semantic formatting is useful without introducing an authoring UI. The trusted HTML boundary is acceptable only while migrations are the sole authoring path.
- **Authority:** [ADR 009 — In-app updates storage and read state](../docs/adr/009-in-app-updates.md), [ADR 010 — Trusted HTML in update bodies](../docs/adr/010-in-app-updates-html-content.md), [`internal/updates`](../internal/updates), and migrations [`042_create_app_updates.sql`](../db/migrations/042_create_app_updates.sql) and [`043_update_initial_app_updates_content.sql`](../db/migrations/043_update_initial_app_updates_content.sql).
- **History:** [In-app updates](2026-08-19-in-app-updates.md), [Trusted HTML for in-app update descriptions](2026-08-23-in-app-updates-html-content.md).
- **Revisit:** Add a sanitizer or constrained content format before introducing admin/user-authored HTML; add a deliberate schema change if supported locales expand.

## KPO web amount display

- **Current rule:** KPO web tables show the converted RSD amount as the primary value and, for foreign-currency rows, the original amount with its currency code as a muted secondary value. Monetary headers identify RSD, and one localized tooltip beside the original total states that the official KPO PDF contains only the RSD amount; the PDF remains RSD-only.
- **Why:** Inline secondary values preserve the compact accountant-facing table while making source amounts visible without confusing them with the official RSD ledger output.
- **Authority:** [ADR 014 — KPO web display of original foreign-currency amounts](../docs/adr/014-kpo-original-amount-display.md), the KPO web templates, and the locale bundles.
- **History:** [KPO original amount display](2026-10-26-kpo-original-amount-display.md).
- **Revisit:** Reconsider if the official KPO representation or the web table's column structure changes.

## KPO currency conversion

- **Current rule:** Non-RSD KPO and invoice amounts use the NBS historical middle rate for the operation date. Original amounts and applied snapshots are persisted; when NBS/cache data is unavailable, source data is saved with explicit `pending` status and retried on later relevant reads, never with a fallback rate. The shared currency picker renders its open menu as a body-level fixed overlay, repositioned on scroll and viewport changes.
- **Why:** KPO totals, tax-limit alerts, PDFs, and invoice-generated entries must remain reproducible and must not depend on an invented or later-changing rate. The body-level overlay avoids overflow clipping without changing table scrolling or picker interaction contracts.
- **Authority:** [ADR 007 — NBS exchange-rate cache](../docs/adr/007-nbs-exchange-rate-cache.md), [ADR 008 — currency picker and pending rates](../docs/adr/008-currency-picker-and-pending-rates.md), [ADR 011 — body-level currency picker overlay](../docs/adr/011-currency-picker-overlay.md); migrations [`040_create_exchange_rates.sql`](../db/migrations/040_create_exchange_rates.sql) and [`041_add_pending_rate_status.sql`](../db/migrations/041_add_pending_rate_status.sql).
- **History:** [KPO NBS exchange-rate conversion](2026-08-17-kpo-nbs-exchange-rates.md), [KPO currency picker and pending historical rates](2026-08-17-kpo-currency-picker-pending-rates.md), [Currency picker overlay positioning](2026-08-23-currency-picker-overlay.md).
- **Revisit:** Update the static NBS currency-code list or adapter if the official archive changes its published list or format; revisit overlay positioning if the picker is embedded in an iframe or native modal.
