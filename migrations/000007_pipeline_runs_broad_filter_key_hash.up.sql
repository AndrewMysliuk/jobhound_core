-- SHA-256 hex (64 chars), nullable; specs/006-cache-and-ingest/contracts/ingest-watermark-and-filter-key.md §3.
ALTER TABLE pipeline_runs
    ADD COLUMN broad_filter_key_hash TEXT NULL;
