ALTER TABLE invoices ADD COLUMN IF NOT EXISTS bank_account_id UUID REFERENCES bank_accounts(id) ON DELETE SET NULL;
ALTER TABLE invoices ADD COLUMN IF NOT EXISTS correspondent_bank_id UUID REFERENCES correspondent_banks(id) ON DELETE SET NULL;
