ALTER TABLE kpo_entries ADD COLUMN IF NOT EXISTS rate_status TEXT NOT NULL DEFAULT 'applied';
ALTER TABLE invoices ADD COLUMN IF NOT EXISTS original_total NUMERIC(15,2) NOT NULL DEFAULT 0;
ALTER TABLE invoices ADD COLUMN IF NOT EXISTS rate_status TEXT NOT NULL DEFAULT 'applied';
UPDATE kpo_entries SET rate_status = 'applied' WHERE rate_status IS NULL OR rate_status = '';
-- Before this migration invoices did not retain their source amount. Preserve
-- their existing persisted RSD total instead of leaving the new field at zero.
UPDATE invoices SET original_total = total_rsd WHERE original_total = 0;
UPDATE invoices SET rate_status = 'applied' WHERE rate_status IS NULL OR rate_status = '';
ALTER TABLE kpo_entries ADD CONSTRAINT kpo_entries_rate_status CHECK (rate_status IN ('applied','pending'));
ALTER TABLE invoices ADD CONSTRAINT invoices_rate_status CHECK (rate_status IN ('applied','pending'));
