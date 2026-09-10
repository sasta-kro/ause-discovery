-- +goose Up
-- StatementDescription:
-- Per-Artifact storage backend marker (AD-011). Existing rows keep serving
-- from the local filesystem byte store. New rows record the backend that
-- holds their bytes so mixed-backend serving and gradual migration stay safe.
ALTER TABLE artifacts
    ADD COLUMN storage_backend text NOT NULL DEFAULT 'local'
    CHECK (storage_backend IN ('local', 'b2'));

-- +goose Down
-- StatementDescription:
ALTER TABLE artifacts
    DROP COLUMN storage_backend;
