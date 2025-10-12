CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE IF NOT EXISTS jobs_embeddings (
    id         BIGSERIAL PRIMARY KEY,
    job_id     BIGINT NOT NULL UNIQUE
    REFERENCES jobs(id) ON DELETE CASCADE,
    canonical_text TEXT,
    skills_text TEXT,
    responsibilities_text TEXT,
    canonical_text_embeddings VECTOR(768),
    skills_text_embeddings VECTOR(768),
    responsibilities_text_embeddings VECTOR(768),
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now()
);
