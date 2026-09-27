-- name: CreateUser :one
INSERT INTO users (email, password_hash, name, role)
VALUES (LOWER(sqlc.arg(email)), sqlc.arg(password_hash), sqlc.arg(name), sqlc.arg(role))
RETURNING id, email, password_hash, name, role, status, created_at, updated_at;

-- name: GetUserByEmail :one
SELECT id, email, password_hash, name, role, status, created_at, updated_at
FROM users
WHERE LOWER(email) = LOWER($1);

-- name: GetUserByID :one
SELECT id, email, password_hash, name, role, status, created_at, updated_at
FROM users
WHERE id = $1;

-- name: ListUsers :many
SELECT id, email, password_hash, name, role, status, created_at, updated_at
FROM users
ORDER BY created_at DESC, id DESC;

-- name: UpdateUserRole :one
UPDATE users
SET role = $2, updated_at = NOW()
WHERE id = $1
RETURNING id, email, password_hash, name, role, status, created_at, updated_at;

-- name: UpdateUserStatus :one
UPDATE users
SET status = $2, updated_at = NOW()
WHERE id = $1
RETURNING id, email, password_hash, name, role, status, created_at, updated_at;

-- name: CreateSession :one
INSERT INTO sessions (user_id, token_hash, expires_at)
VALUES ($1, $2, $3)
RETURNING id, user_id, token_hash, expires_at, created_at;

-- name: GetSessionByTokenHash :one
SELECT id, user_id, token_hash, expires_at, created_at
FROM sessions
WHERE token_hash = $1 AND expires_at > NOW();

-- name: DeleteSessionByTokenHash :exec
DELETE FROM sessions
WHERE token_hash = $1;

-- name: DeleteExpiredSessions :exec
DELETE FROM sessions
WHERE expires_at <= NOW();

-- name: CreateUserInvitation :one
INSERT INTO user_invitations (email, name, role, token_hash, invited_by, expires_at)
VALUES (
	LOWER(sqlc.arg(email)),
	sqlc.arg(name),
	sqlc.arg(role),
	sqlc.arg(token_hash),
	sqlc.arg(invited_by),
	sqlc.arg(expires_at)
)
RETURNING id, email, name, role, token_hash, invited_by, accepted_by, expires_at, accepted_at, created_at;

-- name: GetUserInvitationByTokenHash :one
SELECT id, email, name, role, token_hash, invited_by, accepted_by, expires_at, accepted_at, created_at
FROM user_invitations
WHERE token_hash = $1 AND accepted_at IS NULL AND expires_at > NOW();

-- name: AcceptUserInvitation :one
UPDATE user_invitations
SET accepted_by = sqlc.arg(accepted_by)::bigint, accepted_at = NOW()
WHERE id = sqlc.arg(id) AND accepted_at IS NULL
RETURNING id, email, name, role, token_hash, invited_by, accepted_by, expires_at, accepted_at, created_at;
