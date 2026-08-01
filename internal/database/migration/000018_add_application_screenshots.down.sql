ALTER TABLE applications
  DROP COLUMN IF EXISTS submission_screenshot,
  DROP COLUMN IF EXISTS result_screenshot;
