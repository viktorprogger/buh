CREATE TABLE IF NOT EXISTS slip_merges (
    id                      UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    managed_entrepreneur_id UUID        NOT NULL REFERENCES managed_entrepreneurs(id) ON DELETE CASCADE,
    year                    INT         NOT NULL,
    merged_at               TIMESTAMPTZ NOT NULL DEFAULT now(),
    merged_by               UUID        NOT NULL,
    UNIQUE (managed_entrepreneur_id, year)
);
