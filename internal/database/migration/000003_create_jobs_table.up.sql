CREATE TABLE jobs (
    id SERIAL PRIMARY KEY,
    source_id TEXT,          -- e.g., Remotive's job ID
    source TEXT NOT NULL,    -- e.g., 'remotive'
    title TEXT,
    company TEXT,
    logo TEXT,
    location TEXT,
    url TEXT,
    tags TEXT[],
    description TEXT,
    publish_at TIMESTAMPTZ DEFAULT NOW(),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE (source, source_id)
);
