CREATE TABLE IF NOT EXISTS CV_JOB_MATCHES
(
    cv_id      BIGINT        NOT NULL,
    job_id     BIGINT        NOT NULL,
    percentage NUMERIC(5, 2) NOT NULL,
    created_at TIMESTAMPZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (cv_id, job_id)
);
