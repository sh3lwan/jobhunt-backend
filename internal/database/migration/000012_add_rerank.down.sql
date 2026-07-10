ALTER TABLE cv_job_matches
    DROP COLUMN IF EXISTS rerank_score,
    DROP COLUMN IF EXISTS rerank_details,
    DROP COLUMN IF EXISTS reranked_at;
