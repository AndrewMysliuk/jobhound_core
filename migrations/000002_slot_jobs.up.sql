-- Slot ↔ job membership (specs/008-manual-search-workflow). Candidate pool per search slot.
CREATE TABLE slot_jobs (
    slot_id UUID NOT NULL,
    job_id TEXT NOT NULL REFERENCES jobs (id) ON DELETE CASCADE,
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (slot_id, job_id)
);

CREATE INDEX IF NOT EXISTS slot_jobs_job_id_idx ON slot_jobs (job_id);
