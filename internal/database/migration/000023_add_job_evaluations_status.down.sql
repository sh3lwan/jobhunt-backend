DROP INDEX IF EXISTS idx_job_evaluations_status;

ALTER TABLE job_evaluations
    DROP COLUMN IF EXISTS error,
    DROP COLUMN IF EXISTS requested_at,
    DROP COLUMN IF EXISTS status;
