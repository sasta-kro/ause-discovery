-- name: UpsertProjectSearchSync :one
INSERT INTO project_search_sync (project_id, desired_revision, desired_action, state, updated_at)
VALUES ($1, $2, $3, 'pending', now())
ON CONFLICT (project_id) DO UPDATE
SET desired_revision = EXCLUDED.desired_revision,
    desired_action = EXCLUDED.desired_action,
    state = 'pending',
    next_attempt_at = NULL,
    lease_owner = NULL,
    lease_expires_at = NULL,
    error_detail = NULL,
    updated_at = now()
RETURNING *;

-- name: ClaimProjectSearchSync :many
WITH claimable AS (
    SELECT project_id
    FROM project_search_sync
    WHERE state IN ('pending', 'failed')
      AND (next_attempt_at IS NULL OR next_attempt_at <= now())
    ORDER BY updated_at, project_id
    LIMIT $1
    FOR UPDATE SKIP LOCKED
)
UPDATE project_search_sync
SET state = 'processing',
    attempt_count = attempt_count + 1,
    lease_owner = $2,
    lease_expires_at = $3,
    last_attempt_at = now(),
    error_detail = NULL,
    updated_at = now()
WHERE project_id IN (SELECT project_id FROM claimable)
RETURNING *;

-- name: MarkProjectSearchSyncSucceeded :one
UPDATE project_search_sync
SET indexed_revision = desired_revision,
    state = 'synced',
    lease_owner = NULL,
    lease_expires_at = NULL,
    error_detail = NULL,
    synchronized_at = now(),
    updated_at = now()
WHERE project_id = $1 AND state = 'processing' AND lease_owner = $2
RETURNING *;

-- name: MarkProjectSearchSyncFailed :one
UPDATE project_search_sync
SET state = 'failed',
    lease_owner = NULL,
    lease_expires_at = NULL,
    next_attempt_at = $3,
    error_detail = $4,
    updated_at = now()
WHERE project_id = $1 AND state = 'processing' AND lease_owner = $2
RETURNING *;

-- name: CreateSearchRebuildOperation :one
INSERT INTO search_rebuild_operations (id, state, physical_index_name, initiated_by)
VALUES ($1, 'pending', $2, $3)
RETURNING *;

-- name: GetSearchRebuildOperation :one
SELECT *
FROM search_rebuild_operations
WHERE id = $1;

-- name: StartSearchRebuildOperation :one
UPDATE search_rebuild_operations
SET state = 'processing', started_at = now(), total_count = $2, updated_at = now()
WHERE id = $1 AND state = 'pending'
RETURNING *;

-- name: AdvanceSearchRebuildOperation :one
UPDATE search_rebuild_operations
SET processed_count = $2, updated_at = now()
WHERE id = $1 AND state = 'processing'
RETURNING *;

-- name: CompleteSearchRebuildOperation :one
UPDATE search_rebuild_operations
SET state = 'completed', processed_count = total_count, completed_at = now(), updated_at = now()
WHERE id = $1 AND state = 'processing'
RETURNING *;

-- name: FailSearchRebuildOperation :one
UPDATE search_rebuild_operations
SET state = 'failed', error_detail = $2, completed_at = now(), updated_at = now()
WHERE id = $1 AND state IN ('pending', 'processing')
RETURNING *;
