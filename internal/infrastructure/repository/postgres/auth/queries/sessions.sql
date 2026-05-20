-- name: CreateRefreshSession :one
INSERT INTO refresh_sessions (user_id,
                              token_hash,
                              expires_at,
                              absolute_expires_at,
                              ip,
                              user_agent)
VALUES ($1, $2, $3, $4, $5, $6) RETURNING *;


-- name: GetRefreshSessionByTokenHash :one
SELECT *
FROM refresh_sessions
WHERE token_hash = $1
  AND revoked_at IS NULL
  AND expires_at > now()
  AND absolute_expires_at > now();

-- name: RevokeSessionByTokenHash :exec
UPDATE refresh_sessions
SET revoked_at = now()
WHERE token_hash = $1
  AND revoked_at IS NULL;

-- name: RevokeAllSessionsByUser :execrows
UPDATE refresh_sessions
SET revoked_at = now()
WHERE user_id = $1
  AND revoked_at IS NULL;

-- name: RotateSession :exec
UPDATE refresh_sessions
SET token_hash = $2,
    expires_at = $3
WHERE id = $1
  AND revoked_at IS NULL
  AND expires_at > now()
  AND absolute_expires_at > now();

-- name: ListActiveSessionsByUser :many
SELECT id, user_id, expires_at, created_at, revoked_at, ip, user_agent
FROM refresh_sessions
WHERE user_id = $1
  AND revoked_at IS NULL
  AND expires_at > now()
  AND absolute_expires_at > now()
ORDER BY created_at DESC;
