ALTER TABLE jobs
    DROP COLUMN IF EXISTS is_remote,
    DROP COLUMN IF EXISTS country_code;
