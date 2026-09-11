# Web server prototype

**Date:** 2026-06-24

## Problem
Client needs a prototype UI to demonstrate the paušal payment slip workflow.

## Approach
- New `server` CLI command (Cobra), starts HTTP server on configurable port
- Primitive password auth via cookie
- Upload page: Serbian text + up to 4 PDF file inputs
- On submit: extract QR codes from all PDFs, parse IPS, render uplatnice as HTML
- Each uplatnica: full PP50-style layout, editable amount input, "Овде ће бити QR код" placeholder, Download button
- Download: POST form with all IPS fields + edited amount → generate PDF via existing slip.GeneratePDF → serve as attachment

## Files created
- `cmd/server.go` — cobra command
- `internal/web/handler.go` — HTTP handlers + inline HTML templates
