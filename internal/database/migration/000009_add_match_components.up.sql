-- Per-component similarity scores so the UI can show WHY a job matched.
ALTER TABLE cv_job_matches
    ADD COLUMN IF NOT EXISTS canonical_pct        NUMERIC(5, 2),
    ADD COLUMN IF NOT EXISTS skills_pct           NUMERIC(5, 2),
    ADD COLUMN IF NOT EXISTS responsibilities_pct NUMERIC(5, 2);
