-- name: CreateApplicationUser :one
INSERT INTO application_users (id, username, status)
VALUES ($1, $2, 'active')
RETURNING *;

-- name: GetApplicationUserByID :one
SELECT *
FROM application_users
WHERE id = $1;

-- name: GetApplicationUserByUsername :one
SELECT *
FROM application_users
WHERE username = $1;

-- name: DisableApplicationUser :one
UPDATE application_users
SET status = 'disabled', revision = revision + 1, updated_at = now()
WHERE id = $1 AND revision = $2
RETURNING *;

-- name: CreateLocalCredential :one
INSERT INTO local_credentials (user_id, password_hash)
VALUES ($1, $2)
RETURNING *;

-- name: UpdateLocalCredential :one
UPDATE local_credentials
SET password_hash = $2, updated_at = now()
WHERE user_id = $1
RETURNING *;

-- name: GetCredentialByUsername :one
SELECT application_users.id, application_users.username, application_users.status, local_credentials.password_hash
FROM application_users
JOIN local_credentials ON local_credentials.user_id = application_users.id
WHERE application_users.username = $1;

-- name: CreateSession :one
INSERT INTO sessions (
    id, user_id, token_hash, csrf_token_hash, last_active_at, idle_expires_at, absolute_expires_at
)
VALUES ($1, $2, $3, $4, now(), $5, $6)
RETURNING *;

-- name: GetActiveSessionForUpdate :one
SELECT sessions.*, application_users.status AS user_status
FROM sessions
JOIN application_users ON application_users.id = sessions.user_id
WHERE sessions.token_hash = $1
  AND sessions.revoked_at IS NULL
  AND sessions.idle_expires_at > now()
  AND sessions.absolute_expires_at > now()
FOR UPDATE;

-- name: TouchSession :one
UPDATE sessions
SET last_active_at = now(), idle_expires_at = $2
WHERE id = $1 AND revoked_at IS NULL
RETURNING *;

-- name: RevokeSession :execrows
UPDATE sessions
SET revoked_at = now()
WHERE id = $1 AND revoked_at IS NULL;

-- name: InsertLoginFailure :exec
INSERT INTO login_failures (username, source_ip)
VALUES ($1, $2);

-- name: CountRecentLoginFailures :one
SELECT count(*)
FROM login_failures
WHERE username = $1 AND source_ip = $2 AND attempted_at >= $3;
