-- +goose Up
-- StatementDescription:
-- Project Logo metadata (Increment 12). A Project Logo is Project-owned
-- presentation metadata stored in the configured object store, not an
-- Artifact: it never enters artifacts, quotas, Artifact facets, or Project
-- File lists. Old rows and bytes remain after replacement or removal; only
-- the row with status 'active' is public, enforced by a partial unique
-- index per Project.
CREATE TABLE project_logos (
    id uuid PRIMARY KEY,
    project_id uuid NOT NULL REFERENCES projects(id),
    storage_key text NOT NULL UNIQUE CHECK (btrim(storage_key) <> ''),
    storage_backend text NOT NULL CHECK (storage_backend IN ('local', 'b2')),
    mime_type text NOT NULL CHECK (mime_type = 'image/png'),
    extension text NOT NULL CHECK (extension = 'png'),
    byte_count bigint NOT NULL CHECK (byte_count > 0),
    sha256 bytea NOT NULL CHECK (length(sha256) = 32),
    status text NOT NULL CHECK (status IN ('active', 'deleted')),
    revision bigint NOT NULL DEFAULT 1 CHECK (revision >= 1),
    actor_id uuid REFERENCES application_users(id),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);

CREATE UNIQUE INDEX project_logos_one_active_per_project
    ON project_logos (project_id)
    WHERE status = 'active';

-- +goose Down
-- StatementDescription:
DROP TABLE IF EXISTS project_logos;
