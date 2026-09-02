-- name: CreateAuditEvent :one
INSERT INTO audit_events (id, actor_id, event_type, target_type, target_id, metadata)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListAuditEvents :many
SELECT *
FROM audit_events
WHERE (sqlc.narg('actor_id')::uuid IS NULL OR actor_id = sqlc.narg('actor_id')::uuid)
  AND (sqlc.narg('target_type')::text IS NULL OR target_type = sqlc.narg('target_type')::text)
  AND (sqlc.narg('target_id')::uuid IS NULL OR target_id = sqlc.narg('target_id')::uuid)
ORDER BY created_at DESC, id DESC
LIMIT $1 OFFSET $2;
