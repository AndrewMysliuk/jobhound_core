-- Search slots (009 HTTP public API): named slot rows; FK from slot-scoped tables for CASCADE delete.
CREATE TABLE slots (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX slots_created_at_idx ON slots (created_at);

ALTER TABLE slot_jobs
    ADD CONSTRAINT slot_jobs_slot_id_fkey
    FOREIGN KEY (slot_id) REFERENCES slots (id) ON DELETE CASCADE;

ALTER TABLE ingest_watermarks
    ADD CONSTRAINT ingest_watermarks_slot_id_fkey
    FOREIGN KEY (slot_id) REFERENCES slots (id) ON DELETE CASCADE;

ALTER TABLE pipeline_runs
    ADD CONSTRAINT pipeline_runs_slot_id_fkey
    FOREIGN KEY (slot_id) REFERENCES slots (id) ON DELETE CASCADE;
