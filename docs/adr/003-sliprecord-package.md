# ADR 003: sliprecord package for DB persistence of payment slips

## Context

The existing `internal/slip` package handles PDF generation via `slip.GeneratePDF`. A new package is needed to persist generated payment slip data to PostgreSQL.

## Alternatives

**Option A: Add DB persistence to `internal/slip`**
Reuse the existing package by adding a `SlipRecord` struct and `Repo` there alongside `GeneratePDF`. Simple in terms of file count, but mixes two concerns: PDF generation (pure function, no DB) and DB persistence (stateful, requires `*sql.DB`). Makes the package harder to test independently and creates an awkward import relationship if other packages want only one of the two responsibilities.

**Option B: New `internal/sliprecord` package**
Separate package for the DB entity and repository. Clear separation of concerns: `slip` = PDF output, `sliprecord` = DB record. Both can evolve independently. Slightly more files, but each package has a single responsibility.

## Decision

**Option B** — new `internal/sliprecord` package.

The name collision risk with `internal/slip` is real (Go disallows two imports with the same base name without aliasing), and the concerns are genuinely different. The cost of an extra package is low; the cost of tangled concerns is higher.

## Consequences

- `internal/slip` remains a pure PDF-generation package with no DB dependency.
- `internal/sliprecord` owns the `SlipRecord` struct and `Repo` with `Save`, `ListByEntrepreneur`, `FindByID`.
- `internal/web` imports both packages without aliasing.
