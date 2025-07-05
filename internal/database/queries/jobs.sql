-- name: CreateJob :exec
INSERT INTO jobs (source_id, source, title, company, logo, url, description, tags, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (source, source_id) DO NOTHING;
