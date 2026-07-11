ALTER TABLE users
    DROP COLUMN IF EXISTS preferred_industries,
    DROP COLUMN IF EXISTS preferred_stages;
