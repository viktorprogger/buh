# ADR 012: Container output for visit logging

## Context

The application needs a short diagnostic entry for each non-static request made by an authenticated account. Operators inspect the application through the container runtime and need these entries in `docker logs`.

## Alternatives considered

1. **Standard Go logger** — Docker collects the process output, requires no extra service or credential, and keeps the request path independent of delivery availability.
2. **File inside the container** — requires a volume and retention/rotation policy and is not directly available through `docker logs`.
3. **External notification service** — provides remote delivery, but adds credentials, network dependency, and a separate operational sink.

## Decision

When `BUH_VISIT_LOGGING_ENABLED=true`, the outer middleware resolves the authoritative account email in a detached goroutine and writes `#buh #visited <email>\\n<method> <RequestURI>` through the standard Go logger. The logger's process output is collected by Docker. The feature remains opt-in, excludes static paths, and uses a short timeout for the identity lookup.

## Consequences

Visit diagnostics are available with `docker logs <container>`. A request does not wait for identity lookup or logging. Entries remain best effort and may be lost during process termination; retention and rotation are controlled by the Docker logging driver.
