-- +goose Up
CREATE EXTENSION IF NOT EXISTS btree_gist;

CREATE TABLE programs (
    id uuid PRIMARY KEY,
    key text NOT NULL UNIQUE CHECK (key ~ '^[a-z0-9_]+$'),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE program_versions (
    id uuid PRIMARY KEY,
    program_id uuid NOT NULL REFERENCES programs,
    label text NOT NULL CHECK (btrim(label) <> ''),
    valid_from_year integer NOT NULL,
    valid_to_year integer,
    major_required boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK (valid_to_year IS NULL OR valid_to_year >= valid_from_year),
    EXCLUDE USING gist (program_id WITH =, int4range(valid_from_year, valid_to_year, '[]') WITH &&)
);

CREATE TABLE majors (
    id uuid PRIMARY KEY,
    key text NOT NULL UNIQUE CHECK (key ~ '^[a-z0-9_]+$'),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE major_versions (
    id uuid PRIMARY KEY,
    major_id uuid NOT NULL REFERENCES majors,
    label text NOT NULL CHECK (btrim(label) <> ''),
    valid_from_year integer NOT NULL,
    valid_to_year integer,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK (valid_to_year IS NULL OR valid_to_year >= valid_from_year),
    EXCLUDE USING gist (major_id WITH =, int4range(valid_from_year, valid_to_year, '[]') WITH &&)
);

CREATE TABLE courses (
    id uuid PRIMARY KEY,
    key text NOT NULL UNIQUE CHECK (key ~ '^[a-z0-9_]+$'),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE course_versions (
    id uuid PRIMARY KEY,
    course_id uuid NOT NULL REFERENCES courses,
    label text NOT NULL CHECK (btrim(label) <> ''),
    code text NOT NULL CHECK (btrim(code) <> ''),
    valid_from_year integer NOT NULL,
    valid_to_year integer,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK (valid_to_year IS NULL OR valid_to_year >= valid_from_year),
    EXCLUDE USING gist (course_id WITH =, int4range(valid_from_year, valid_to_year, '[]') WITH &&)
);

CREATE TABLE course_program_versions (
    course_version_id uuid NOT NULL REFERENCES course_versions,
    program_version_id uuid NOT NULL REFERENCES program_versions,
    PRIMARY KEY (course_version_id, program_version_id)
);

CREATE TABLE taxonomy_values (
    id uuid PRIMARY KEY,
    dimension text NOT NULL CHECK (dimension IN ('category', 'platform', 'domain', 'topic', 'technology')),
    key text NOT NULL CHECK (key ~ '^[a-z0-9_]+$'),
    labels jsonb NOT NULL CHECK (jsonb_typeof(labels) = 'object' AND labels ? 'en'),
    description text,
    sort_order integer NOT NULL DEFAULT 0,
    retired_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (dimension, key),
    UNIQUE (id, dimension)
);

CREATE TABLE application_users (
    id uuid PRIMARY KEY,
    username text NOT NULL UNIQUE CHECK (username = lower(btrim(username)) AND username <> ''),
    status text NOT NULL CHECK (status IN ('active', 'disabled')),
    revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE local_credentials (
    user_id uuid PRIMARY KEY REFERENCES application_users,
    password_hash text NOT NULL CHECK (btrim(password_hash) <> ''),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE sessions (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES application_users,
    token_hash bytea NOT NULL UNIQUE CHECK (octet_length(token_hash) > 0),
    csrf_token_hash bytea NOT NULL CHECK (octet_length(csrf_token_hash) > 0),
    last_active_at timestamptz NOT NULL DEFAULT now(),
    idle_expires_at timestamptz NOT NULL,
    absolute_expires_at timestamptz NOT NULL,
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK (absolute_expires_at >= idle_expires_at),
    CHECK (idle_expires_at >= created_at)
);

CREATE TABLE login_failures (
    username text NOT NULL CHECK (username = lower(btrim(username)) AND username <> ''),
    source_ip inet NOT NULL,
    attempted_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (username, source_ip, attempted_at)
);

CREATE TABLE people (
    id uuid PRIMARY KEY,
    display_name text NOT NULL CHECK (btrim(display_name) <> ''),
    normalized_name text NOT NULL CHECK (normalized_name = lower(btrim(normalized_name)) AND normalized_name <> ''),
    student_id text UNIQUE CHECK (student_id IS NULL OR student_id ~ '^[0-9]{7}$'),
    staff_id text UNIQUE CHECK (staff_id IS NULL OR staff_id = lower(btrim(staff_id))),
    revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE projects (
    id uuid PRIMARY KEY,
    reference_code text CHECK (reference_code IS NULL OR btrim(reference_code) <> ''),
    title text CHECK (title IS NULL OR btrim(title) <> ''),
    abstract text CHECK (abstract IS NULL OR btrim(abstract) <> ''),
    academic_year integer CHECK (academic_year IS NULL OR academic_year BETWEEN 1900 AND 9999),
    semester text CHECK (semester IS NULL OR semester IN ('first', 'second', 'summer')),
    program_version_id uuid REFERENCES program_versions,
    major_version_id uuid REFERENCES major_versions,
    course_version_id uuid REFERENCES course_versions,
    status text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published', 'deleted')),
    pre_delete_status text CHECK (pre_delete_status IN ('draft', 'published')),
    extra_metadata jsonb NOT NULL DEFAULT '{}' CHECK (jsonb_typeof(extra_metadata) = 'object'),
    revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0),
    published_at timestamptz,
    deleted_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK (
        (status = 'draft' AND pre_delete_status IS NULL AND published_at IS NULL AND deleted_at IS NULL)
        OR (status = 'published' AND pre_delete_status IS NULL AND published_at IS NOT NULL AND deleted_at IS NULL)
        OR (status = 'deleted' AND pre_delete_status = 'draft' AND published_at IS NULL AND deleted_at IS NOT NULL)
        OR (status = 'deleted' AND pre_delete_status = 'published' AND published_at IS NOT NULL AND deleted_at IS NOT NULL)
    )
);

CREATE TABLE project_title_aliases (
    project_id uuid NOT NULL REFERENCES projects,
    position integer NOT NULL CHECK (position >= 0),
    value text NOT NULL CHECK (btrim(value) <> ''),
    normalized_value text NOT NULL CHECK (normalized_value = lower(btrim(normalized_value)) AND normalized_value <> ''),
    PRIMARY KEY (project_id, position),
    UNIQUE (project_id, normalized_value)
);

CREATE TABLE project_participations (
    project_id uuid NOT NULL REFERENCES projects,
    person_id uuid NOT NULL REFERENCES people,
    role text NOT NULL CHECK (role IN ('student', 'advisor', 'co_advisor', 'committee_member')),
    position integer NOT NULL CHECK (position >= 0),
    PRIMARY KEY (project_id, person_id, role),
    UNIQUE (project_id, role, position)
);

CREATE TABLE project_taxonomy_values (
    project_id uuid NOT NULL REFERENCES projects,
    taxonomy_value_id uuid NOT NULL,
    dimension text NOT NULL CHECK (dimension IN ('category', 'platform', 'domain', 'topic', 'technology')),
    position integer NOT NULL CHECK (position >= 0),
    PRIMARY KEY (project_id, taxonomy_value_id),
    UNIQUE (project_id, dimension, position),
    FOREIGN KEY (taxonomy_value_id, dimension) REFERENCES taxonomy_values (id, dimension)
);

CREATE TABLE artifacts (
    id uuid PRIMARY KEY,
    project_id uuid NOT NULL REFERENCES projects,
    type text NOT NULL CHECK (btrim(type) <> ''),
    display_name text NOT NULL CHECK (btrim(display_name) <> ''),
    original_filename text NOT NULL CHECK (btrim(original_filename) <> ''),
    storage_key text NOT NULL UNIQUE CHECK (btrim(storage_key) <> ''),
    mime_type text NOT NULL CHECK (btrim(mime_type) <> ''),
    extension text NOT NULL CHECK (extension ~ '^[a-z0-9]+$'),
    byte_count bigint NOT NULL CHECK (byte_count > 0),
    sha256 bytea NOT NULL CHECK (octet_length(sha256) = 32),
    status text NOT NULL CHECK (status IN ('active', 'deleted')),
    revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0),
    actor_id uuid REFERENCES application_users,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz,
    CHECK ((status = 'active' AND deleted_at IS NULL) OR (status = 'deleted' AND deleted_at IS NOT NULL))
);

CREATE TABLE project_search_sync (
    project_id uuid PRIMARY KEY REFERENCES projects,
    desired_revision bigint NOT NULL CHECK (desired_revision > 0),
    indexed_revision bigint CHECK (indexed_revision IS NULL OR indexed_revision > 0),
    desired_action text NOT NULL CHECK (desired_action IN ('upsert', 'remove')),
    state text NOT NULL CHECK (state IN ('pending', 'processing', 'synced', 'failed')),
    attempt_count integer NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
    next_attempt_at timestamptz,
    lease_owner text,
    lease_expires_at timestamptz,
    error_detail text CHECK (error_detail IS NULL OR char_length(error_detail) <= 4096),
    last_attempt_at timestamptz,
    synchronized_at timestamptz,
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK ((state = 'processing') = (lease_owner IS NOT NULL AND lease_expires_at IS NOT NULL)),
    CHECK (state <> 'synced' OR (indexed_revision = desired_revision AND synchronized_at IS NOT NULL)),
    CHECK (state <> 'failed' OR error_detail IS NOT NULL)
);

CREATE TABLE search_rebuild_operations (
    id uuid PRIMARY KEY,
    state text NOT NULL CHECK (state IN ('pending', 'processing', 'completed', 'failed')),
    physical_index_name text NOT NULL UNIQUE CHECK (btrim(physical_index_name) <> ''),
    total_count integer NOT NULL DEFAULT 0 CHECK (total_count >= 0),
    processed_count integer NOT NULL DEFAULT 0 CHECK (processed_count >= 0 AND processed_count <= total_count),
    failed_count integer NOT NULL DEFAULT 0 CHECK (failed_count >= 0 AND failed_count <= total_count),
    initiated_by uuid REFERENCES application_users,
    started_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    completed_at timestamptz,
    error_detail text CHECK (error_detail IS NULL OR char_length(error_detail) <= 4096),
    CHECK (state <> 'completed' OR completed_at IS NOT NULL),
    CHECK (state <> 'failed' OR error_detail IS NOT NULL)
);

CREATE UNIQUE INDEX search_rebuild_one_active_idx ON search_rebuild_operations ((1)) WHERE state IN ('pending', 'processing');

CREATE TABLE import_batches (
    id uuid PRIMARY KEY,
    source_filename text NOT NULL CHECK (btrim(source_filename) <> ''),
    source_sha256 bytea NOT NULL CHECK (octet_length(source_sha256) = 32),
    format text NOT NULL CHECK (format IN ('csv', 'xlsx')),
    state text NOT NULL CHECK (state IN ('previewing', 'ready', 'committing', 'committed', 'failed', 'expired')),
    total_row_count integer NOT NULL DEFAULT 0 CHECK (total_row_count >= 0),
    valid_row_count integer NOT NULL DEFAULT 0 CHECK (valid_row_count >= 0 AND valid_row_count <= total_row_count),
    warning_row_count integer NOT NULL DEFAULT 0 CHECK (warning_row_count >= 0 AND warning_row_count <= total_row_count),
    error_row_count integer NOT NULL DEFAULT 0 CHECK (error_row_count >= 0 AND error_row_count <= total_row_count),
    expires_at timestamptz NOT NULL,
    revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0),
    initiated_by uuid NOT NULL REFERENCES application_users,
    result jsonb CHECK (result IS NULL OR jsonb_typeof(result) = 'object'),
    committed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK (state <> 'committed' OR (result IS NOT NULL AND committed_at IS NOT NULL))
);

CREATE TABLE import_rows (
    id uuid PRIMARY KEY,
    batch_id uuid NOT NULL REFERENCES import_batches,
    row_number integer NOT NULL CHECK (row_number > 0),
    import_key text NOT NULL CHECK (btrim(import_key) <> ''),
    draft jsonb NOT NULL CHECK (jsonb_typeof(draft) = 'object'),
    issues jsonb NOT NULL DEFAULT '[]' CHECK (jsonb_typeof(issues) = 'array'),
    selected boolean NOT NULL DEFAULT false,
    warnings_acknowledged boolean NOT NULL DEFAULT false,
    duplicate_resolution jsonb CHECK (duplicate_resolution IS NULL OR jsonb_typeof(duplicate_resolution) = 'object'),
    committed_project_id uuid REFERENCES projects,
    UNIQUE (batch_id, row_number),
    UNIQUE (batch_id, import_key)
);

CREATE TABLE audit_events (
    id uuid PRIMARY KEY,
    actor_id uuid REFERENCES application_users,
    event_type text NOT NULL CHECK (btrim(event_type) <> ''),
    target_type text NOT NULL CHECK (btrim(target_type) <> ''),
    target_id uuid,
    metadata jsonb NOT NULL DEFAULT '{}' CHECK (jsonb_typeof(metadata) = 'object'),
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX program_versions_program_idx ON program_versions (program_id);
CREATE INDEX major_versions_major_idx ON major_versions (major_id);
CREATE INDEX course_versions_course_idx ON course_versions (course_id);
CREATE INDEX course_program_versions_program_idx ON course_program_versions (program_version_id);
CREATE INDEX taxonomy_values_active_dimension_idx ON taxonomy_values (dimension, sort_order, key) WHERE retired_at IS NULL;
CREATE INDEX sessions_user_active_idx ON sessions (user_id, last_active_at DESC) WHERE revoked_at IS NULL;
CREATE INDEX sessions_expiration_idx ON sessions (absolute_expires_at) WHERE revoked_at IS NULL;
CREATE INDEX login_failures_recent_idx ON login_failures (username, source_ip, attempted_at DESC);
CREATE INDEX people_normalized_name_idx ON people (normalized_name);
CREATE INDEX projects_reference_code_idx ON projects (reference_code) WHERE reference_code IS NOT NULL;
CREATE INDEX projects_public_academic_idx ON projects (academic_year, semester, program_version_id, major_version_id, course_version_id) WHERE status = 'published';
CREATE INDEX projects_status_updated_idx ON projects (status, updated_at DESC);
CREATE INDEX projects_program_version_idx ON projects (program_version_id) WHERE program_version_id IS NOT NULL;
CREATE INDEX projects_major_version_idx ON projects (major_version_id) WHERE major_version_id IS NOT NULL;
CREATE INDEX projects_course_version_idx ON projects (course_version_id) WHERE course_version_id IS NOT NULL;
CREATE INDEX project_participations_person_idx ON project_participations (person_id, role, project_id);
CREATE INDEX project_taxonomy_values_taxonomy_idx ON project_taxonomy_values (taxonomy_value_id, project_id);
CREATE INDEX artifacts_project_active_idx ON artifacts (project_id, type) WHERE status = 'active';
CREATE INDEX project_search_sync_pending_idx ON project_search_sync (next_attempt_at, updated_at) WHERE state IN ('pending', 'failed');
CREATE INDEX search_rebuild_operations_state_idx ON search_rebuild_operations (state, created_at DESC);
CREATE INDEX search_rebuild_operations_initiated_by_idx ON search_rebuild_operations (initiated_by) WHERE initiated_by IS NOT NULL;
CREATE INDEX import_batches_expiration_idx ON import_batches (expires_at) WHERE state IN ('previewing', 'ready', 'failed', 'expired');
CREATE INDEX import_batches_initiated_by_idx ON import_batches (initiated_by, created_at DESC);
CREATE INDEX import_rows_batch_selected_idx ON import_rows (batch_id, row_number) WHERE selected;
CREATE INDEX import_rows_committed_project_idx ON import_rows (committed_project_id) WHERE committed_project_id IS NOT NULL;
CREATE INDEX audit_events_target_created_idx ON audit_events (target_type, target_id, created_at DESC);
CREATE INDEX audit_events_actor_created_idx ON audit_events (actor_id, created_at DESC) WHERE actor_id IS NOT NULL;

-- +goose StatementBegin
CREATE FUNCTION assert_published_project_complete(project_uuid uuid)
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
    project_record projects%ROWTYPE;
BEGIN
    SELECT * INTO project_record FROM projects WHERE id = project_uuid;
    IF NOT FOUND OR project_record.status <> 'published' THEN
        RETURN;
    END IF;

    IF project_record.title IS NULL OR btrim(project_record.title) = ''
        OR project_record.abstract IS NULL OR btrim(project_record.abstract) = ''
        OR project_record.academic_year IS NULL
        OR project_record.semester IS NULL OR btrim(project_record.semester) = ''
        OR project_record.program_version_id IS NULL
        OR project_record.course_version_id IS NULL THEN
        RAISE EXCEPTION 'published project % is missing required core metadata', project_uuid;
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM course_program_versions
        WHERE course_version_id = project_record.course_version_id
          AND program_version_id = project_record.program_version_id
    ) THEN
        RAISE EXCEPTION 'published project % has an unsupported course and program combination', project_uuid;
    END IF;

    IF EXISTS (
        SELECT 1 FROM program_versions
        WHERE id = project_record.program_version_id AND major_required
    ) AND project_record.major_version_id IS NULL THEN
        RAISE EXCEPTION 'published project % requires a major', project_uuid;
    END IF;

    IF NOT EXISTS (SELECT 1 FROM project_participations WHERE project_id = project_uuid AND role = 'student')
        OR NOT EXISTS (SELECT 1 FROM project_participations WHERE project_id = project_uuid AND role = 'advisor') THEN
        RAISE EXCEPTION 'published project % requires a student and advisor', project_uuid;
    END IF;

    IF NOT EXISTS (SELECT 1 FROM project_taxonomy_values WHERE project_id = project_uuid AND dimension = 'category')
        OR NOT EXISTS (SELECT 1 FROM project_taxonomy_values WHERE project_id = project_uuid AND dimension = 'platform') THEN
        RAISE EXCEPTION 'published project % requires a category and platform', project_uuid;
    END IF;
END;
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION enforce_published_project_from_project()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    PERFORM assert_published_project_complete(NEW.id);
    RETURN NULL;
END;
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION enforce_published_project_from_child()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    PERFORM assert_published_project_complete(COALESCE(NEW.project_id, OLD.project_id));
    RETURN NULL;
END;
$$;
-- +goose StatementEnd

CREATE CONSTRAINT TRIGGER projects_published_complete_trigger
AFTER INSERT OR UPDATE ON projects DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW EXECUTE FUNCTION enforce_published_project_from_project();

CREATE CONSTRAINT TRIGGER project_participations_published_complete_trigger
AFTER INSERT OR UPDATE OR DELETE ON project_participations DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW EXECUTE FUNCTION enforce_published_project_from_child();

CREATE CONSTRAINT TRIGGER project_taxonomy_values_published_complete_trigger
AFTER INSERT OR UPDATE OR DELETE ON project_taxonomy_values DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW EXECUTE FUNCTION enforce_published_project_from_child();

-- +goose Down
-- +goose StatementBegin
DO $$
BEGIN
    RAISE EXCEPTION 'AUSE Discovery migrations are forward-only';
END;
$$;
-- +goose StatementEnd
