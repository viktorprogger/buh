# Uplatnica download filenames

## Problem
Downloaded payment slips currently use the payee name for PDFs and a fixed `uplatnice.zip` archive name. Users need files grouped and recognizable by the official entrepreneur name and payment purpose.

## Decision and approach
Keep the ZIP as the single bulk-download artifact: one HTTP response cannot reliably deliver multiple independent files, and the existing architecture intentionally avoids browser popup/multiple-download behavior. Use slugified official entrepreneur and purpose values in every PDF filename, and the entrepreneur slug in the ZIP filename. For entrepreneur-owned slips without a paired managed profile, use the stored payer name as the available official-name fallback.

## Implemented
Shared filename generation now slugifies the official managed entrepreneur name and payment purpose for single PDFs and ZIP entries. Bulk responses use the matching entrepreneur slug, with safe payer/`slip` fallbacks for direct unpaired slips, bounded components, and deterministic duplicate-entry suffixes.

## Validation
Focused shared, web, race, vet, and fresh independent verification passed. The full Go suite passed; Playwright filename scenarios remain to be run with `BUH_DATABASE_URL` configured.

## Follow-up
Run the focused Playwright specs in an environment with database access.
