# i18n: Serbian / English / Russian

## Problem
App is entirely in Serbian (Cyrillic). Need to support Serbian, English, Russian.

## Decisions
- **Language resolution**: Accept-Language header as default → cookie for guests → DB preference for logged-in users
- **Library**: `nicksnyder/go-i18n/v2`
- **Template injection**: Register `T` in FuncMap at parse time (placeholder); at render time, clone template and inject real localizer via `Funcs()`
- **PDFs**: UI only; PDFs remain Serbian for now
- **DB**: `language` column in `accountants` and `entrepreneur_users` (default `sr`)

## Why clone-at-render vs embed T in structs
Go template FuncMaps are updated in `execFuncs` which is read at execution time. After `Clone()`, the clone has its own `execFuncs` map. Calling `Funcs()` on the clone before Execute updates only the clone's map. This is safe and correct — verified against Go source.

## Approach
1. Add `go-i18n/v2` dep
2. Create `internal/i18n/` with bundle + context helpers
3. Create `internal/web/locales/{sr,en,ru}.json`
4. Register placeholder `T` in all template FuncMaps at parse time
5. Modify `RenderTemplate`/`RenderNamedTemplate` to accept `*http.Request`, clone, inject T
6. Add language middleware (resolve lang from DB/cookie/Accept-Language)
7. Add `language` column migration
8. Rewrite all ~30 templates to use `{{T "key"}}`
9. Add language switcher in base templates
10. Add `POST /language` endpoint

## Status
In progress
