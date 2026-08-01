-- Cached form-field detections, keyed by page URL. The applier's detection step
-- is a slow LLM call (~80–100s via OpenCode) that produces the same field list
-- every time for a given form. Caching it lets re-runs, retries, and repeat
-- postings skip detection entirely and go straight to filling.
CREATE TABLE IF NOT EXISTS form_detections (
    url         TEXT PRIMARY KEY,
    fields      JSONB NOT NULL,
    field_count INT NOT NULL,
    detected_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
