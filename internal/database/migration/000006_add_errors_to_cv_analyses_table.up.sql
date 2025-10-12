ALTER TABLE cv_analyses
ADD COLUMN errors jsonb DEFAULT '{}'::jsonb;
