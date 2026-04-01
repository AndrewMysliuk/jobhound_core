ALTER TABLE jobs
    ADD COLUMN stage1_status TEXT
        CONSTRAINT jobs_stage1_status_check
        CHECK (stage1_status IS NULL OR stage1_status = 'PASSED_STAGE_1');
