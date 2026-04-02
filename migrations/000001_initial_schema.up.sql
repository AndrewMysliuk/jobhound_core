-- Initial schema (consolidated): jobs, pipeline runs, ingest watermarks.
-- See specs/002-postgres-gorm-migrations/contracts/jobs-schema.md,
-- specs/006-cache-and-ingest/contracts/ingest-watermark-and-filter-key.md,
-- specs/007-llm-policy-and-caps/contracts/pipeline-run-job-status.md.

CREATE TABLE jobs (
    id TEXT PRIMARY KEY,
    source TEXT NOT NULL,
    title TEXT NOT NULL DEFAULT '',
    company TEXT NOT NULL DEFAULT '',
    url TEXT NOT NULL DEFAULT '',
    apply_url TEXT,
    description TEXT NOT NULL DEFAULT '',
    posted_at TIMESTAMPTZ,
    user_id TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    is_remote BOOLEAN,
    country_code TEXT NOT NULL DEFAULT '',
    salary_raw TEXT NOT NULL DEFAULT '',
    tags JSONB NOT NULL DEFAULT '[]'::jsonb,
    position TEXT,
    stage1_status TEXT
        CONSTRAINT jobs_stage1_status_check
        CHECK (stage1_status IS NULL OR stage1_status = 'PASSED_STAGE_1')
);

CREATE INDEX IF NOT EXISTS jobs_created_at_idx ON jobs (created_at);

CREATE INDEX IF NOT EXISTS jobs_source_posted_at_idx ON jobs (source, posted_at);

CREATE INDEX IF NOT EXISTS jobs_passed_stage1_source_posted_at_idx ON jobs (source, posted_at)
    WHERE stage1_status = 'PASSED_STAGE_1';

CREATE TABLE pipeline_runs (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    broad_filter_key_hash TEXT NULL,
    slot_id UUID NULL
);

CREATE INDEX pipeline_runs_slot_id_idx ON pipeline_runs (slot_id);

CREATE TABLE pipeline_run_jobs (
    pipeline_run_id BIGINT NOT NULL REFERENCES pipeline_runs (id) ON DELETE CASCADE,
    job_id TEXT NOT NULL REFERENCES jobs (id) ON DELETE CASCADE,
    status TEXT NOT NULL,
    CONSTRAINT pipeline_run_jobs_status_check CHECK (
        status IN (
            'REJECTED_STAGE_2',
            'PASSED_STAGE_2',
            'PASSED_STAGE_3',
            'REJECTED_STAGE_3'
        )
    ),
    PRIMARY KEY (pipeline_run_id, job_id)
);

CREATE INDEX pipeline_run_jobs_run_id_status_idx ON pipeline_run_jobs (pipeline_run_id, status);

CREATE TABLE ingest_watermarks (
    slot_id UUID NOT NULL,
    source_id TEXT NOT NULL,
    cursor TEXT,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (slot_id, source_id)
);
