-- Tracks when an embedding request was last dispatched for a job, so the
-- reconciler doesn't re-send work that is already in the parser's queue —
-- including across jobhunter restarts (the previous in-memory cooldown was
-- lost on restart, causing duplicate LLM work).
ALTER TABLE jobs
    ADD COLUMN IF NOT EXISTS embedding_requested_at TIMESTAMPTZ;
