-- name: CreateImportBatch :one
INSERT INTO import_batches (id, source_filename, source_sha256, format, state, expires_at, initiated_by)
VALUES ($1, $2, $3, $4, 'previewing', $5, $6)
RETURNING *;

-- name: GetImportBatchByID :one
SELECT *
FROM import_batches
WHERE id = $1;

-- name: UpdateImportBatchPreview :one
UPDATE import_batches
SET state = $2,
    total_row_count = $3,
    valid_row_count = $4,
    warning_row_count = $5,
    error_row_count = $6,
    revision = revision + 1,
    updated_at = now()
WHERE id = $1 AND revision = $7 AND state = 'previewing'
RETURNING *;

-- name: CreateImportRow :one
INSERT INTO import_rows (
    id, batch_id, row_number, import_key, draft, issues, selected, warnings_acknowledged, duplicate_resolution
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: ListImportRows :many
SELECT *
FROM import_rows
WHERE batch_id = $1
ORDER BY row_number;

-- name: UpdateImportRowSelection :one
UPDATE import_rows
SET selected = $3, warnings_acknowledged = $4, duplicate_resolution = $5
WHERE id = $1 AND batch_id = $2
RETURNING *;

-- name: ListSelectedImportRowsForUpdate :many
SELECT *
FROM import_rows
WHERE batch_id = $1 AND selected
ORDER BY row_number
FOR UPDATE;

-- name: MarkImportRowCommitted :one
UPDATE import_rows
SET committed_project_id = $3
WHERE id = $1 AND batch_id = $2
RETURNING *;

-- name: MarkImportBatchCommitted :one
UPDATE import_batches
SET state = 'committed', result = $3, committed_at = now(), revision = revision + 1, updated_at = now()
WHERE id = $1 AND revision = $2 AND state = 'committing'
RETURNING *;

-- name: ExpireImportBatches :execrows
UPDATE import_batches
SET state = 'expired', revision = revision + 1, updated_at = now()
WHERE id IN (
    SELECT id
    FROM import_batches
    WHERE state IN ('previewing', 'ready', 'failed') AND expires_at <= now()
    ORDER BY expires_at
    LIMIT $1
    FOR UPDATE SKIP LOCKED
);
