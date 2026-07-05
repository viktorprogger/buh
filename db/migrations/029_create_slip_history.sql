CREATE TABLE IF NOT EXISTS slip_history (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    slip_id    UUID        NOT NULL,
    event      TEXT        NOT NULL,
    actor_type TEXT        NOT NULL,
    actor_id   UUID        NOT NULL,
    changes    JSONB       NOT NULL DEFAULT '{}',
    changed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS slip_history_slip_id_idx ON slip_history(slip_id);
