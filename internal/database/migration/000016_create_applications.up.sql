-- Auto-apply queue. Each row is one job the user chose to apply to. The
-- jobapplier worker fills the form (→ awaiting_approval), the user approves
-- (→ approved), then the worker submits (→ submitted). Human-in-the-loop:
-- nothing is submitted without an explicit approval.
CREATE TABLE IF NOT EXISTS applications (
    id            BIGINT PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
    user_id       BIGINT NOT NULL REFERENCES users(id),
    cv_id         BIGINT REFERENCES cv_analyses(id),
    job_id        BIGINT NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    status        TEXT   NOT NULL DEFAULT 'queued'
                  CHECK (status IN ('queued','filling','awaiting_approval','approved','submitting','submitted','failed','discarded')),
    filled_inputs JSONB,          -- [{label, value, type, skipped}]
    error         TEXT,
    submitted_at  TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, job_id)      -- don't queue the same job twice
);

CREATE INDEX IF NOT EXISTS applications_status_idx ON applications (status);
