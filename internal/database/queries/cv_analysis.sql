-- name: CreateCVAnalysis :one
INSERT INTO cv_analyses (file_name, original_name, parsed_text, status)
VALUES ($1, $2, $3, $4) RETURNING *;

-- name: GetCVAnalysis :one
SELECT *
FROM cv_analyses
WHERE id = $1;

-- name: GetAllCVAnalysis :many
SELECT *
FROM cv_analyses
WHERE ($3::text[] IS NULL OR status = ANY($3))
ORDER BY created_at DESC LIMIT $1
OFFSET $2;

-- name: UpdateCVStatus :exec
UPDATE cv_analyses
SET status      = $2,
    parsed_text = $3
WHERE id = $1;

-- name: UpdateCVStructuredJSON :exec
UPDATE cv_analyses
SET structured_json = $2,
    status          = 'analyzed'
WHERE id = $1;

-- name: InsertCVEmbedding :exec
INSERT INTO cv_analyses_embeddings (cv_id, embedding)
VALUES ($1, $2) ON CONFLICT (cv_id) DO
UPDATE SET embedding = EXCLUDED.embedding;

-- name: GetCVEmbedding :one
SELECT embedding
FROM cv_analyses_embeddings
WHERE cv_id = $1;

-- name: GetDistinctSkills :many
SELECT DISTINCT tech::text AS technology
FROM cv_analyses,
LATERAL jsonb_array_elements(structured_json->'technologies') AS tech
WHERE structured_json IS NOT NULL;

