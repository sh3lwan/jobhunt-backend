-- HNSW indexes for cosine similarity search on all embedding columns.
-- Without these, every <=> comparison is a sequential scan.
CREATE INDEX IF NOT EXISTS cv_embeddings_canonical_hnsw_idx
    ON cv_embeddings USING hnsw (canonical_text_embeddings vector_cosine_ops);

CREATE INDEX IF NOT EXISTS cv_embeddings_skills_hnsw_idx
    ON cv_embeddings USING hnsw (skills_text_embeddings vector_cosine_ops);

CREATE INDEX IF NOT EXISTS cv_embeddings_responsibilities_hnsw_idx
    ON cv_embeddings USING hnsw (responsibilities_text_embeddings vector_cosine_ops);

CREATE INDEX IF NOT EXISTS jobs_embeddings_canonical_hnsw_idx
    ON jobs_embeddings USING hnsw (canonical_text_embeddings vector_cosine_ops);

CREATE INDEX IF NOT EXISTS jobs_embeddings_skills_hnsw_idx
    ON jobs_embeddings USING hnsw (skills_text_embeddings vector_cosine_ops);

CREATE INDEX IF NOT EXISTS jobs_embeddings_responsibilities_hnsw_idx
    ON jobs_embeddings USING hnsw (responsibilities_text_embeddings vector_cosine_ops);
