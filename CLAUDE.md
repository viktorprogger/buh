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
