## Problem
Prepare project for long-term maintenance: move logic into `internal/` packages, add Cobra CLI framework, and expose current behavior as a `run` subcommand.

## Approach
- Move `ips/payment.go` → `internal/ips/payment.go` (unchanged)
- Move `ips/slip.go` → `internal/slip/slip.go` (package slip, GenerateSlip → GeneratePDF)
- Extract PDF/QR functions from `main.go` → `internal/extractor/extractor.go`
- Create `cmd/root.go` (rootCmd + Execute) and `cmd/run.go` (orchestration + console output)
- Shrink `main.go` to single `cmd.Execute()` call
- Add `github.com/spf13/cobra` via Dockerfile `go get`

## Cobra project layout (confirmed from docs)
Standard: `cmd/` for commands, `main.go` calls `cmd.Execute()`. Each command in its own file.

## Key design decision
`internal/slip/` is separate from `internal/ips/` to isolate output rendering.
When HTML/PNG output is added, it goes here as `slip.GenerateHTML()` / `slip.GeneratePNG()`.
`internal/extractor/` has no knowledge of IPS — it returns raw `[]string` QR codes.
All orchestration (parse IPS, print, call slip) lives in `cmd/run.go`.

## Solution
```
main.go                → cmd.Execute()
cmd/root.go            → rootCmd + Execute()
cmd/run.go             → processFile, printIPS, writeSlip (command-layer orchestration)
internal/ips/          → Payment, Parse, Validate, Fields, String
internal/extractor/    → ExtractQRCodes(path) (pageCount, []string, error)
internal/slip/         → GeneratePDF(pay, path); future: GenerateHTML, GeneratePNG
```

Usage changes: `qr-extractor <file.pdf>` → `buh run <file.pdf>`
