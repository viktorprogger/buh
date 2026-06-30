CREATE TABLE IF NOT EXISTS invoices (
    id              UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    entrepreneur_id UUID          NOT NULL REFERENCES entrepreneurs(id) ON DELETE CASCADE,
    client_id       UUID          REFERENCES clients(id) ON DELETE SET NULL,
    client_name     TEXT          NOT NULL DEFAULT '',
    invoice_type    TEXT          NOT NULL DEFAULT 'standard',
    invoice_number  TEXT          NOT NULL DEFAULT '',
    issue_date      DATE          NOT NULL,
    period_start    DATE,
    period_end      DATE,
    due_date        DATE,
    currency        TEXT          NOT NULL DEFAULT 'RSD',
    notes           TEXT          NOT NULL DEFAULT '',
    total_rsd       NUMERIC(15,2) NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ   NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS invoice_items (
    id           UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    invoice_id   UUID          NOT NULL REFERENCES invoices(id) ON DELETE CASCADE,
    description  TEXT          NOT NULL DEFAULT '',
    quantity     NUMERIC(10,2) NOT NULL DEFAULT 1,
    unit_price   NUMERIC(15,2) NOT NULL DEFAULT 0,
    discount_pct NUMERIC(5,2)  NOT NULL DEFAULT 0,
    is_product   BOOLEAN       NOT NULL DEFAULT FALSE,
    position     INTEGER       NOT NULL DEFAULT 0
);
