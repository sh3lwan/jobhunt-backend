-- name: CreateJob :exec
INSERT INTO jobs (source_id, source, title, company, logo, url, description, tags, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (source, source_id) DO NOTHING;

-- name: GetAllJobs :many
SELECT *
FROM jobs
WHERE ($1::text[] IS NULL OR source = ANY($1))
ORDER BY publish_at DESC
LIMIT $2 OFFSET $3;

-- name: GetJobById :one
SELECT *
FROM jobs
WHERE id = $1;

-- name: InsertJobEmbedding :exec
INSERT INTO jobs_embeddings (job_id, canonical_text, skills_text, responsibilities_text, canonical_text_embeddings, skills_text_embeddings, responsibilities_text_embeddings)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (job_id)
DO UPDATE SET
    canonical_text = EXCLUDED.canonical_text,
    skills_text = EXCLUDED.skills_text,
    responsibilities_text = EXCLUDED.responsibilities_text,
    canonical_text_embeddings = EXCLUDED.canonical_text_embeddings,
    skills_text_embeddings = EXCLUDED.skills_text_embeddings,
    responsibilities_text_embeddings = EXCLUDED.responsibilities_text_embeddings,
    updated_at = now();


-- name: InsertAllMissingCvJobPairs :execrows
INSERT INTO cv_job_matches (cv_id, job_id, percentage)
SELECT
  c.cv_id,
  je.job_id,
  ROUND(((1 - (c.embedding <=> je.embedding)) * 100)::numeric, 2) AS percentage
FROM cv_embeddings AS c
JOIN jobs_embeddings AS je ON TRUE
LEFT JOIN cv_job_matches AS m
    ON m.cv_id = c.cv_id AND m.job_id = je.job_id
WHERE m.cv_id IS NULL;


-- name: GetJobMatchesByCvIds :many
SELECT *
FROM cv_job_matches
WHERE cv_id = ANY(sqlc.arg(cvids)::bigint[])
ORDER BY percentage DESC;

-- name: GetJobMatchesByCvId :many
SELECT *
FROM cv_job_matches
WHERE cv_id = $1;

-- name: GetJobMatchesByJobId :many
SELECT *
FROM cv_job_matches
WHERE job_id = $1;
