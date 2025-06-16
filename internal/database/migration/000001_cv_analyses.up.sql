CREATE TABLE cv_analyses
(
    id              BIGINT PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
    original_name   TEXT      NOT NULL,
    file_name        TEXT      NOT NULL,
    parsed_text     TEXT,
    structured_json JSONB,
    status          TEXT      NOT NULL CHECK (status IN ('uploaded', 'parsed', 'analyzed', 'error')),
    created_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);