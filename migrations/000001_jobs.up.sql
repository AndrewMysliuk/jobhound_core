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
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
