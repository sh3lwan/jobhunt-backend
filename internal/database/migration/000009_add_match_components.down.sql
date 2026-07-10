ALTER TABLE cv_job_matches
    DROP COLUMN IF EXISTS canonical_pct,
    DROP COLUMN IF EXISTS skills_pct,
    DROP COLUMN IF EXISTS responsibilities_pct;
