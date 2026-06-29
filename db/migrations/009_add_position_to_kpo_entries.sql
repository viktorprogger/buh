ALTER TABLE kpo_entries ADD COLUMN position INTEGER;

UPDATE kpo_entries e
SET position = sub.rn
FROM (
  SELECT id,
         ROW_NUMBER() OVER (PARTITION BY kpo_book_id ORDER BY collection_date, created_at) AS rn
  FROM kpo_entries
) sub
WHERE e.id = sub.id;

ALTER TABLE kpo_entries ALTER COLUMN position SET NOT NULL;
