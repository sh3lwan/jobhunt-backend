ALTER TABLE applications DROP COLUMN IF EXISTS tailored_cv_path;

DROP INDEX IF EXISTS idx_job_evaluations_score;

DROP TABLE IF EXISTS job_evaluations;
