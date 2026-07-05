# Plan: DB Upload Queue

**Status**: implementing

## Problem

`POST /a/process` processes PDFs synchronously inside the HTTP request.
On container restart (SIGTERM → SIGKILL after grace period), all in-flight
uploads are killed mid-loop: some files already committed to DB, the rest lost.
Users see a connection error and have no way to know what was saved.

## Solution

Async DB queue: upload saves file bytes to DB and returns immediately with a
batch status page. Background workers drain the queue, delete successful rows,
keep failed rows with error text.

## DB schema (migration 034)

```
upload_batches(id, accountant_id, created_at)
upload_queue(id, batch_id, accountant_id, filename, file_data BYTEA,
             status [pending|processing|failed], error_text, created_at, claimed_at)
upload_results(id, batch_id, filename, entrepreneur_name, entrepreneur_id,
               is_new_entrepreneur, slips_created, slips_updated, processed_at)
```

- On success: DELETE from upload_queue + INSERT into upload_results (one transaction)
- On failure: UPDATE status='failed', file_data=NULL (free space)
- On startup: reset all 'processing' → 'pending' (no worker was running before)

## Files

| File | Change |
|------|--------|
| `cmd/server.go` | migration 034, start N workers, graceful HTTP shutdown |
| `internal/uploadqueue/queue.go` | Repo: Enqueue, CreateBatch, Claim (SKIP LOCKED), Complete, Fail, ResetStale, BatchSummary |
| `internal/uploadqueue/worker.go` | Worker.Run loop: claim → process → complete/fail |
| `internal/importer/importer.go` | add ProcessFileData(ctx, accountantID, filename, data []byte) |
| `internal/web/accountant/handler.go` | add uploadQueue field, new GET /import/batches/{id} route |
| `internal/web/accountant/entrepreneur.go` | handleProcess → enqueue+redirect; add handleBatchStatus |
| `internal/web/shared/templates.go` | add UploadBatch template |
| `internal/web/templates/upload_batch.html` | batch status page with meta-refresh while pending |
| `internal/web/handler.go` | wire uploadqueue.Repo |
| `db/migrations/034_create_upload_queue.sql` | reference SQL |

## UX flow

1. Upload form submits → files saved to DB → redirect to `/a/import/batches/{id}`
2. Batch page: shows pending/done/failed per file, auto-refreshes every 2s via `<meta http-equiv="refresh">`
3. Polling stops when pending = 0 (page renders without the meta tag)
4. Failed rows show filename + error; done rows show entrepreneur + slip counts
