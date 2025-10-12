-- name: CreateCVAnalysis :one
INSERT INTO cv_analyses (file_name, original_name, parsed_text, status, user_id)
VALUES ($1, $2, $3, $4, $5) RETURNING *;

-- name: GetCVAnalysis :one
SELECT *
FROM cv_analyses
WHERE id = $1;

-- name: GetAllCVAnalysis :many
SELECT *
FROM cv_analyses
WHERE ($3::text[] IS NULL OR status = ANY($3))
AND user_id = $4
ORDER BY created_at DESC LIMIT $1
OFFSET $2;

-- name: UpdateCVParsedText :exec
UPDATE cv_analyses
SET parsed_text = $2,
    status      = 'parsed',
    errors      = NULL
WHERE id = $1;

-- name: UpdateCVStructuredJSON :exec
UPDATE cv_analyses
SET structured_json = $2,
    status          = 'analyzed',
    errors          = NULL
WHERE id = $1;

-- name: UpdateCVStatus :exec
UPDATE cv_analyses
SET status = $2
WHERE id = $1;

-- name: UpdateCVErrors :exec
UPDATE cv_analyses
SET errors = $2,
    status = 'error'
WHERE id = $1;

-- name: InsertCVEmbedding :exec
INSERT INTO cv_embeddings (cv_id, canonical_text, skills_text, responsibilities_text, canonical_text_embeddings, skills_text_embeddings, responsibilities_text_embeddings)
VALUES ($1, $2, $3, $4, $5, $6, $7) ON CONFLICT (cv_id) DO
UPDATE SET
    canonical_text = EXCLUDED.canonical_text,
    skills_text = EXCLUDED.skills_text,
    responsibilities_text = EXCLUDED.responsibilities_text,
    canonical_text_embeddings = EXCLUDED.canonical_text_embeddings,
    skills_text_embeddings = EXCLUDED.skills_text_embeddings,
    responsibilities_text_embeddings = EXCLUDED.responsibilities_text_embeddings,
    updated_at = now();

-- name: GetCVEmbedding :one
SELECT *
FROM cv_embeddings
WHERE cv_id = $1;

-- name: GetDistinctSkills :many
SELECT DISTINCT tech::text AS technology
FROM cv_analyses,
LATERAL jsonb_array_elements(structured_json->'technologies') AS tech
WHERE structured_json IS NOT NULL;


-- name: GetDistinctSkillsForCV :many
SELECT DISTINCT tech::text AS technology
FROM cv_analyses,
LATERAL jsonb_array_elements(structured_json->'technologies') AS tech
WHERE structured_json IS NOT NULL
AND id = $1;

