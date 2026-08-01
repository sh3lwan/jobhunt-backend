-- name: UpsertLearnedAnswer :exec
INSERT INTO learned_answers (user_id, label, value, updated_at)
VALUES ($1, $2, $3, now())
ON CONFLICT (user_id, label) DO UPDATE
  SET value = EXCLUDED.value, updated_at = now();

-- name: ListLearnedAnswers :many
SELECT label, value FROM learned_answers WHERE user_id = $1;
