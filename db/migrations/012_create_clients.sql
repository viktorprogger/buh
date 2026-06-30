CREATE TABLE IF NOT EXISTS clients (
    id                  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    entrepreneur_id     UUID        NOT NULL REFERENCES entrepreneurs(id) ON DELETE CASCADE,
    name                TEXT        NOT NULL,
    pib                 TEXT        NOT NULL DEFAULT '',
    registration_number TEXT        NOT NULL DEFAULT '',
    email               TEXT        NOT NULL DEFAULT '',
    address             TEXT        NOT NULL DEFAULT '',
    is_foreign          BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS clients_entrepreneur_id_idx ON clients(entrepreneur_id);
