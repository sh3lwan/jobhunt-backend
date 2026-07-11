-- Per-user target company-size preference. Drives the automatic CV-driven
-- crawl (small|midsize|large|enterprise). NULL/empty means all sizes.
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS preferred_company_sizes TEXT[] DEFAULT '{}';
