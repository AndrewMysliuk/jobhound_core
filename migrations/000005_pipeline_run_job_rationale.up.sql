-- Stage-3 LLM rationale for 009 job list (GET …/stages/3/jobs → stage_3_rationale).
ALTER TABLE pipeline_run_jobs
    ADD COLUMN stage3_rationale TEXT NULL;
