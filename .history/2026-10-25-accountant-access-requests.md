# Accountant access requests

## Problem

The production accountant account is shared by multiple people, so the existing account identity cannot safely identify which individual should receive access. The service needed a voluntary, durable request flow without changing authentication, creating accounts, copying data, or exposing personal emails in URLs or visit logs.

## Decision and approach

Keep the request record in a two-column CSV, `email,sent`, under persistent `/data` storage rather than adding a database table or external queue. Persist each accepted email before starting asynchronous Telegram notification. Retry unsent rows at startup, after submission, and hourly; accept at-least-once Telegram delivery and keep the CSV authoritative. For the current manual test, render the banner only when the authenticated accountant's stored login equals the hardcoded `viktorprogger@gmail.com` value.

## Implemented

- Added the gated collapsible banner and explicit “enter another address” correction flow.
- Added server-side email validation and POST-only handling.
- Added locked, atomic CSV rewrites with fail-closed storage and malformed-row validation.
- Added asynchronous Telegram delivery with success-only `sent=true` updates, startup/immediate/hourly retries, and graceful cancellation.
- Redacted access-request query strings from visit logs.
- Added production `/data` bind-mount wiring, runtime variables, runbook guidance, and Compose checks in `../buh_prod`.
- Kept account creation, invitations, email delivery, data copying, and authorization changes outside the feature.

## Validation

Application tests, vet, build, targeted race tests, deployment shell tests, rendered Compose validation, and independent final verification passed. No commit or push was made. Live Telegram delivery, production host permissions, backup coverage, container recreation, and rollout behavior remain unverified.

## Follow-up

Before testing, verify the Telegram bot/destination, host-backed `/srv/buh_prod/data` permissions and backup coverage, then run controlled submission, outage/retry, and container-recreation checks. After the manual test, change `accessRequestAccountantEmail` in `internal/web/accountant/handler.go` to the final login. Revisit the CSV/single-replica boundary only if volume or concurrency requirements change; keep account creation and data-copy policy manual until ownership is decided.
