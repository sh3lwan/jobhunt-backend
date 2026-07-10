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


-- name: GetJobsMissingEmbeddings :many
SELECT j.*
FROM jobs AS j
LEFT JOIN jobs_embeddings AS je ON je.job_id = j.id
WHERE je.job_id IS NULL
  AND (j.embedding_requested_at IS NULL
       OR j.embedding_requested_at < now() - interval '30 minutes')
ORDER BY j.created_at DESC
LIMIT $1;

-- name: MarkJobsEmbeddingRequested :exec
UPDATE jobs
SET embedding_requested_at = now()
WHERE id = ANY(sqlc.arg(job_ids)::int[]);

-- Weighted cosine similarity across the three embedding components, with the
-- per-component scores stored so the UI can explain the total.
-- Zero vectors (inserted when the parser produced no embedding) would make
-- <=> return NaN, so each component falls back to 0 in that case.
-- A domain gate multiplies the final score: canonical_text on both sides
-- begins with "DOMAIN_IDENTITY: <DOMAIN>", and a cross-domain pair (e.g. a
-- nursing job against a software CV) is penalized to 40%; unknown domains get
-- a mild 85% so parse hiccups don't zero out real candidates.
-- Raw cosine similarities cluster in a narrow ~0.50-0.85 band, which makes
-- every job look like a 60-80% match. Each component is calibrated through a
-- linear stretch — (sim - 0.50) / 0.35, clamped to [0,1] — so scores use the
-- full range: excellent fits read 85%+, weak fits drop below 40%.
-- name: InsertAllMissingCvJobPairs :execrows
INSERT INTO cv_job_matches (cv_id, job_id, percentage, canonical_pct, skills_pct, responsibilities_pct, domain_multiplier)
SELECT
  cal.cv_id,
  cal.job_id,
  ROUND(((0.5 * cal.canonical_cal + 0.3 * cal.skills_cal + 0.2 * cal.resp_cal) * cal.domain_mult * 100)::numeric, 2),
  ROUND((cal.canonical_cal * 100)::numeric, 2),
  ROUND((cal.skills_cal * 100)::numeric, 2),
  ROUND((cal.resp_cal * 100)::numeric, 2),
  cal.domain_mult
FROM (
  SELECT
    s.cv_id,
    s.job_id,
    s.domain_mult,
    GREATEST(0, LEAST(1, (s.canonical_sim - 0.50) / 0.35)) AS canonical_cal,
    GREATEST(0, LEAST(1, (s.skills_sim - 0.50) / 0.35)) AS skills_cal,
    GREATEST(0, LEAST(1, (s.resp_sim - 0.50) / 0.35)) AS resp_cal
  FROM (
  SELECT
    c.cv_id,
    je.job_id,
    COALESCE(
      CASE WHEN vector_norm(c.canonical_text_embeddings) = 0
             OR vector_norm(je.canonical_text_embeddings) = 0
           THEN 0
           ELSE 1 - (c.canonical_text_embeddings <=> je.canonical_text_embeddings)
      END, 0) AS canonical_sim,
    COALESCE(
      CASE WHEN vector_norm(c.skills_text_embeddings) = 0
             OR vector_norm(je.skills_text_embeddings) = 0
           THEN 0
           ELSE 1 - (c.skills_text_embeddings <=> je.skills_text_embeddings)
      END, 0) AS skills_sim,
    COALESCE(
      CASE WHEN vector_norm(c.responsibilities_text_embeddings) = 0
             OR vector_norm(je.responsibilities_text_embeddings) = 0
           THEN 0
           ELSE 1 - (c.responsibilities_text_embeddings <=> je.responsibilities_text_embeddings)
      END, 0) AS resp_sim,
    CASE
      WHEN cvd.domain = '' OR jd.domain = '' THEN 0.85
      WHEN cvd.domain = jd.domain THEN 1.0
      ELSE 0.40
    END::numeric(3,2) AS domain_mult
  FROM cv_embeddings AS c
  JOIN jobs_embeddings AS je ON TRUE
  CROSS JOIN LATERAL (
    SELECT CASE
      WHEN upper(c.canonical_text) LIKE 'DOMAIN_IDENTITY:%'
      THEN upper(trim(split_part(split_part(c.canonical_text, E'\n', 1), ':', 2)))
      ELSE ''
    END AS domain
  ) AS cvd
  CROSS JOIN LATERAL (
    SELECT CASE
      WHEN upper(je.canonical_text) LIKE 'DOMAIN_IDENTITY:%'
      THEN upper(trim(split_part(split_part(je.canonical_text, E'\n', 1), ':', 2)))
      ELSE ''
    END AS domain
  ) AS jd
  LEFT JOIN cv_job_matches AS m
      ON m.cv_id = c.cv_id AND m.job_id = je.job_id
  WHERE m.cv_id IS NULL
  ) AS s
) AS cal;

-- name: GetMatchedJobs :many
SELECT
  j.id, j.source_id, j.source, j.title, j.company, j.logo, j.location,
  j.url, j.tags, j.description, j.publish_at, j.created_at,
  m.percentage,
  m.canonical_pct,
  m.skills_pct,
  m.responsibilities_pct,
  m.domain_multiplier,
  m.rerank_score,
  m.rerank_details,
  (je.job_id IS NOT NULL)::bool AS embedded
FROM jobs AS j
LEFT JOIN cv_job_matches AS m ON m.job_id = j.id AND m.cv_id = sqlc.arg(cv_id)
LEFT JOIN jobs_embeddings AS je ON je.job_id = j.id
WHERE (sqlc.narg(sources)::text[] IS NULL OR j.source = ANY(sqlc.narg(sources)::text[]))
  AND (sqlc.narg(min_percentage)::numeric IS NULL
       OR COALESCE(m.rerank_score, m.percentage) >= sqlc.narg(min_percentage)::numeric)
  AND (sqlc.narg(search)::text IS NULL
       OR j.title ILIKE '%' || sqlc.narg(search)::text || '%'
       OR j.company ILIKE '%' || sqlc.narg(search)::text || '%'
       OR j.description ILIKE '%' || sqlc.narg(search)::text || '%')
ORDER BY COALESCE(m.rerank_score, m.percentage) DESC NULLS LAST, j.publish_at DESC
LIMIT sqlc.arg(max_results) OFFSET sqlc.arg(result_offset);

-- name: CountMatchedJobs :one
SELECT COUNT(*)
FROM jobs AS j
LEFT JOIN cv_job_matches AS m ON m.job_id = j.id AND m.cv_id = sqlc.arg(cv_id)
WHERE (sqlc.narg(sources)::text[] IS NULL OR j.source = ANY(sqlc.narg(sources)::text[]))
  AND (sqlc.narg(min_percentage)::numeric IS NULL
       OR COALESCE(m.rerank_score, m.percentage) >= sqlc.narg(min_percentage)::numeric)
  AND (sqlc.narg(search)::text IS NULL
       OR j.title ILIKE '%' || sqlc.narg(search)::text || '%'
       OR j.company ILIKE '%' || sqlc.narg(search)::text || '%'
       OR j.description ILIKE '%' || sqlc.narg(search)::text || '%');

-- Candidates for LLM reranking: strongest un-reranked matches first. The 35%
-- floor skips domain-gated noise — no point spending LLM time scoring jobs
-- the gate already buried.
-- name: GetMatchesNeedingRerank :many
SELECT
  m.cv_id,
  m.job_id,
  m.percentage,
  j.title,
  j.company,
  j.location,
  j.tags,
  j.description,
  c.canonical_text AS cv_canonical,
  c.skills_text    AS cv_skills
FROM cv_job_matches AS m
JOIN jobs AS j ON j.id = m.job_id
JOIN cv_embeddings AS c ON c.cv_id = m.cv_id
WHERE m.rerank_score IS NULL
  AND m.percentage >= 35
ORDER BY m.percentage DESC
LIMIT $1;

-- name: UpdateMatchRerank :exec
UPDATE cv_job_matches
SET rerank_score   = $3,
    rerank_details = $4,
    reranked_at    = now(),
    updated_at     = now()
WHERE cv_id = $1 AND job_id = $2;

-- name: GetLatestAnalyzedCVId :one
SELECT id
FROM cv_analyses
WHERE status = 'analyzed' AND user_id = $1
ORDER BY created_at DESC
LIMIT 1;

-- name: GetPipelineStats :one
SELECT
  (SELECT COUNT(*) FROM jobs)                                        AS total_jobs,
  (SELECT COUNT(*) FROM jobs_embeddings)                             AS embedded_jobs,
  (SELECT COUNT(*) FROM cv_job_matches)                              AS total_matches,
  (SELECT COUNT(*) FROM cv_job_matches WHERE rerank_score IS NOT NULL) AS reranked_matches,
  (SELECT COUNT(*) FROM cv_analyses)                                 AS total_cvs,
  (SELECT COUNT(*) FROM cv_analyses WHERE status = 'analyzed')       AS analyzed_cvs,
  (SELECT COUNT(*) FROM jobs WHERE created_at > now() - interval '24 hours') AS jobs_last_24h;

-- name: GetJobsBySource :many
SELECT
  j.source,
  COUNT(*)                                    AS total,
  COUNT(je.job_id)                            AS embedded,
  MAX(j.created_at)::timestamptz              AS last_collected
FROM jobs AS j
LEFT JOIN jobs_embeddings AS je ON je.job_id = j.id
GROUP BY j.source
ORDER BY total DESC;

-- name: GetRecentScrapeTasks :many
SELECT task_id, platform, skills, location, status, created_at, updated_at
FROM job_fetch_tasks
ORDER BY created_at DESC
LIMIT $1;


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
