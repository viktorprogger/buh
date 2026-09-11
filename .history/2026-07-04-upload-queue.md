# DB Upload Queue

**Date**: 2026-07-04

## Problem

`POST /a/process` processed PDFs synchronously inside the HTTP handler.
On container restart, all in-flight uploads died mid-loop: some files saved to DB,
the rest lost with no way to know what succeeded. SIGTERM killed the process
immediately (no graceful shutdown).

## Solution

Three-table DB queue (`upload_batches`, `upload_queue`, `upload_results`).

Upload handler now:
1. Saves file bytes to `upload_queue` as BYTEA
2. Redirects to `/a/import/batches/{id}` immediately

Background workers (N = min(CPU, 8)):
1. `SELECT … FOR UPDATE SKIP LOCKED` to claim a row atomically
2. Process it (write to temp file, run extractor, save slips)
3. On success: DELETE queue row + INSERT into upload_results (single transaction)
4. On failure: UPDATE status='failed', file_data=NULL (free space)

On startup: reset all 'processing' → 'pending' (stale rows from crashed instance).

Batch status page auto-refreshes via `<meta http-equiv="refresh" content="2">` while pending > 0; stops automatically when done.

Also added graceful HTTP shutdown: `http.Server.Shutdown()` on SIGTERM/SIGINT with 30s drain window.

## Files

- `internal/uploadqueue/queue.go` — Repo (CreateBatch, Enqueue, Claim, Complete, Fail, ResetStale, BatchSummary)
- `internal/uploadqueue/worker.go` — Worker.Run loop
- `internal/importer/importer.go` — added ProcessFileData(ctx, accountantID, filename, data []byte)
- `internal/web/accountant/handler.go` — removed importer field, added uploadQueue
- `internal/web/accountant/entrepreneur.go` — handleProcess enqueues; new handleBatchStatus
- `internal/web/shared/templates.go` — added UploadBatch
- `internal/web/templates/upload_batch.html` — new batch status page
- `internal/web/templates/base.html` — added head_extra block
- `internal/web/handler.go` — wired uploadqueue.Repo
- `cmd/server.go` — migration 034, worker startup, graceful shutdown
- `db/migrations/034_create_upload_queue.sql` — reference SQL
- i18n locales — batch.* and results.col_entrepreneur keys
