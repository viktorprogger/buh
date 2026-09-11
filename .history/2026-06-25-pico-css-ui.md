# 2026-06-25 Pico CSS UI

## Problem

All HTML was inline Go string constants in handler.go. Only three pages existed (/login, /, /process results). No entrepreneur or slip detail pages.

## Approach

Extract templates to `internal/web/templates/*.html`, embed with `embed.FS`, parse at startup. Use Pico CSS from CDN.

## Solution

- `base.html` — shared layout: Pico nav (Предузетници / Одјава), `<main class="container">`, footer (Политика приватности / Услови коришћења)
- `login.html` — standalone (no base), centered card layout
- `index.html` — entrepreneur list table; empty state message
- `entrepreneur.html` — shows name+JIPD, upload form, slip history table
- `results.html` — PP50 slip grid preserved with custom CSS; Pico for page chrome
- `slip.html` — slip detail table + re-download form
- `placeholder.html` — generic placeholder for /privacy and /terms

New routes added: GET /entrepreneurs/{id}, GET /slips/{id}, GET /privacy, GET /terms.

handleProcess: added support for explicit `entrepreneur_id` form field (set when uploading from entrepreneur page, bypassing extraction).

handleDownload: removed explicit method check (route pattern POST /download handles it).

## Key decisions

- Templates parsed eagerly in `parseTemplates()` called from `NewHandler` — fail fast on bad templates
- ADR 004 written for template organization choice (embedded files vs inline strings)

## Difficulties

None significant. The Go `html/template` block inheritance requires parsing base + page together per template set — handled cleanly with `mustPageTmpl`.
