CREATE TABLE IF NOT EXISTS kpo_entries (
    id              UUID           PRIMARY KEY DEFAULT gen_random_uuid(),
    kpo_book_id     UUID           NOT NULL REFERENCES kpo_books(id) ON DELETE CASCADE,
    collection_date DATE           NOT NULL,
    invoice_number  TEXT           NOT NULL DEFAULT '',
    product_revenue NUMERIC(15,2)  NOT NULL DEFAULT 0,
    service_revenue NUMERIC(15,2)  NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ    NOT NULL DEFAULT now()
);
