# ADR 001: Environment Variable Prefix

## Context

All application env vars need a consistent prefix to avoid collisions with system or third-party variables in the container environment.

## Alternatives Considered

1. **`BUH_`** — matches the Go module name (`module buh`) and the compiled binary name (`buh`). Consistent and discoverable.
2. **No prefix** — simpler names (`DATABASE_URL`, `SESSION_HASH_KEY`) but collides easily with other tools (e.g. `DATABASE_URL` is used by Heroku, many ORMs, and other apps).
3. **`APP_`** — generic, conveys nothing about which app it belongs to; useless when multiple apps share an environment.

## Decision

Use `BUH_` prefix for all application-specific environment variables.

## Reasoning

Matches the binary and module name, making the origin of each variable immediately obvious. Avoids conflicts with system defaults and other tools that consume common names like `DATABASE_URL`.
