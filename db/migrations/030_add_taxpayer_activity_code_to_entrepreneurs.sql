ALTER TABLE managed_entrepreneurs ADD COLUMN IF NOT EXISTS taxpayer_code TEXT NOT NULL DEFAULT '';
ALTER TABLE managed_entrepreneurs ADD COLUMN IF NOT EXISTS activity_code TEXT NOT NULL DEFAULT '';
