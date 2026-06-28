# ADR 004: Template organization — embedded files vs inline strings

## Context

The web handler needed five distinct HTML pages (login, index, entrepreneur, results, slip) plus two placeholder pages. The original code kept all templates as inline Go string constants. With Pico CSS, proper nav/footer, and the PP50 slip layout, the total HTML grew to ~400+ lines per file.

## Alternatives

**A. Inline string constants** (original approach)  
Keeps everything in one Go file; no file embedding needed. Hard to read/edit as templates grow. IDE HTML tooling does not activate inside Go string literals.

**B. Separate `*.html` files in `internal/web/templates/`, embedded via `embed.FS`**  
Templates are real `.html` files — IDE tooling works, syntax highlighting, formatters. Templates are parsed at startup (`NewHandler`) so errors surface immediately. Slightly more files to manage.

## Decision

Option B — separate embedded template files.

Rationale: the template volume makes option A unmanageable. `embed.FS` is zero-dependency (stdlib since Go 1.16) and adds no runtime overhead. Parsing at startup gives the same fail-fast behaviour as `template.Must(template.New(...).Parse(...))`.

## Implementation

- `internal/web/templates/base.html` — shared layout with nav and footer
- One `*.html` per page; each calls `{{template "base" .}}` and overrides `{{define "content"}}` etc.
- `mustPageTmpl(name, funcs)` parses `base.html` + the page file into one `*template.Template`
- All templates pre-parsed in `parseTemplates()` called from `NewHandler`
