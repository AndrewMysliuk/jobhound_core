-- v2 initial schema: jobs, profile_runs, profile_matches, ingest_watermarks.
-- jobs.id is the dedup key. Profiles live in YAML, not in a table.

CREATE TABLE jobs (
    id TEXT PRIMARY KEY,
    sources TEXT[] NOT NULL DEFAULT '{}'::text[],
    title TEXT NOT NULL DEFAULT '',
    company TEXT NOT NULL DEFAULT '',
    company_key TEXT NOT NULL DEFAULT '',
    company_website TEXT NOT NULL DEFAULT '',
    url TEXT NOT NULL DEFAULT '',
    apply_url TEXT,
    description TEXT NOT NULL DEFAULT '',
    posted_at TIMESTAMPTZ,
    location JSONB NOT NULL DEFAULT '{"type":"","regions":[],"countries":[],"timezone":"","raw":""}'::jsonb,
    salary_raw TEXT NOT NULL DEFAULT '',
    tags JSONB NOT NULL DEFAULT '[]'::jsonb,
    position TEXT,
    first_seen_at TIMESTAMPTZ NOT NULL,
    last_seen_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX jobs_first_seen_at_idx ON jobs (first_seen_at);

CREATE INDEX jobs_last_seen_at_idx ON jobs (last_seen_at);

CREATE INDEX jobs_company_key_idx ON jobs (company_key);

CREATE TABLE profile_runs (
    id BIGSERIAL PRIMARY KEY,
    profile_id TEXT NOT NULL,
    status TEXT NOT NULL,
    started_at TIMESTAMPTZ NOT NULL,
    finished_at TIMESTAMPTZ,
    jobs_scored INT NOT NULL DEFAULT 0,
    sources_skipped INT NOT NULL DEFAULT 0,
    idempotency_key UUID NOT NULL,
    CONSTRAINT profile_runs_idempotency_key_key UNIQUE (idempotency_key)
);

CREATE TABLE profile_matches (
    profile_id TEXT NOT NULL,
    job_id TEXT NOT NULL REFERENCES jobs (id) ON DELETE CASCADE,
    bucket TEXT NOT NULL,
    score INT NOT NULL,
    signals JSONB NOT NULL,
    user_status TEXT NOT NULL DEFAULT 'NEW',
    run_id BIGINT NOT NULL REFERENCES profile_runs (id),
    updated_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (profile_id, job_id),
    CONSTRAINT profile_matches_bucket_check CHECK (bucket IN ('PASSED', 'REJECTED')),
    CONSTRAINT profile_matches_user_status_check CHECK (user_status IN ('NEW', 'HIDDEN'))
);

CREATE INDEX profile_matches_list ON profile_matches (profile_id, bucket, user_status, score DESC);

CREATE TABLE ingest_watermarks (
    source_id TEXT PRIMARY KEY,
    cursor TEXT,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
