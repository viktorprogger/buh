# 2026-06-30 — Error pages for 403/404

## Problem
403 and 404 responses used bare `http.Error`/`http.NotFound`, showing Go's plain-text error page instead of a styled UI.

## Solution
- Added `internal/web/templates/error.html` — uses `base.html` layout, shows the error code large, a Serbian-language title and message, and a home-page link
- Added `errPage *template.Template` to the `templates` struct and parsed in `parseTemplates`
- Added `handler.renderError(w, code int)` — looks up the per-code copy, sets the status header, then renders the template
- Replaced all `http.NotFound(w, r)`, `http.Error(w, "Forbidden", …)`, and `http.Error(w, "КПО је укњижен", …)` calls with `h.renderError(w, http.StatusNotFound/Forbidden)`
