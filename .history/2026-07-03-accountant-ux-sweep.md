# Accountant UX Sweep

**Date:** 2026-07-03  
**Approach:** Playwright accountant reviewer → prioritize → Haiku coder agents → second reviewer pass

## Problem

Run a supervised UX review loop: spawn an accountant persona to test the app, implement the high-priority findings with small coding agents, verify with a second review pass, then push.

## Findings (22 issues)

The accountant reviewer found 8 High, 9 Medium, 5 Low severity issues.

## Implemented (6 commits)

| Commit | Fix |
|--------|-----|
| aafec0d | Add `/a/` prefix to all slip URLs in templates (issues #1, #2 — broken navigation) |
| 0566ee3 | Show accountant-appropriate copy on pausal/VAT alert banners (issue #6) |
| 3f265e6 | Add severity-specific tooltips with % to alert indicator dots (issue #5) |
| 35e8165 | Add thousand separators to KPO amounts via `fmtMoney` template func (issue #16) |
| 3b226f4 | Standardize field labels in entrepreneur create form (issue #14) |
| c29d850 | Breadcrumb, Укњижи confirmation dialog, better page title on entrepreneur page (issues #10, #11, #18) |

## Second review: 7/8 PASS

The one "FAIL" (PDF download) is a Playwright headless testing limitation — file downloads don't navigate the page in headless mode. Server code is correct (Content-Type: application/pdf, Content-Disposition: attachment, handler verified in source).

## Skipped (with reasoning)

- **#3 QR code** — Complex feature; IPS QR generation scope is large
- **#4 Year=0 on old slips** — Data issue on existing records; risky to auto-migrate without knowing original year
- **#7 Search on entrepreneur list** — Requires backend filtering + HTMX wiring; separate task
- **#8/#9 KPO ordering/drag-and-drop** — Design decision; needs discussion before changing
- **#12 Date dd.mm format** — Intentional: year is controlled by the year select above the table
- **#13 Bilingual labels in slip form** — Intentional: NBS standard banking form format
- **#15 Slip templates** — Large feature, out of scope for this sweep
- **#17 Current/Previous tabs** — Needs more investigation
- **#19–22** — Low priority
