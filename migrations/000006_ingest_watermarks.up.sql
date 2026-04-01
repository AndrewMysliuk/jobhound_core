-- Per specs/006-cache-and-ingest/contracts/ingest-watermark-and-filter-key.md §1.
CREATE TABLE ingest_watermarks (
    source_id TEXT PRIMARY KEY,
    cursor TEXT,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
