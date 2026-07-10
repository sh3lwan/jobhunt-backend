-- LLM reranker output for top matches: a recruiter-rubric score plus the
-- structured assessment (explanation, strengths, gaps, seniority fit) shown
-- in the dashboard. Jobs without a rerank fall back to the embedding score.
ALTER TABLE cv_job_matches
    ADD COLUMN IF NOT EXISTS rerank_score   NUMERIC(5, 2),
    ADD COLUMN IF NOT EXISTS rerank_details JSONB,
    ADD COLUMN IF NOT EXISTS reranked_at    TIMESTAMPTZ;
