-- Retention (created_at cutoff) and stage-1 style reads per 006 spec.md / retention-jobs.md §4.
CREATE INDEX IF NOT EXISTS jobs_created_at_idx ON jobs (created_at);

CREATE INDEX IF NOT EXISTS jobs_source_posted_at_idx ON jobs (source, posted_at);

CREATE INDEX IF NOT EXISTS jobs_passed_stage1_source_posted_at_idx ON jobs (source, posted_at)
    WHERE stage1_status = 'PASSED_STAGE_1';
