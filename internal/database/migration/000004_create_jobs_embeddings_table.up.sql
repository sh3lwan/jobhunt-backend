CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE IF NOT EXISTS jobs_embeddings (
  id         BIGSERIAL PRIMARY KEY,
  job_id     BIGINT NOT NULL UNIQUE
             REFERENCES jobs(id) ON DELETE CASCADE,
  embedding  vector(768) NOT NULL,  -- change dim if needed
  created_at TIMESTAMPTZ DEFAULT now(),
  updated_at TIMESTAMPTZ DEFAULT now()
);
