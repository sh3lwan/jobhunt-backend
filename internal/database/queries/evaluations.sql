-- Deep-evaluation queue: career-ops A-G evaluations requested from the UI and
-- processed by the jobbridge watch worker (claude -p). Statuses:
-- requested -> running -> done | failed.

-- name: RequestJobEvaluation :execrows
INSERT INTO job_evaluations (cv_id, job_id, status, requested_at)
VALUES ($1, $2, 'requested', now())
ON CONFLICT (cv_id, job_id) DO UPDATE
SET status = 'requested', requested_at = now(), error = NULL
WHERE job_evaluations.status IN ('done', 'failed');

-- name: GetJobEvaluation :one
SELECT cv_id, job_id, score, final_decision, machine_summary, report_path,
       tailored_cv_path, evaluator, model, status, requested_at, error, evaluated_at
FROM job_evaluations
WHERE cv_id = $1 AND job_id = $2;
