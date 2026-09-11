# PIB and MB Format Validation

**Date:** 2026-07-07

## Problem
PIB (Serbian tax ID) and MB (registration number) were accepted as free-text with no format validation. Users could enter anything.

## Solution

Created `internal/validate/serbian.go` with:
- `PIB(s string) bool` — exactly 9 digits + modulo-11 check digit (standard Serbian Tax Administration algorithm)
- `MB(s string) bool` — exactly 8 digits

Validation added to three handlers:
- `internal/web/entrepreneur/profile.go` — `handleProfileUpdate`
- `internal/web/accountant/entrepreneur.go` — `handleEntrepreneurNewSubmit`
- `internal/web/accountant/entrepreneur.go` — `handleEntrepreneurUpdate` (redirects to detail page with `?edit_error=` code; detail handler translates the code and passes it to the template)

HTML `pattern="\d{9}"` / `pattern="\d{8}"` added to all PIB/MB inputs in 3 templates for client-side pre-validation (check digit not enforceable client-side).

Error messages added to all 3 locale files (en, sr, ru) under keys: `profile.error_invalid_pib`, `profile.error_invalid_mb`, `entrepreneur_new.error_invalid_pib`, `entrepreneur_new.error_invalid_mb`, `entrepreneur_detail.error_invalid_pib`, `entrepreneur_detail.error_invalid_mb`.

## PIB check digit algorithm
```
P = 10
for each of first 8 digits d:
    P = (P + d) % 10; if P == 0: P = 10; P = (P*2) % 11
check_digit = (11 - P) % 10
```
