-- name: CreateUser :one
INSERT INTO users (id, email, name, role)
VALUES (sqlc.arg(id), LOWER(sqlc.arg(email)), sqlc.arg(name), sqlc.arg(role))
RETURNING id, email, name, role, status, created_at, updated_at;

-- name: GetUserByEmail :one
SELECT id, email, name, role, status, created_at, updated_at
FROM users
WHERE LOWER(email) = LOWER($1);

-- name: GetUserByID :one
SELECT id, email, name, role, status, created_at, updated_at
FROM users
WHERE id = $1;

-- name: ListUsers :many
SELECT id, email, name, role, status, created_at, updated_at
FROM users
ORDER BY created_at DESC, id DESC;

-- name: UpdateUserRole :one
UPDATE users
SET role = $2, updated_at = NOW()
WHERE id = $1
RETURNING id, email, name, role, status, created_at, updated_at;

-- name: UpdateUserStatus :one
UPDATE users
SET status = $2, updated_at = NOW()
WHERE id = $1
RETURNING id, email, name, role, status, created_at, updated_at;
