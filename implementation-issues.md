# Implementation issues

## Existing migration 044 has different publication dates

- **Issue:** `db/migrations/044_add_kpo_rate_display_update.sql` publishes update `00000000-0000-0000-0000-000000000045` at `2026-09-01T00:00:00Z`, while the executable migration embedded in `cmd/server.go` uses `2026-08-26T00:00:00Z`.
- **Evidence:** The two migration definitions contain the same update but different `published_at` values.
- **Why out of scope:** This task adds the separate production-optimization update dated `2026-10-25`; changing an already published update's date would be unrelated content/data behavior.
- **Impact/risk:** The reference SQL and executable migration can produce different update dates, which may affect ordering and registration-cutoff visibility near the publication boundary.
- **Recommendation:** Reconcile migration 044 in a dedicated migration/content-maintenance task after confirming which publication date is authoritative.
