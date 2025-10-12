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

-- name: GetUserByUsername :one
SELECT *
FROM users
WHERE username = $1;
