CREATE
EXTENSION IF NOT EXISTS vector;

CREATE TABLE cv_analyses_embeddings
(
    cv_id      BIGINT PRIMARY KEY REFERENCES cv_analyses (id) ON DELETE CASCADE,
    embedding  vector(768), -- or 1536 if you're using OpenAI/Ada
    created_at TIMESTAMP DEFAULT now()
);