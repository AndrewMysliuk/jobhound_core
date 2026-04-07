-- Consolidated initial schema (former 000001–000005): jobs, pipeline, watermarks, slots, slot_jobs, user_profile.
-- See specs/002-postgres-gorm-migrations/contracts/jobs-schema.md,
-- specs/006-cache-and-ingest/contracts/ingest-watermark-and-filter-key.md,
-- specs/007-llm-policy-and-caps/contracts/pipeline-run-job-status.md (pipeline_run_jobs: stage2_status + optional stage3_status),
-- specs/008-manual-search-workflow (slot_jobs), specs/009-http-public-api (slots, user_profile, stage3_rationale).

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

CREATE TABLE slots (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX slots_created_at_idx ON slots (created_at);

-- POST /api/v1/slots: bind Idempotency-Key (UUID) to created slot_id (009).
CREATE TABLE slot_idempotency_keys (
    idempotency_key UUID PRIMARY KEY,
    slot_id UUID NOT NULL REFERENCES slots (id) ON DELETE CASCADE
);

CREATE TABLE pipeline_runs (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    broad_filter_key_hash TEXT NULL,
    slot_id UUID NULL REFERENCES slots (id) ON DELETE CASCADE
);

CREATE INDEX pipeline_runs_slot_id_idx ON pipeline_runs (slot_id);

CREATE TABLE pipeline_run_jobs (
    pipeline_run_id BIGINT NOT NULL REFERENCES pipeline_runs (id) ON DELETE CASCADE,
    job_id TEXT NOT NULL REFERENCES jobs (id) ON DELETE CASCADE,
    stage2_status TEXT NOT NULL,
    stage3_status TEXT NULL,
    stage3_rationale TEXT NULL,
    CONSTRAINT pipeline_run_jobs_stage_check CHECK (
        stage2_status IN ('REJECTED_STAGE_2', 'PASSED_STAGE_2')
            AND (
                stage3_status IS NULL
                OR (
                    stage3_status IN ('PASSED_STAGE_3', 'REJECTED_STAGE_3')
                    AND stage2_status = 'PASSED_STAGE_2'
                )
            )
    ),
    PRIMARY KEY (pipeline_run_id, job_id)
);

CREATE INDEX pipeline_run_jobs_run_id_stage2_stage3_idx ON pipeline_run_jobs (pipeline_run_id, stage2_status, stage3_status);

CREATE TABLE ingest_watermarks (
    slot_id UUID NOT NULL REFERENCES slots (id) ON DELETE CASCADE,
    source_id TEXT NOT NULL,
    cursor TEXT,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (slot_id, source_id)
);

CREATE TABLE slot_jobs (
    slot_id UUID NOT NULL REFERENCES slots (id) ON DELETE CASCADE,
    job_id TEXT NOT NULL REFERENCES jobs (id) ON DELETE CASCADE,
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (slot_id, job_id)
);

CREATE INDEX IF NOT EXISTS slot_jobs_job_id_idx ON slot_jobs (job_id);

CREATE TABLE user_profile (
    id SMALLINT PRIMARY KEY CHECK (id = 1),
    text TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO user_profile (id, text) VALUES (1, '');
