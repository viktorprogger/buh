CREATE TABLE IF NOT EXISTS kpo_books (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    entrepreneur_id UUID        NOT NULL REFERENCES entrepreneurs(id) ON DELETE CASCADE,
    year            INTEGER     NOT NULL,
    finalized_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(entrepreneur_id, year)
);
