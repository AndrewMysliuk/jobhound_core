-- Single local revision (version 1): jobs, companies, profile_runs,
-- profile_matches, ingest_watermarks, job_verdicts. Profiles live in YAML,
-- not in a table. Fresh local stack — no follow-up data migrations.

CREATE TABLE companies (
    key TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    aliases TEXT[] NOT NULL DEFAULT '{}'::text[],
    website TEXT NOT NULL DEFAULT '',
    ats TEXT NOT NULL DEFAULT '',
    ats_slug TEXT NOT NULL DEFAULT '',
    board_url TEXT NOT NULL DEFAULT '',
    board_etag TEXT NOT NULL DEFAULT '',
    resolution_status TEXT NOT NULL,
    resolution_method TEXT NOT NULL DEFAULT '',
    poll_state TEXT NOT NULL DEFAULT 'ACTIVE',
    next_resolve_at TIMESTAMPTZ,
    first_seen_at TIMESTAMPTZ NOT NULL,
    last_seen_at TIMESTAMPTZ NOT NULL,
    last_polled_at TIMESTAMPTZ,
    poll_fail_count INT NOT NULL DEFAULT 0,
    empty_poll_streak INT NOT NULL DEFAULT 0,
    needs_review BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX companies_due_pollable_idx ON companies (resolution_status, poll_state, last_polled_at);

CREATE INDEX companies_website_idx ON companies (website);

CREATE INDEX companies_aliases_gin_idx ON companies USING GIN (aliases);

CREATE INDEX companies_resolve_queue_idx ON companies (resolution_status, next_resolve_at);

CREATE TABLE jobs (
    id TEXT PRIMARY KEY,
    sources TEXT[] NOT NULL DEFAULT '{}'::text[],
    title TEXT NOT NULL DEFAULT '',
    company TEXT NOT NULL DEFAULT '',
    company_key TEXT NOT NULL DEFAULT '',
    company_website TEXT NOT NULL DEFAULT '',
    company_key_ref TEXT REFERENCES companies (key) ON DELETE SET NULL,
    url TEXT NOT NULL DEFAULT '',
    apply_url TEXT,
    description TEXT NOT NULL DEFAULT '',
    posted_at TIMESTAMPTZ,
    location JSONB NOT NULL DEFAULT '{"type":"","regions":[],"countries":[],"timezone":"","raw":""}'::jsonb,
    openings JSONB NOT NULL DEFAULT '[]'::jsonb,
    salary_raw TEXT NOT NULL DEFAULT '',
    tags JSONB NOT NULL DEFAULT '[]'::jsonb,
    position TEXT,
    closed_at TIMESTAMPTZ,
    closed_by TEXT NOT NULL DEFAULT '',
    -- Set on every board UpsertBoardJobs write. CloseAbsent only closes rows
    -- where this is non-null (R1.4 a): aggregator-linked jobs stay on last_seen_at.
    board_seen_at TIMESTAMPTZ,
    first_seen_at TIMESTAMPTZ NOT NULL,
    last_seen_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX jobs_first_seen_at_idx ON jobs (first_seen_at);

CREATE INDEX jobs_last_seen_at_idx ON jobs (last_seen_at);

CREATE INDEX jobs_company_key_idx ON jobs (company_key);

CREATE INDEX jobs_open_closed_at_idx ON jobs (closed_at) WHERE closed_at IS NULL;

CREATE TABLE profile_runs (
    id BIGSERIAL PRIMARY KEY,
    profile_id TEXT NOT NULL,
    status TEXT NOT NULL,
    started_at TIMESTAMPTZ NOT NULL,
    finished_at TIMESTAMPTZ,
    jobs_scored INT NOT NULL DEFAULT 0,
    sources_skipped INT NOT NULL DEFAULT 0,
    companies_discovered INT NOT NULL DEFAULT 0,
    companies_merged INT NOT NULL DEFAULT 0,
    companies_blocked INT NOT NULL DEFAULT 0,
    companies_resolved INT NOT NULL DEFAULT 0,
    companies_unsupported INT NOT NULL DEFAULT 0,
    companies_polled INT NOT NULL DEFAULT 0,
    polls_not_modified INT NOT NULL DEFAULT 0,
    polls_partial INT NOT NULL DEFAULT 0,
    poll_failures INT NOT NULL DEFAULT 0,
    jobs_upserted_ats INT NOT NULL DEFAULT 0,
    jobs_closed INT NOT NULL DEFAULT 0,
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
    CONSTRAINT profile_matches_bucket_check CHECK (bucket IN ('TOP', 'REVIEW', 'REJECTED')),
    CONSTRAINT profile_matches_user_status_check CHECK (user_status IN ('NEW', 'HIDDEN'))
);

CREATE INDEX profile_matches_list ON profile_matches (profile_id, bucket, user_status, score DESC);

CREATE TABLE ingest_watermarks (
    source_id TEXT PRIMARY KEY,
    cursor TEXT,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

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
