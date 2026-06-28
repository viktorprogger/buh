CREATE TABLE IF NOT EXISTS slip_records (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    entrepreneur_id UUID        NOT NULL REFERENCES entrepreneurs(id) ON DELETE CASCADE,
    payment_code    TEXT        NOT NULL DEFAULT '',
    amount          TEXT        NOT NULL DEFAULT '',
    currency        TEXT        NOT NULL DEFAULT '',
    purpose         TEXT        NOT NULL DEFAULT '',
    payee_account   TEXT        NOT NULL DEFAULT '',
    reference       TEXT        NOT NULL DEFAULT '',
    payee           TEXT        NOT NULL DEFAULT '',
    generated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
