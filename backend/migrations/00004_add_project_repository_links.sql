-- +goose Up
-- StatementDescription:
-- Project Repository Links (Increment 15). A repository link is Project
-- metadata identifying one source-control repository: it owns no bytes, so
-- it never enters artifacts, storage providers, quotas, or Project File
-- lists. The Project Content manifest is authoritative for a declared set;
-- replacement deletes and reinserts rows in one transaction and the audit
-- record preserves the mutation history.
CREATE TABLE project_repository_links (
    id uuid PRIMARY KEY,
    project_id uuid NOT NULL REFERENCES projects(id),
    url text NOT NULL CHECK (btrim(url) <> '' AND length(url) <= 2048),
    is_primary boolean NOT NULL,
    availability text NOT NULL CHECK (availability IN ('accessible', 'not_accessible', 'unverified')),
    checked_at timestamptz NOT NULL,
    sort_order integer NOT NULL CHECK (sort_order >= 0),
    actor_id uuid REFERENCES application_users(id),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (project_id, url),
    UNIQUE (project_id, sort_order)
);

CREATE UNIQUE INDEX project_repository_links_one_primary_per_project
    ON project_repository_links (project_id)
    WHERE is_primary;

-- +goose Down
-- StatementDescription:
DROP TABLE IF EXISTS project_repository_links;
