-- jobscrapper creates this table at runtime; declaring it here makes it part
-- of the canonical schema so sqlc can generate queries against it and fresh
-- databases get it without booting the scraper first. Identical DDL to
-- jobscrapper/src/db/schema.ts (CREATE IF NOT EXISTS on both sides).
CREATE TABLE IF NOT EXISTS job_fetch_tasks (
    id SERIAL PRIMARY KEY,
    task_id TEXT UNIQUE NOT NULL,
    platform TEXT NOT NULL,
    skills TEXT[],
    location TEXT,
    status TEXT NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
