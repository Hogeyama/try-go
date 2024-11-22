-- name: CreateUser :one
INSERT INTO users (
    username,
    password_hash,
    role
) VALUES (
    $1, $2, $3
) RETURNING *;

-- name: GetUserByUsername :one
SELECT * FROM users
WHERE username = $1;

-- name: GetUserById :one
SELECT * FROM users
WHERE id = $1;

-- name: CreateSession :one
INSERT INTO sessions (
    user_id,
    session_token,
    expires_at
) VALUES (
    $1, $2, $3
) RETURNING *;

-- name: GetSessionByToken :one
SELECT *
FROM sessions s
WHERE s.session_token = $1 AND s.expires_at > NOW();

-- name: DeleteSession :exec
DELETE FROM sessions
WHERE session_token = $1;

-- name: DeleteExpiredSessions :exec
DELETE FROM sessions
WHERE expires_at <= NOW();
