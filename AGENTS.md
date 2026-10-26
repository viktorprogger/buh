# Project Instructions

## Development Status

This project is in active development, but the first users already use it in prod. Always make sure you correctly write DB migrations, and all the old data persists and stays uncorrupted.

## DB Migrations

Migrations are **hardcoded in `cmd/server.go`** inside the `runMigrations` function — the SQL files in `db/migrations/` are for reference only and are not executed automatically. When adding a migration, add it to both places. The server applies pending migrations on startup via a `schema_migrations` tracking table.

## Architectural Decisions

Before making any non-obvious architectural decision, **always consider at least two alternatives** and document the reasoning. Save each decision as a new `.md` file in `docs/adr/`:

- Filename: `docs/adr/NNN-short-title.md` (e.g. `docs/adr/001-storage-backend.md`)
- Include: context, alternatives considered, chosen option, and why

Do this for decisions involving: storage format, protocol choice, package structure, external dependencies, concurrency model, or any design tradeoff where a future reader would ask "why not X?"

Mechanical choices (variable names, trivial refactors) do not need ADRs.

## Frontend

Frontend is built using the default go templating.
For styling Tailwind V4 is used, and we should always follow the Tailwind best practices.
For dynamic data load HTMX should always be used.

## Translatins

All the UI texts should be displayed via the i18n package. The transations themselves are in `internal/i18n/locales`.
Currently we support 3 languages: Serbian, English and Russian. The primary one is Serbian, as the service is designed primarily for Serbian accountants. All UI copy must be perfectly clear and unambiguous for the target audience of accountants approximately 20–70 years old; audience comprehension and precise meaning take priority over stylistic preferences, including whether a phrase sounds formal or bureaucratic. Serbian copy should be written in natural Serbian Cyrillic. English and Russian are translations, but they should sound natural too: prefer translation by meaning over literal translation, while preserving the same unambiguous terminology and distinctions.
