CREATE TABLE IF NOT EXISTS CV_JOB_MATCHES
(
    cv_id      BIGINT        NOT NULL,
    job_id     BIGINT        NOT NULL,
    percentage NUMERIC(5, 2) NOT NULL,
    created_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (cv_id, job_id)
);
