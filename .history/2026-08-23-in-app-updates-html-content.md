# Trusted HTML for in-app update descriptions

## Problem

The two production update descriptions were stored as plain text, so `html/template` escaped any semantic markup. The update feed needs migration-authored HTML while keeping the production authoring boundary explicit and excluding the Playwright fixture from production migrations.

## Decision and approach

Keep localized bodies in the existing `TEXT` columns and add a follow-up migration that converts only production update IDs `...042` and `...043` to paragraph HTML and dates them `2026-08-23`. Represent localized bodies as `html/template.HTML` in the presentation item so the existing `{{.Body}}` expression renders the trusted markup. Keep titles escaped. A sanitizer/admin authoring workflow remains a separate future decision.

## Implemented

Added migration 043 in the reference SQL and embedded server migrations, preserving update IDs/read state and not inserting test ID `...044`. Updated the update item model and shared list template. The deterministic Playwright fixture and regression assertion now verify that a `<p>` element is rendered.

## Validation

The regression test failed before the implementation because the `<p>` element was absent, then the focused updates suite passed 4/4 after implementation. `go test ./...`, `go vet ./...`, migration parity, locale validation, local row verification, and the complete Playwright suite (`npm test`, 129 passed) also passed. An independent verifier returned PASS with only the documented migration-only trusted-HTML caveat.

## Follow-up

If update authoring expands beyond reviewed migrations, add HTML sanitization or adopt a constrained content format before accepting external input.
