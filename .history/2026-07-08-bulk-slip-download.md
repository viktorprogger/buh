# Bulk Slip Download

**Date:** 2026-07-08

## Problem

Accountants need to choose individual visible payment slips or download every visible slip in one action.

## Approach

Added checkbox selection and “download selected” / “download all visible” controls to both accountant and entrepreneur slip lists. Each role has a bulk-download endpoint that authorizes every requested slip before invoking the common archive generator. The response is a ZIP containing one existing-format PDF per slip; duplicate filenames are numbered.

## Validation

- `go test ./...`
- ZIP unit test verifies a generated PDF is included with a valid PDF header.
