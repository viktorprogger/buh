CREATE TABLE IF NOT EXISTS bank_accounts (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    entrepreneur_id UUID        NOT NULL REFERENCES entrepreneurs(id) ON DELETE CASCADE,
    account_type    TEXT        NOT NULL DEFAULT 'local', -- 'local' or 'foreign'
    bank_name       TEXT        NOT NULL DEFAULT '',
    account_number  TEXT        NOT NULL DEFAULT '',
    iban            TEXT        NOT NULL DEFAULT '',
    swift           TEXT        NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS bank_accounts_entrepreneur_id_idx ON bank_accounts(entrepreneur_id);

CREATE TABLE IF NOT EXISTS correspondent_banks (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    bank_account_id UUID        NOT NULL REFERENCES bank_accounts(id) ON DELETE CASCADE,
    bank_name       TEXT        NOT NULL DEFAULT '',
    swift           TEXT        NOT NULL DEFAULT '',
    bank_address    TEXT        NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
