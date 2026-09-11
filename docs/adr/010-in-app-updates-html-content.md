# ADR 010: Trusted HTML in manually authored update bodies

## Context

The in-app updates feed needs semantic formatting in update descriptions. Update bodies are stored in PostgreSQL and are currently authored only by developers through SQL migrations. The web layer uses `html/template`, which escapes ordinary strings.

## Alternatives considered

1. Keep update bodies as plain text. This preserves the safest rendering boundary but prevents paragraphs, lists, and links in update descriptions.
2. Render database strings as trusted `template.HTML`. This is minimal and supports semantic HTML, but is safe only while the migration-only authoring boundary remains trusted.
3. Add a sanitizer or Markdown pipeline. This would provide a safer future authoring boundary, but introduces a dependency and a larger content-format decision before an admin authoring workflow exists.

## Decision

Store semantic HTML in the existing localized `body_sr`, `body_en`, and `body_ru` text columns. Convert the localized body to `html/template.HTML` in the updates presentation model and keep the template expression as `{{.Body}}`. Production content is added or changed only through reviewed SQL migrations; test fixtures are not part of production migrations.

## Consequences

- HTML such as paragraphs and lists renders without a template helper.
- The database content is a trusted-rendering boundary. No user- or admin-supplied HTML may be written to these fields without adding an allowlist sanitizer or changing the content format first.
- Existing update IDs and read-state rows remain stable when content is revised through a follow-up migration.
- Titles remain plain text and continue to be escaped.
