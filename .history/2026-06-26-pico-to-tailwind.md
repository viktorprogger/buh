# 2026-06-26 Replace pico.css with Tailwind CSS v4

## Problem
Pico CSS produces an ugly default UI. Need to replace with Tailwind CSS v4 and make the color scheme easily changeable via CSS vars.

## Approach
- Tailwind v4 standalone CLI downloaded in Dockerfile for production build
- `@theme` block in `input.css` defines color scheme variables (`--color-primary`, etc.) — changing these variables changes the entire scheme
- Generated `app.css` committed to git (small artifact, required for Go embed at build time)
- `internal/web/static/` directory added: `input.css` (source) + `app.css` (generated, committed)
- Static files served via `http.FileServer(http.FS(staticFS))` from embedded FS
- `slip_styles.html` custom CSS kept (physical bank form layout with mm units) — only replaced `--pico-primary` → `--color-primary`
- air.toml `pre_cmd` runs tailwind before each Go rebuild in dev

## Solution
- Created `internal/web/static/input.css` with `@import "tailwindcss"` + `@theme` color vars
- All templates rewritten with Tailwind classes; pico-specific `hgroup`, `article`, `role="button"`, pico color classes removed
- Login page is standalone (no base template), rewritten with Tailwind
- ADR created: `docs/adr/002-tailwind-css.md`
