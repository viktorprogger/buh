CREATE TABLE IF NOT EXISTS exchange_rates (
    requested_date DATE NOT NULL,
    currency TEXT NOT NULL,
    effective_date DATE NOT NULL,
    unit INTEGER NOT NULL,
    middle_rate NUMERIC(15,6) NOT NULL,
    fetched_at TIMESTAMPTZ NOT NULL,
    source_url TEXT NOT NULL,
    PRIMARY KEY (requested_date, currency)
);

ALTER TABLE kpo_entries ADD COLUMN IF NOT EXISTS original_product_revenue NUMERIC(15,2) NOT NULL DEFAULT 0;
ALTER TABLE kpo_entries ADD COLUMN IF NOT EXISTS original_service_revenue NUMERIC(15,2) NOT NULL DEFAULT 0;
ALTER TABLE kpo_entries ADD COLUMN IF NOT EXISTS currency TEXT NOT NULL DEFAULT 'RSD';
ALTER TABLE kpo_entries ADD COLUMN IF NOT EXISTS rate_unit INTEGER NOT NULL DEFAULT 1;
ALTER TABLE kpo_entries ADD COLUMN IF NOT EXISTS rate_middle NUMERIC(15,6) NOT NULL DEFAULT 1;
ALTER TABLE kpo_entries ADD COLUMN IF NOT EXISTS rate_effective_date DATE NOT NULL DEFAULT CURRENT_DATE;
ALTER TABLE kpo_entries ADD COLUMN IF NOT EXISTS rate_fetched_at TIMESTAMPTZ NOT NULL DEFAULT now();
ALTER TABLE kpo_entries ADD COLUMN IF NOT EXISTS rate_source_url TEXT NOT NULL DEFAULT '';

-- Existing KPO values were already recorded in RSD. Preserve them verbatim and
-- make the new audit fields describe that fact rather than an external fetch.
UPDATE kpo_entries
SET original_product_revenue = product_revenue,
    original_service_revenue = service_revenue,
    currency = 'RSD',
    rate_unit = 1,
    rate_middle = 1,
    rate_effective_date = collection_date,
    rate_source_url = ''
WHERE currency = 'RSD';
