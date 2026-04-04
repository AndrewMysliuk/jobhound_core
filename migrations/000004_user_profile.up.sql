-- Global stage-3 profile text (009); single implicit user row.
CREATE TABLE user_profile (
    id SMALLINT PRIMARY KEY CHECK (id = 1),
    text TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO user_profile (id, text) VALUES (1, '');
