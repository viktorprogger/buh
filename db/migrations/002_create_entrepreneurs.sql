CREATE TABLE IF NOT EXISTS entrepreneurs (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    accountant_id UUID        NOT NULL REFERENCES accountants(id) ON DELETE CASCADE,
    name          TEXT        NOT NULL,
    jipd          TEXT        NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (accountant_id, jipd)
);
