-- One ready model reply per (job_id, prompt_version, model, input_hash).
-- Pending is the absence of a row. No status column.

CREATE TABLE job_verdicts (
    job_id TEXT NOT NULL REFERENCES jobs (id) ON DELETE CASCADE,
    prompt_version INT NOT NULL,
    model TEXT NOT NULL,
    input_hash TEXT NOT NULL,
    facts JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT job_verdicts_job_id_prompt_version_model_input_hash_key UNIQUE (job_id, prompt_version, model, input_hash)
);
