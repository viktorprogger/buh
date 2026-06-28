# ADR 005: Tailwind CSS v4 for UI Styling

## Context

The existing Pico CSS CDN link produced an unsatisfying default UI. We need a styling solution that:
- Gives full control over layout and appearance
- Makes the color scheme easy to change globally
- Does not require a complex frontend build pipeline (project is Go-only)

## Alternatives Considered

### A: Tailwind CSS v4 via standalone CLI
Download the pre-compiled `tailwindcss` binary in the Dockerfile; run it as a build step before `go build`. The generated `app.css` is embedded in the Go binary via `//go:embed`.

**Pros:** No Node.js in production image; full Tailwind feature set; `@theme` block gives a single place to change the color scheme.
**Cons:** Standalone binary download needed; first-time dev setup requires running tailwind manually or via air pre_cmd.

### B: Node.js build stage in Dockerfile
Add a `node:alpine` stage that runs `@tailwindcss/postcss` and copies the compiled CSS into the Go build stage.

**Pros:** Official recommended workflow; easy to extend with other PostCSS plugins.
**Cons:** Adds Node.js to the build pipeline; heavier Docker image for dev; overkill for a single-CSS-file project.

### C: Pico CSS (status quo)
Keep the CDN link. Minor customization via Pico's CSS variables.

**Cons:** Limited layout control; default appearance is unattractive; no utility-class approach.

## Decision

**Option A — Tailwind v4 standalone CLI.**

The project has no other JavaScript dependencies and no need for a full Node.js build pipeline. The standalone binary keeps Docker simple. Tailwind v4's `@theme` block in `input.css` defines all color tokens (`--color-primary`, `--color-primary-hover`, `--color-primary-light`) — changing these three hex values switches the entire color scheme.

The slip-form layout CSS (`.slip`, `.slip-left`, etc.) uses physical print units (`0.5mm`, `9pt`) that Tailwind utilities cannot express. It is kept as custom CSS in `slip_styles.html` and remains unchanged.

Generated `app.css` is committed alongside `input.css` so that `go build` (and `//go:embed`) always works without a mandatory pre-step.
