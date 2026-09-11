# ADR 011: Body-level overlay for the currency picker menu

## Context

The shared currency picker is rendered inside KPO table cells and invoice forms. In the KPO table, the picker is inside containers with `overflow-x-auto` and `overflow-hidden`. Its absolutely positioned menu is therefore clipped when opened near the bottom of the table, even when its z-index is increased.

The picker must retain exact filtering, keyboard navigation, accessibility attributes, and use on both KPO and invoice forms. The fix should work on desktop and mobile without a dependency or continuous background work.

## Alternatives considered

1. Change the KPO containers to `overflow-visible` or increase the menu z-index. This is small, but cannot reliably preserve horizontal table scrolling and rounded card clipping; z-index cannot escape an overflow clipping context.
2. Flip the existing absolute menu upward when the trigger is near the bottom. This avoids the reported case with little code, but remains constrained by ancestor clipping and is fragile when the available space changes during scrolling or mobile viewport resize.
3. Move the open menu to `document.body` and position it as a viewport-fixed overlay. This escapes table clipping while preserving the existing picker DOM, filtering, keyboard behavior, and selected value. Positioning is recalculated on scroll and resize, with viewport-aware direction and bounds.

## Decision

Use the body-level overlay approach only while a picker is open. Keep the menu in its original picker while closed; on open, append it to `document.body`, set fixed coordinates from the toggle's `getBoundingClientRect()`, and restore it to its original parent on close. Reposition on captured scroll, window resize, and `visualViewport` resize/scroll where available. Throttle repositioning with `requestAnimationFrame`, and remove all transient listeners when the menu closes.

## Consequences

- Currency menus are no longer clipped by KPO table/card overflow containers.
- Search state, focus, keyboard navigation, and the existing hidden form input remain unchanged.
- The overlay must treat clicks inside the reparented menu as inside the active picker and must remain above the viewport without allowing horizontal overflow.
- The implementation uses only browser APIs and no third-party dependency. A responsive Playwright regression suite covers KPO bottom-row visibility, scroll/resize repositioning, and invoice picker behavior.
