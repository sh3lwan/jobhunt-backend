-- Domain gate: cross-domain matches (e.g. nurse vs software CV) are heavily
-- penalized. The multiplier is stored so the UI can explain the final score.
ALTER TABLE cv_job_matches
    ADD COLUMN IF NOT EXISTS domain_multiplier NUMERIC(3, 2) NOT NULL DEFAULT 1.0;
