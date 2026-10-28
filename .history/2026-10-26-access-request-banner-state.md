# Access-request banner state

## Problem
The accountant access-request banner always opens on every page load, and its success presentation remains intrusive after navigation or reload. The UI needs browser-local persistence without changing the server-side request and notification contract.

## Decision and approach
Keep the existing server-side POST/redirect and CSV notification flow. Add a small browser-side state machine backed by `localStorage`: default open state is remembered, a submitted request remains a compact collapsed success banner across pages and reloads, and choosing “enter another email” resets the banner to the original expanded form state as if nothing had been submitted. Remove the success query from browser history at that point so the reset survives reload. Render both banner modes in the shared template so client-side restoration does not depend on the current URL.

## Implemented
The shared accountant template now renders default and submitted banner modes with stable data hooks. `internal/web/static/access-request.js` stores `open`, `submitted`, and `editing` under `buh.access-request.v1`, restores the first-visit/default state, keeps submitted banners compact and collapsed after navigation/reload, and resets the complete banner to the original header, descriptions, colors, and form after “enter another email”. The `access_submitted=1` query is removed from browser history during that reset. Server POST, redirect, CSV storage, and notification behavior remain unchanged.

## Validation
- Test-first structural coverage was added; the new tests failed before implementation and pass afterward.
- `go test ./...` passed.
- `go build ./...` passed.
- `node --check internal/web/static/access-request.js` passed.
- `git diff --check` passed.
- Fresh read-only verification found no defects; it also confirmed the existing access-request handler contract and embedded static filesystem remain in scope.
- Headless Chrome exercised first visit, manual collapse persistence, fresh success, quiet success after reload/navigation, and the “enter another email” reset flow, including clearing `access_submitted=1` from the URL. The embedded script returned HTTP 200 from the running app.

## Follow-up
None.
