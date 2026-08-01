CREATE TABLE IF NOT EXISTS job_evaluations (
    cv_id BIGINT NOT NULL REFERENCES cv_analyses (id) ON DELETE CASCADE,
    job_id INTEGER NOT NULL REFERENCES jobs (id) ON DELETE CASCADE,
    score NUMERIC(3, 1),
    final_decision TEXT,
    machine_summary JSONB,
    report_path TEXT,
    tailored_cv_path TEXT,
    evaluator TEXT NOT NULL DEFAULT 'career-ops',
    model TEXT,
    evaluated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (cv_id, job_id)
);

CREATE INDEX IF NOT EXISTS idx_job_evaluations_score ON job_evaluations (score DESC);

ALTER TABLE applications ADD COLUMN IF NOT EXISTS tailored_cv_path TEXT;
