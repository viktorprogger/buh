# Bulk slip download format

## Context

A user can select multiple payment slips from a list and download every currently visible slip with one action. Browsers handle one file download per request, while each slip remains an individually generated PDF.

## Alternatives considered

1. Trigger one PDF download per selected slip in browser JavaScript. This can be blocked as pop-ups, produces many download prompts, and gives no single download artifact.
2. Generate a single combined PDF. This would require merging PDFs and risks changing the established single-slip layout and pagination behavior.
3. Generate each existing PDF server-side and return a ZIP archive.

## Chosen option

Return selected PDFs in a ZIP archive from an authorized bulk-download endpoint.

## Why

A ZIP is universally supported, requires only Go's standard library, preserves each existing PDF unchanged, and allows a single predictable browser download. The server authorizes every requested slip before starting the response, so a mixed authorized/unauthorized request cannot yield a partial archive.

## Filename contract

The bulk response is named `uplatnice-<entrepreneur-slug>.zip`. Each generated PDF, whether downloaded individually or included in that archive, is named `uplatnica-<entrepreneur-slug>-<purpose-slug>.pdf`. Slugs are lowercase, ASCII-safe, separator-collapsed, Serbian Latin/Cyrillic-aware, and bounded to keep the resulting filenames portable. The managed entrepreneur's official `Name` is authoritative; direct unpaired slips fall back to their stored payer name and then `slip` when no profile name is available.
