-- name: CreateUser :one
INSERT INTO users (email, login, password_hash)
VALUES ($1, $2, $3) RETURNING users.id;

-- name: GetUserByEmail :one
SELECT *
FROM users
WHERE email = $1;

-- name: GetUserByID :one
SELECT *
FROM users
WHERE id = $1;
