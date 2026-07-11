-- Per-user target industry + funding-stage preferences, alongside size.
-- Drive the automatic CV-driven crawl. NULL/empty means "all".
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS preferred_industries TEXT[] DEFAULT '{}',
    ADD COLUMN IF NOT EXISTS preferred_stages     TEXT[] DEFAULT '{}';
