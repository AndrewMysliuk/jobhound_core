-- Minimal pipeline run header (007 contract §4). Child: pipeline_run_jobs (§5).
CREATE TABLE pipeline_runs (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Per-run stage 2/3 outcomes (007 contract §5).
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
