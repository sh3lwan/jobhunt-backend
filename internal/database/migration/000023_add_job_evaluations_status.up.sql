ALTER TABLE job_evaluations
    ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'done'
        CHECK (status IN ('requested', 'running', 'done', 'failed')),
    ADD COLUMN IF NOT EXISTS requested_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS error TEXT;

CREATE INDEX IF NOT EXISTS idx_job_evaluations_status ON job_evaluations (status);
