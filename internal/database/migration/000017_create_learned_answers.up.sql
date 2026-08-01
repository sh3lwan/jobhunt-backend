-- Per-user remembered field answers. When a user approves an application, the
-- reusable short field values they confirmed are stored here so future fills
-- reuse them (matched by label) instead of re-deriving via the LLM. Tailored
-- free-text (cover letters, "why this company") is intentionally excluded.
CREATE TABLE IF NOT EXISTS learned_answers (
    user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    label      TEXT   NOT NULL,
    value      TEXT   NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, label)
);
