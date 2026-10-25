# ADR 013: Accountant access request persistence and notification

## Context

Production currently uses one shared accountant login for multiple people. The application cannot infer which individual is using that login, but the operator needs a low-friction way to collect voluntary email addresses for manual follow-up. The request must survive application-container recreation, remain available when Telegram is unavailable, and must not alter existing authentication or accountant data.

The feature is intentionally low-volume and deployed as one application container. Its durable record needs only the submitted email and whether the operator notification has been sent.

## Alternatives considered

### PostgreSQL table with an in-process or external queue

This would provide stronger querying and a natural shared persistence layer, but it would add schema/migration work and make a small operational request flow depend on queue semantics or a second delivery component. It was rejected because the user explicitly required a two-column CSV and the approved deployment is single-container, low-volume.

### Synchronous Telegram notification before the HTTP response

This would simplify the worker lifecycle and could report immediate delivery failure, but Telegram outages and network latency would block the authenticated form response. It was rejected because persistence must remain available independently of Telegram.

### Ephemeral container-local file or automatic host-directory creation

This would reduce deployment preparation, but requests could disappear on recreation or be silently written outside the intended persistent volume. It was rejected because missing or unwritable storage must fail closed.

## Decision

Use a process-local locked CSV store at `/data/accountant-access-requests.csv` with exactly the columns `email,sent`. Accepted submissions are durably appended before an asynchronous notifier is woken. CSV mutations use complete same-directory temporary-file rewrites and atomic rename; malformed existing files and unavailable storage are operational errors, not reset conditions.

A single in-process notifier retries every unsent row after startup, immediately after a successful append, and hourly. It sets `sent=true` only after a successful Telegram API response. Telegram is at-least-once: a crash after delivery and before the CSV rewrite may produce a duplicate notification. Missing Telegram configuration leaves persisted rows retryable and emits a redacted warning.

The banner is rendered only when the authenticated accountant's stored login equals the hardcoded `viktorprogger@gmail.com` value in `internal/web/accountant/handler.go`. The access-request POST remains under the existing authenticated accountant route. Production mounts a pre-created host directory at `/data` with Compose host-path creation disabled. Account creation, invitations, password handling, email delivery, data copying, and multi-replica coordination are outside this boundary.

## Consequences

- The CSV is simple and inspectable, but it is not a scalable query store and supports only the approved single-container deployment.
- A persistent host directory and backup procedure are deployment prerequisites.
- Telegram outages do not block request persistence or the form response, but duplicate notifications are possible.
- The operator must manually review requests and decide whether and how to create accounts or transfer data.

## Revisit

Reconsider the storage and coordination boundary if the service gains multiple application replicas, materially higher request volume, or a requirement for exactly-once notification or automated account/data ownership workflows.
