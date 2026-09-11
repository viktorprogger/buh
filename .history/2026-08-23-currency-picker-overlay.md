# Currency picker overlay positioning

## Problem

The shared currency picker menu is clipped when opened from the KPO entry row. The picker lives inside a horizontally scrollable table wrapper and a card with `overflow-hidden`; the menu's z-index cannot escape those clipping contexts.

## Decision and approach

Keep the existing picker markup and interaction contract, but move the menu to `document.body` only while it is open. Position it with `position: fixed` from the toggle's viewport rectangle, flip it above the toggle when needed, clamp its horizontal position and height to the current viewport, and reposition on scroll and resize. Use only browser APIs and clean up transient listeners on close.

Alternatives considered were changing overflow styles and only flipping the absolute menu upward. Both were more fragile because they either interfere with table scrolling/card clipping or remain constrained by ancestor overflow.

## Implemented

The shared currency picker JavaScript now owns the temporary body-level overlay lifecycle. Existing exact filtering, case-insensitive matching, keyboard selection, ARIA relationships, hidden form values, and both KPO/invoice usages remain intact. Playwright coverage adds bottom-row clipping, window resize, horizontal table scroll, and mobile viewport behavior checks.

## Validation

The focused currency-picker suite passed 10/10, covering invoice/KPO use, body portal and bottom-row visibility, above/below flipping, window resize, horizontal table scroll, click-away, Escape, HTMX cleanup, mobile viewport resize, and selection. `node --check`, `go test ./...`, `go vet ./...`, locale JSON validation, and the full Playwright suite (`npm test`, 137 passed) also passed. Independent verification returned PASS. Real pinch-pan with nonzero visualViewport offsets remains an environment limitation of headless Chromium, but the production code handles those offsets.

## Follow-up

If the picker is reused inside a native modal or iframe, verify its containing browsing context separately; the overlay remains scoped to the current document.
