# In-app updates

## Problem

Authenticated users need a multilingual, manually maintained update list. Updates published before a user's registration must not appear, while viewed updates must remain available without unread emphasis.

## Decision and approach

Store explicit Serbian, English, and Russian title/body values in `app_updates`, and store per-user read markers in `app_update_reads`. Use the existing account `created_at` timestamps as the registration cutoff and require `published_at > created_at`. Manual entries are seeded through migrations rather than adding an admin interface. Add an authenticated `/updates` page, an unread-count navigation link above logout, and mark displayed updates read when the page is opened.

## Implemented

Added migration 042 and the `internal/updates` repository, with explicit Serbian/English/Russian content, strict registration cutoffs, per-user read markers, and idempotent read marking on `/updates` visits. Added role-aware navigation links above logout, localized accountant and entrepreneur updates pages, subdued styling for read items, and deterministic Go/Playwright coverage.

## Validation

`go test ./...`, `go vet ./...`, JSON validation, embedded/reference migration parity, and the complete Playwright suite passed (129 tests). Focused updates Playwright coverage passed 4/4.
