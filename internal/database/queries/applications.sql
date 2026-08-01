-- name: QueueApplication :exec
INSERT INTO applications (user_id, cv_id, job_id, status, tailored_cv_path)
VALUES ($1, $2, $3, 'queued',
        (SELECT e.tailored_cv_path FROM job_evaluations AS e
         WHERE e.cv_id = $2 AND e.job_id = $3))
ON CONFLICT (user_id, job_id) DO NOTHING;

-- name: ListApplications :many
SELECT
  a.id, a.status, a.filled_inputs, a.error, a.submitted_at, a.created_at, a.updated_at,
  a.cv_id, a.job_id,
  (a.submission_screenshot IS NOT NULL) AS has_submission_screenshot,
  (a.result_screenshot IS NOT NULL) AS has_result_screenshot,
  j.title, j.company, j.location, j.url, j.source
FROM applications AS a
JOIN jobs AS j ON j.id = a.job_id
WHERE a.user_id = $1
ORDER BY a.updated_at DESC
LIMIT $2 OFFSET $3;

-- name: GetSubmissionScreenshot :one
SELECT submission_screenshot FROM applications WHERE id = $1 AND user_id = $2;

-- name: GetResultScreenshot :one
SELECT result_screenshot FROM applications WHERE id = $1 AND user_id = $2;

-- name: CountAwaitingApproval :one
SELECT COUNT(*)
FROM applications
WHERE user_id = $1 AND status = 'awaiting_approval';

-- name: GetApplication :one
SELECT * FROM applications WHERE id = $1;

-- Approve moves an awaiting_approval application to approved so the worker
-- submits it. Owner-scoped; only valid from awaiting_approval.
-- name: ApproveApplication :execrows
UPDATE applications
SET status = 'approved', updated_at = now()
WHERE id = $1 AND user_id = $2 AND status = 'awaiting_approval';

-- name: DiscardApplication :execrows
UPDATE applications
SET status = 'discarded', updated_at = now()
WHERE id = $1 AND user_id = $2
  AND status IN ('queued','filling','awaiting_approval','failed');

-- UpdateApplicationInputs saves the user's edited field values. Owner-scoped and
-- only while awaiting_approval, so an in-flight or submitted application can't be
-- rewritten. The submit phase replays exactly these values.
-- name: UpdateApplicationInputs :execrows
UPDATE applications
SET filled_inputs = $3, updated_at = now()
WHERE id = $1 AND user_id = $2 AND status = 'awaiting_approval';
