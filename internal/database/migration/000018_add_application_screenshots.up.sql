-- Full-page PNGs captured by the applier at submit time so the user can see
-- exactly what was submitted (the filled form, including which CV was attached)
-- and the outcome page (success/error) after clicking Submit.
ALTER TABLE applications
  ADD COLUMN IF NOT EXISTS submission_screenshot BYTEA,
  ADD COLUMN IF NOT EXISTS result_screenshot     BYTEA;
