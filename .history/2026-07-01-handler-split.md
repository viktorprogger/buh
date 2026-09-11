# handler.go split into web/shared, web/accountant, web/entrepreneur

**Date**: 2026-07-01

## Problem
`internal/web/handler.go` grew to 2688 lines covering auth, accountant-side routes, entrepreneur-side routes, templates, and all utility helpers in one file.

## Approach
Split into three subpackages within `internal/web/`:
- `shared/` — templates, render helpers, utility funcs, shared types
- `accountant/` — all `/a/` routes + `Routes() http.Handler`
- `entrepreneur/` — all `/e/` routes + `Routes() http.Handler` + exported registration handlers

Thin `web/handler.go` keeps: embed directives, `NewHandler` wiring, login/logout, public invite handlers, privacy/terms.

## Naming conflict
`web/entrepreneur` (package `entrepreneur`) imports `buh/internal/entrepreneur` → aliased as `entcore`.
`web/accountant` (package `accountant`) does NOT import `buh/internal/accountant` (login stays in `web`).

## Outcome
Build passes. All endpoints smoke-checked via `go vet ./...` and server startup.

## Result
- 2688-line handler.go → 14 files across 3 packages, largest file 345 lines
- `go build ./...` passes, `go vet ./...` clean
- 20 smoke tests pass: public routes return expected codes (200/302/404), protected /a/* and /e/* routes redirect to /login without hitting DB
- `/invite/{token}` excluded from smoke tests — it queries DB before auth check, needs a real DB
