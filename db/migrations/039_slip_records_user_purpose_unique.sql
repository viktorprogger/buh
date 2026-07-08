-- Deduplicate entrepreneur_user_id rows (keep newest) before adding unique constraint.
DELETE FROM slip_records
WHERE id IN (
    SELECT id FROM (
        SELECT id, ROW_NUMBER() OVER (
            PARTITION BY entrepreneur_user_id, purpose, year, advance
            ORDER BY generated_at DESC
        ) AS rn
        FROM slip_records
        WHERE entrepreneur_user_id IS NOT NULL
    ) t
    WHERE rn > 1
);

CREATE UNIQUE INDEX slip_records_user_purpose_uniq
    ON slip_records (entrepreneur_user_id, purpose, year, advance)
    WHERE entrepreneur_user_id IS NOT NULL;
