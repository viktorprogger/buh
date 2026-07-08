ALTER TABLE slip_records RENAME COLUMN entrepreneur_id TO managed_entrepreneur_id;
ALTER TABLE slip_records ALTER COLUMN managed_entrepreneur_id DROP NOT NULL;
ALTER TABLE slip_records ADD COLUMN entrepreneur_user_id UUID REFERENCES entrepreneur_users(id) ON DELETE CASCADE;
ALTER TABLE slip_records ADD CONSTRAINT slip_records_owner_xor CHECK (
    (managed_entrepreneur_id IS NOT NULL AND entrepreneur_user_id IS NULL) OR
    (managed_entrepreneur_id IS NULL AND entrepreneur_user_id IS NOT NULL)
);
