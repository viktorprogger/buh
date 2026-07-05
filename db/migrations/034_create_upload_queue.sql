CREATE TABLE IF NOT EXISTS upload_batches (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    accountant_id UUID        NOT NULL REFERENCES accountants(id) ON DELETE CASCADE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS upload_queue (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    batch_id      UUID        NOT NULL REFERENCES upload_batches(id) ON DELETE CASCADE,
    accountant_id UUID        NOT NULL REFERENCES accountants(id) ON DELETE CASCADE,
    filename      TEXT        NOT NULL,
    file_data     BYTEA,
    status        TEXT        NOT NULL DEFAULT 'pending',
    error_text    TEXT        NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    claimed_at    TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS upload_queue_pending_idx ON upload_queue(created_at) WHERE status = 'pending';
CREATE INDEX IF NOT EXISTS upload_queue_batch_idx   ON upload_queue(batch_id);

CREATE TABLE IF NOT EXISTS upload_results (
    id                  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    batch_id            UUID        NOT NULL REFERENCES upload_batches(id) ON DELETE CASCADE,
    filename            TEXT        NOT NULL,
    entrepreneur_name   TEXT        NOT NULL DEFAULT '',
    entrepreneur_id     UUID,
    is_new_entrepreneur BOOLEAN     NOT NULL DEFAULT FALSE,
    slips_created       INT         NOT NULL DEFAULT 0,
    slips_updated       INT         NOT NULL DEFAULT 0,
    processed_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS upload_results_batch_idx ON upload_results(batch_id);
