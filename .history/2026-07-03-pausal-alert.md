# Pausal Limit Alerting System

**Date:** 2026-07-03

## Problem

No visual feedback when entrepreneurs approach the pausal turnover limit. Need milestone-based alerts with different severities, shown on both accountant and entrepreneur views, plus a projected alert on invoice creation.

## Approach

1. `shared/pausal.go` — `PausalAlert` struct + `ComputePausalAlert(total, limit, now)` with milestone logic
2. `kpo.Repo.SumForEntrepreneurUser` — new method for entrepreneur-side year totals
3. `templates/pausal_alert.html` — reusable named template fragment (`{{define "pausal_alert"}}`)
4. Parse `pausal_alert.html` alongside templates that need it (entrepreneur.html, entrepreneur_kpo.html)
5. Pass `PausalAlert` to accountant entrepreneur page + entrepreneur KPO page
6. Invoice form: pure JS alert — pass `PAUSAL_CURRENT_TOTAL`, `PAUSAL_LIMIT`, `PAUSAL_YEAR_PCT` as JS vars; JS recomputes on every item change

## Milestone logic

- 90%: alert (red)
- 80%: high (orange)
- 75%: minor (amber)
- 70% + year < 70% passed: minor (amber)
- 50% + year < 50% passed: minor (amber)
- Only shown for current year

## Non-obvious choices

- Invoice form uses JS-only (no Go template) because the alert must update live as the user enters items
- For invoice form, projected = current_total + invoice_RSD; only shown when issue date year == current year (cross-year alert requires additional DB query, deferred)
- `pausal_alert.html` parsed as an extra file alongside each template that needs it (Go template's named-define mechanism)
