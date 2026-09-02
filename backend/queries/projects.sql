-- name: CreateProject :one
INSERT INTO projects (
    id, reference_code, title, abstract, academic_year, semester,
    program_version_id, major_version_id, course_version_id, extra_metadata
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: GetProjectByID :one
SELECT *
FROM projects
WHERE id = $1;

-- name: ListPublishedProjects :many
SELECT *
FROM projects
WHERE status = 'published'
ORDER BY academic_year DESC NULLS LAST, updated_at DESC, id
LIMIT $1 OFFSET $2;

-- name: ListProjectsForAdministration :many
SELECT *
FROM projects
WHERE sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status')::text
ORDER BY updated_at DESC, id
LIMIT $1 OFFSET $2;

-- name: UpdateProjectDraft :one
UPDATE projects
SET reference_code = $2,
    title = $3,
    abstract = $4,
    academic_year = $5,
    semester = $6,
    program_version_id = $7,
    major_version_id = $8,
    course_version_id = $9,
    extra_metadata = $10,
    revision = revision + 1,
    updated_at = now()
WHERE id = $1 AND revision = $11 AND status = 'draft'
RETURNING *;

-- name: PublishProject :one
UPDATE projects
SET status = 'published',
    published_at = now(),
    revision = revision + 1,
    updated_at = now()
WHERE id = $1 AND revision = $2 AND status = 'draft'
RETURNING *;

-- name: DeleteProject :one
UPDATE projects
SET status = 'deleted',
    pre_delete_status = status,
    deleted_at = now(),
    revision = revision + 1,
    updated_at = now()
WHERE id = $1 AND revision = $2 AND status IN ('draft', 'published')
RETURNING *;

-- name: RestoreProject :one
UPDATE projects
SET status = pre_delete_status,
    pre_delete_status = NULL,
    deleted_at = NULL,
    revision = revision + 1,
    updated_at = now()
WHERE id = $1 AND revision = $2 AND status = 'deleted'
RETURNING *;

-- name: DeleteProjectTitleAliases :exec
DELETE FROM project_title_aliases
WHERE project_id = $1;

-- name: CreateProjectTitleAlias :one
INSERT INTO project_title_aliases (project_id, position, value, normalized_value)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: DeleteProjectParticipations :exec
DELETE FROM project_participations
WHERE project_id = $1;

-- name: CreateProjectParticipation :one
INSERT INTO project_participations (project_id, person_id, role, position)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: DeleteProjectTaxonomyValues :exec
DELETE FROM project_taxonomy_values
WHERE project_id = $1;

-- name: CreateProjectTaxonomyValue :one
INSERT INTO project_taxonomy_values (project_id, taxonomy_value_id, dimension, position)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: CreatePerson :one
INSERT INTO people (id, display_name, normalized_name, student_id, staff_id)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: UpdatePerson :one
UPDATE people
SET display_name = $2,
    normalized_name = $3,
    student_id = $4,
    staff_id = $5,
    revision = revision + 1,
    updated_at = now()
WHERE id = $1 AND revision = $6
RETURNING *;

-- name: FindPeopleByNormalizedName :many
SELECT *
FROM people
WHERE normalized_name = $1
ORDER BY id;

-- name: GetPersonByStudentID :one
SELECT *
FROM people
WHERE student_id = $1;
