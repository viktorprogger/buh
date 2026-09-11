# ADR 009: In-app updates storage and read state

## Context

The application needs a manually maintained list of product updates for both authenticated user types. Each user must see only updates published after their registration, while updates they have already viewed must remain available without retaining unread emphasis. Update text must be available in Serbian, English, and Russian.

## Alternatives considered

1. **Localized JSONB content on each update row** — flexible for adding languages, but weakly constrained and less discoverable for manual SQL maintenance.
2. **A normalized translation table** — flexible and relational, but adds a join and more rows for the small, fixed set of supported locales.
3. **Fixed locale columns on the update row** — explicit, easy to edit manually in migrations, and consistent with the application's currently supported three locales; adding a future locale requires a deliberate schema migration.

## Decision

Use an `app_updates` table with explicit Serbian, English, and Russian title/body columns, plus a separate `app_update_reads` table keyed by `(update_id, user_type, user_id)`. Manual updates are added through SQL migrations; no admin UI is introduced in this slice.

The update repository filters with `published_at > registered_at`, where `registered_at` is read from the authenticated user's existing `created_at`. Visiting the authenticated `/updates` page marks all updates displayed for that user as read. The navigation displays an Updates link and unread count; the updates page keeps read items visible with subdued styling.

## Why

This is the smallest durable design that preserves explicit multilingual content, supports per-user read state for both existing user tables, and makes publication time independent from migration application time. A polymorphic user reference is used for the read table because accountant and entrepreneur accounts live in separate tables; user type is constrained and authorization is performed from the session before marking reads.
