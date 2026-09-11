# Container output for visit logging

## Problem

Authenticated visit diagnostics should be available through `docker logs`, without an external delivery channel or its credentials.

## Decision and approach

Keep the existing opt-in authenticated-request middleware and its detached, bounded identity lookup, but write the visit message through the standard Go logger. Use `BUH_VISIT_LOGGING_ENABLED` as the only visit-logging setting and remove the previous delivery integration and configuration.

## Implemented

`internal/visitlog` writes `#buh #visited <email>\\n<method> <RequestURI>` through the standard Go logger. The application output is collected by Docker, while the existing message, static filtering, identity lookup, and asynchronous best-effort behavior remain unchanged.

## Validation

Focused and full Go tests, race checks, `go vet`, formatting, and `docker compose config` passed. No live enabled server/database or Docker runtime log capture was performed.

## Follow-up

Configure Docker logging retention and rotation according to the deployment environment.
