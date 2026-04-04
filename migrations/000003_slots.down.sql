ALTER TABLE pipeline_runs DROP CONSTRAINT IF EXISTS pipeline_runs_slot_id_fkey;
ALTER TABLE ingest_watermarks DROP CONSTRAINT IF EXISTS ingest_watermarks_slot_id_fkey;
ALTER TABLE slot_jobs DROP CONSTRAINT IF EXISTS slot_jobs_slot_id_fkey;
DROP TABLE IF EXISTS slots;
