-- name: CreateUser :exec
INSERT INTO users (email, username, password)
VALUES ($1, $2, $3);

-- name: GetAllUsers :many
SELECT *
FROM users
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: GetUserById :one
SELECT *
FROM users
WHERE id = $1;

-- name: GetUserByEmail :one
SELECT *
FROM users
WHERE email = $1;

-- name: UpdateUserPassword :exec
UPDATE users
SET password = $2
WHERE id = $1;

-- name: GetUserPreferredSizes :one
SELECT preferred_company_sizes
FROM users
WHERE id = $1;

-- name: UpdateUserPreferredSizes :exec
UPDATE users
SET preferred_company_sizes = $2,
    updated_at = now()
WHERE id = $1;

-- Company-size preference for the owner of a CV — used by the CV-driven
-- crawl dispatch to scope scrapes without a separate lookup round-trip.
-- name: GetPreferredSizesByCvId :one
SELECT u.preferred_company_sizes
FROM cv_analyses c
JOIN users u ON u.id = c.user_id
WHERE c.id = $1;

-- name: GetUserByUsername :one
SELECT *
FROM users
WHERE username = $1;
