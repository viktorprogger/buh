# QR Extractor for APR Tax PDFs

**Date**: 2026-06-23
**Status**: Done

## Problem
Extract IPS bank QR codes from Serbian APR paušal tax decision PDFs.
- `PAUS-RESDOP-*.pdf` — 2 QR codes on last page
- `PAUS-RESPOR-*.pdf` — 6 QR codes on last 2 pages

## Approach
1. Render last 2 pages to PNG via `pdftoppm` (poppler-utils, 200 dpi)
2. Scan for QR codes using `gozxing` multi reader (pure Go, no CGo)
3. All runs inside Docker with `--rm`

## Architecture
- Single-stage Dockerfile: `golang:1.22-bookworm` + `poppler-utils`
- Dependencies fetched at build time via `go get`
- Binary runs with volume-mounted PDFs

## IPS NBS parsing (2026-06-24)

Added `ips/payment.go` — package `ips` with:
- `Payment` struct (all 15 NBS IPS fields)
- `Parse(string) (*Payment, error)` — pipe-delimited parser
- `(*Payment).Validate() error` — validates all fields per NBS spec
- `(*Payment).String() string` — serializer (for future QR generation)
- `(*Payment).Fields() []FieldDisplay` — human-readable field list
- `ParseAmount` / `FormatAmount` helpers for the `I` field
- `IsIPS(string) bool` — quick check before parsing

No external dependencies — pure Go.
All 8 QR codes from both PDFs pass validation as `[IPS NBS — valid]`.

## PP50 slip generation (2026-06-24)

Added `ips/slip.go` — package-level `GenerateSlip(*Payment, outputPath string) error`:
- Generates QR code fresh from `pay.String()` via `github.com/skip2/go-qrcode` (pure Go, Medium error correction)
- Renders HTML PP50 template (210mm × 100mm, CSS `@page`) with embedded QR as base64 data URI
- Converts HTML → PDF via `chromium --headless=new --no-sandbox --print-to-pdf`
- Output: `{source_name}-slip-{n}.pdf` in same directory as source PDF, one file per QR code

Dockerfile additions: `chromium`, `fonts-dejavu`, `go get github.com/skip2/go-qrcode`

Chromium chosen over wkhtmltopdf (unmaintained) and weasyprint (adds Python): best long-term support, scales via process pooling.

## Key findings
- QR codes are embedded as 57×57 px grayscale PNG images at 48 dpi in the PDF
- 400 dpi rendering + `TRY_HARDER` hint needed to detect all codes reliably
- At 200/300 dpi, some finder patterns are missed
- `gozxing/multi/qrcode.NewQRCodeMultiReader()` is the correct constructor (NOT `multi.NewGenericMultipleBarcodeReader`)
- Page 2 of RESPOR has no QR codes (only a decorative banner image 748×23px) — the "no QR codes" warning on that page is expected
