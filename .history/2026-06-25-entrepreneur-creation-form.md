# Manual Entrepreneur Creation Form

**Date**: 2026-06-25

## Problem
Entrepreneurs were only added implicitly via PDF upload. No way to add one manually.

## Solution
- Added `GET /entrepreneurs/new` — renders `entrepreneur_new.html` with Назив and ЈИПД fields
- Added `POST /entrepreneurs/new` — validates non-empty, calls `entrepreneurs.FindOrCreate`, redirects to `/entrepreneurs/{id}`
- Added `entrepreneurNew` template field to `templates` struct and parsed in `parseTemplates()`
- Added "Нови предузетник" button (Pico `role="button"`) on the index page above the entrepreneur list
- Re-uses session accountant ID, mirrors the session handling pattern from `handleIndex`

## Files changed
- `internal/web/handler.go` — struct field, parseTemplates, two new handlers, two new routes
- `internal/web/templates/index.html` — "Нови предузетник" link
- `internal/web/templates/entrepreneur_new.html` — new template (created)
