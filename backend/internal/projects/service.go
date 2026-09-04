package projects

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"ause-discovery.local/backend/generated"
	"ause-discovery.local/backend/internal/audit"
	"ause-discovery.local/backend/internal/platform/database"
	"ause-discovery.local/backend/internal/platform/identity"
	"ause-discovery.local/backend/internal/platform/pagecursor"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound            = errors.New("project not found")
	ErrRevisionConflict    = errors.New("revision conflict")
	ErrInvalidConfirmation = errors.New("delete confirmation does not match")
)

type Service struct{ Pool *pgxpool.Pool }
type Input struct {
	ReferenceCode, Title, Abstract                    *string
	AcademicYear                                      *int
	Semester                                          *string
	ProgramVersionID, MajorVersionID, CourseVersionID *uuid.UUID
	ExtensionMetadata                                 map[string]any
	TitleAliases                                      []string
	Participations                                    []Participation
	TaxonomyValues                                    []TaxonomyValue
}
type Participation struct {
	PersonID  uuid.UUID
	Role      string
	SortOrder int
}
type TaxonomyValue struct {
	ID        uuid.UUID
	SortOrder int
}
type Project struct {
	ID                                                uuid.UUID
	ReferenceCode, Title, Abstract                    *string
	AcademicYear                                      *int
	Semester                                          *string
	ProgramVersionID, MajorVersionID, CourseVersionID *uuid.UUID
	Status                                            string
	Revision                                          int64
	PublishedAt, DeletedAt                            *time.Time
	CreatedAt, UpdatedAt                              time.Time
	ExtensionMetadata                                 map[string]any
	TitleAliases                                      []string
	Participations                                    []Participation
	TaxonomyValues                                    []TaxonomyValue
}

func (service Service) Create(ctx context.Context, actorID uuid.UUID, input Input) (Project, error) {
	var result Project
	err := database.InTransaction(ctx, service.Pool, func(transaction pgx.Tx) error {
		var createErr error
		result, createErr = service.CreateInTransaction(ctx, transaction, actorID, input)
		return createErr
	})
	return result, err
}

func (service Service) CreateInTransaction(ctx context.Context, transaction pgx.Tx, actorID uuid.UUID, input Input) (Project, error) {
	if err := validateInput(input); err != nil {
		return Project{}, err
	}
	projectID := uuid.Must(uuid.NewV7())
	if err := validateReferences(ctx, transaction, input); err != nil {
		return Project{}, err
	}
	metadata, err := json.Marshal(metadataOrEmpty(input.ExtensionMetadata))
	if err != nil {
		return Project{}, err
	}
	record, err := generated.New(transaction).CreateProject(ctx, generated.CreateProjectParams{ID: identity.UUID(projectID), ReferenceCode: optionalText(input.ReferenceCode), Title: optionalText(input.Title), Abstract: optionalText(input.Abstract), AcademicYear: optionalInt(input.AcademicYear), Semester: optionalText(input.Semester), ProgramVersionID: optionalUUID(input.ProgramVersionID), MajorVersionID: optionalUUID(input.MajorVersionID), CourseVersionID: optionalUUID(input.CourseVersionID), ExtraMetadata: metadata})
	if err != nil {
		return Project{}, err
	}
	if err := replaceChildren(ctx, transaction, projectID, input); err != nil {
		return Project{}, err
	}
	if _, err := generated.New(transaction).UpsertProjectSearchSync(ctx, generated.UpsertProjectSearchSyncParams{ProjectID: identity.UUID(projectID), DesiredRevision: record.Revision, DesiredAction: "remove"}); err != nil {
		return Project{}, err
	}
	if err := audit.AppendTx(ctx, transaction, audit.Event{ActorID: actorID, EventType: "project.created", TargetType: "project", TargetID: projectID}); err != nil {
		return Project{}, err
	}
	result := fromRecord(record)
	result.TitleAliases = input.TitleAliases
	result.Participations = input.Participations
	result.TaxonomyValues = input.TaxonomyValues
	return result, nil
}

func (service Service) Replace(ctx context.Context, actorID, projectID uuid.UUID, expectedRevision int64, input Input) (Project, error) {
	if err := validateInput(input); err != nil {
		return Project{}, err
	}
	var result Project
	err := database.InTransaction(ctx, service.Pool, func(transaction pgx.Tx) error {
		if err := validateReferences(ctx, transaction, input); err != nil {
			return err
		}
		metadata, err := json.Marshal(metadataOrEmpty(input.ExtensionMetadata))
		if err != nil {
			return err
		}
		var record generated.Project
		err = transaction.QueryRow(ctx, `UPDATE projects SET reference_code=$2,title=$3,abstract=$4,academic_year=$5,semester=$6,program_version_id=$7,major_version_id=$8,course_version_id=$9,extra_metadata=$10,revision=revision+1,updated_at=now() WHERE id=$1 AND revision=$11 AND status IN ('draft','published') RETURNING id,reference_code,title,abstract,academic_year,semester,program_version_id,major_version_id,course_version_id,status,pre_delete_status,extra_metadata,revision,published_at,deleted_at,created_at,updated_at`, identity.UUID(projectID), optionalText(input.ReferenceCode), optionalText(input.Title), optionalText(input.Abstract), optionalInt(input.AcademicYear), optionalText(input.Semester), optionalUUID(input.ProgramVersionID), optionalUUID(input.MajorVersionID), optionalUUID(input.CourseVersionID), metadata, expectedRevision).Scan(&record.ID, &record.ReferenceCode, &record.Title, &record.Abstract, &record.AcademicYear, &record.Semester, &record.ProgramVersionID, &record.MajorVersionID, &record.CourseVersionID, &record.Status, &record.PreDeleteStatus, &record.ExtraMetadata, &record.Revision, &record.PublishedAt, &record.DeletedAt, &record.CreatedAt, &record.UpdatedAt)
		if errors.Is(err, pgx.ErrNoRows) {
			return classifyRevision(ctx, transaction, projectID)
		}
		if err != nil {
			return err
		}
		if err := replaceChildren(ctx, transaction, projectID, input); err != nil {
			return err
		}
		action := "upsert"
		if record.Status != "published" {
			action = "remove"
		}
		if _, err := generated.New(transaction).UpsertProjectSearchSync(ctx, generated.UpsertProjectSearchSyncParams{ProjectID: identity.UUID(projectID), DesiredRevision: record.Revision, DesiredAction: action}); err != nil {
			return err
		}
		if err := audit.AppendTx(ctx, transaction, audit.Event{ActorID: actorID, EventType: "project.updated", TargetType: "project", TargetID: projectID}); err != nil {
			return err
		}
		result = fromRecord(record)
		result.TitleAliases = input.TitleAliases
		result.Participations = input.Participations
		result.TaxonomyValues = input.TaxonomyValues
		return nil
	})
	return result, err
}

func (service Service) Publish(ctx context.Context, actorID, projectID uuid.UUID, expectedRevision int64) (Project, error) {
	return service.transition(ctx, actorID, projectID, expectedRevision, "publish")
}

func (service Service) PublishInTransaction(ctx context.Context, transaction pgx.Tx, actorID, projectID uuid.UUID, expectedRevision int64) (Project, error) {
	return service.transitionInTransaction(ctx, transaction, actorID, projectID, expectedRevision, "publish")
}
func (service Service) Restore(ctx context.Context, actorID, projectID uuid.UUID, expectedRevision int64) (Project, error) {
	return service.transition(ctx, actorID, projectID, expectedRevision, "restore")
}
func (service Service) Delete(ctx context.Context, actorID, projectID uuid.UUID, expectedRevision int64, confirmation string) (Project, error) {
	current, err := service.Get(ctx, projectID, false)
	if err != nil {
		return Project{}, err
	}
	if !matchesConfirmation(current, confirmation) {
		return Project{}, ErrInvalidConfirmation
	}
	var result Project
	err = database.InTransaction(ctx, service.Pool, func(transaction pgx.Tx) error {
		record, err := generated.New(transaction).DeleteProject(ctx, generated.DeleteProjectParams{ID: identity.UUID(projectID), Revision: expectedRevision})
		if errors.Is(err, pgx.ErrNoRows) {
			return classifyRevision(ctx, transaction, projectID)
		}
		if err != nil {
			return err
		}
		if _, err := generated.New(transaction).UpsertProjectSearchSync(ctx, generated.UpsertProjectSearchSyncParams{ProjectID: identity.UUID(projectID), DesiredRevision: record.Revision, DesiredAction: "remove"}); err != nil {
			return err
		}
		if err := audit.AppendTx(ctx, transaction, audit.Event{ActorID: actorID, EventType: "project.deleted", TargetType: "project", TargetID: projectID}); err != nil {
			return err
		}
		result = fromRecord(record)
		return nil
	})
	return result, err
}
func (service Service) transition(ctx context.Context, actorID, projectID uuid.UUID, expectedRevision int64, kind string) (Project, error) {
	var result Project
	err := database.InTransaction(ctx, service.Pool, func(transaction pgx.Tx) error {
		var transitionErr error
		result, transitionErr = service.transitionInTransaction(ctx, transaction, actorID, projectID, expectedRevision, kind)
		return transitionErr
	})
	return result, err
}

func (service Service) transitionInTransaction(ctx context.Context, transaction pgx.Tx, actorID, projectID uuid.UUID, expectedRevision int64, kind string) (Project, error) {
	q := generated.New(transaction)
	var record generated.Project
	var err error
	if kind == "publish" {
		record, err = q.PublishProject(ctx, generated.PublishProjectParams{ID: identity.UUID(projectID), Revision: expectedRevision})
	} else {
		record, err = q.RestoreProject(ctx, generated.RestoreProjectParams{ID: identity.UUID(projectID), Revision: expectedRevision})
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return Project{}, classifyRevision(ctx, transaction, projectID)
	}
	if err != nil {
		return Project{}, err
	}
	action := "upsert"
	if record.Status != "published" {
		action = "remove"
	}
	if _, err = q.UpsertProjectSearchSync(ctx, generated.UpsertProjectSearchSyncParams{ProjectID: identity.UUID(projectID), DesiredRevision: record.Revision, DesiredAction: action}); err != nil {
		return Project{}, err
	}
	if err := audit.AppendTx(ctx, transaction, audit.Event{ActorID: actorID, EventType: "project." + kind, TargetType: "project", TargetID: projectID}); err != nil {
		return Project{}, err
	}
	return fromRecord(record), nil
}

func (service Service) Get(ctx context.Context, projectID uuid.UUID, publicOnly bool) (Project, error) {
	var record generated.Project
	statement := `SELECT id,reference_code,title,abstract,academic_year,semester,program_version_id,major_version_id,course_version_id,status,pre_delete_status,extra_metadata,revision,published_at,deleted_at,created_at,updated_at FROM projects WHERE id=$1`
	if publicOnly {
		statement += " AND status='published'"
	}
	err := service.Pool.QueryRow(ctx, statement, identity.UUID(projectID)).Scan(&record.ID, &record.ReferenceCode, &record.Title, &record.Abstract, &record.AcademicYear, &record.Semester, &record.ProgramVersionID, &record.MajorVersionID, &record.CourseVersionID, &record.Status, &record.PreDeleteStatus, &record.ExtraMetadata, &record.Revision, &record.PublishedAt, &record.DeletedAt, &record.CreatedAt, &record.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Project{}, ErrNotFound
	}
	if err != nil {
		return Project{}, err
	}
	result := fromRecord(record)
	if err := loadChildren(ctx, service.Pool, &result); err != nil {
		return Project{}, err
	}
	return result, nil
}

type ListPage struct {
	Items      []Project
	Limit      int
	NextCursor *string
}

// List returns one keyset-ordered Project page. Ordering is updated_at then id
// descending, the text filter matches title or reference code case-insensitively
// after trimming, and the opaque cursor is the last returned row position.
func (service Service) List(ctx context.Context, status *string, query string, limit int, cursor string) (ListPage, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	var position pagecursor.Cursor
	if cursor != "" {
		decoded, err := pagecursor.Decode(cursor)
		if err != nil {
			return ListPage{}, err
		}
		if _, err := time.Parse(time.RFC3339Nano, decoded.SortValue); err != nil {
			return ListPage{}, pagecursor.ErrInvalid
		}
		position = decoded
	}
	trimmedQuery := strings.TrimSpace(query)
	queryFilter := nullableString(&trimmedQuery)
	if trimmedQuery == "" {
		queryFilter = pgtype.Text{}
	}
	rows, err := service.Pool.Query(ctx, `SELECT id,reference_code,title,abstract,academic_year,semester,program_version_id,major_version_id,course_version_id,status,pre_delete_status,extra_metadata,revision,published_at,deleted_at,created_at,updated_at FROM projects
WHERE ($1::text IS NULL OR status=$1)
  AND ($2::text IS NULL OR coalesce(title,'') ILIKE '%' || $2::text || '%' OR coalesce(reference_code,'') ILIKE '%' || $2::text || '%')
  AND ($4::timestamptz IS NULL OR (updated_at, id) < ($4::timestamptz, $5::uuid))
ORDER BY updated_at DESC, id DESC LIMIT $3`, nullableString(status), queryFilter, limit+1, pgtype.Timestamptz{Time: cursorTime(position), Valid: cursor != ""}, identity.NullableUUID(position.ID))
	if err != nil {
		return ListPage{}, err
	}
	defer rows.Close()
	items := []Project{}
	for rows.Next() {
		var record generated.Project
		if err := rows.Scan(&record.ID, &record.ReferenceCode, &record.Title, &record.Abstract, &record.AcademicYear, &record.Semester, &record.ProgramVersionID, &record.MajorVersionID, &record.CourseVersionID, &record.Status, &record.PreDeleteStatus, &record.ExtraMetadata, &record.Revision, &record.PublishedAt, &record.DeletedAt, &record.CreatedAt, &record.UpdatedAt); err != nil {
			return ListPage{}, err
		}
		item := fromRecord(record)
		if err := loadChildren(ctx, service.Pool, &item); err != nil {
			return ListPage{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return ListPage{}, err
	}
	page := ListPage{Items: items, Limit: limit}
	if len(items) > limit {
		page.Items = items[:limit]
		last := page.Items[limit-1]
		next, err := pagecursor.EncodeTime(last.UpdatedAt, last.ID)
		if err != nil {
			return ListPage{}, err
		}
		page.NextCursor = &next
	}
	return page, nil
}

func cursorTime(position pagecursor.Cursor) time.Time {
	if position.SortValue == "" {
		return time.Time{}
	}
	parsed, _ := time.Parse(time.RFC3339Nano, position.SortValue)
	return parsed
}

func replaceChildren(ctx context.Context, transaction pgx.Tx, projectID uuid.UUID, input Input) error {
	q := generated.New(transaction)
	if err := q.DeleteProjectTitleAliases(ctx, identity.UUID(projectID)); err != nil {
		return err
	}
	if err := q.DeleteProjectParticipations(ctx, identity.UUID(projectID)); err != nil {
		return err
	}
	if err := q.DeleteProjectTaxonomyValues(ctx, identity.UUID(projectID)); err != nil {
		return err
	}
	for position, value := range input.TitleAliases {
		if _, err := q.CreateProjectTitleAlias(ctx, generated.CreateProjectTitleAliasParams{ProjectID: identity.UUID(projectID), Position: int32(position), Value: strings.TrimSpace(value), NormalizedValue: normalize(value)}); err != nil {
			return err
		}
	}
	for _, value := range input.Participations {
		if _, err := q.CreateProjectParticipation(ctx, generated.CreateProjectParticipationParams{ProjectID: identity.UUID(projectID), PersonID: identity.UUID(value.PersonID), Role: value.Role, Position: int32(value.SortOrder)}); err != nil {
			return err
		}
	}
	for _, value := range input.TaxonomyValues {
		var dimension string
		if err := transaction.QueryRow(ctx, "SELECT dimension FROM taxonomy_values WHERE id=$1 AND retired_at IS NULL", identity.UUID(value.ID)).Scan(&dimension); err != nil {
			return fmt.Errorf("taxonomy value: %w", err)
		}
		if _, err := q.CreateProjectTaxonomyValue(ctx, generated.CreateProjectTaxonomyValueParams{ProjectID: identity.UUID(projectID), TaxonomyValueID: identity.UUID(value.ID), Dimension: dimension, Position: int32(value.SortOrder)}); err != nil {
			return err
		}
	}
	return nil
}
func validateReferences(ctx context.Context, tx pgx.Tx, input Input) error {
	for _, value := range input.Participations {
		var exists bool
		if err := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM people WHERE id=$1)", identity.UUID(value.PersonID)).Scan(&exists); err != nil || !exists {
			return errors.New("participation person does not exist")
		}
	}
	return nil
}
func validateInput(input Input) error {
	for _, alias := range input.TitleAliases {
		if strings.TrimSpace(alias) == "" {
			return errors.New("title aliases cannot be empty")
		}
	}
	seenAliases := map[string]bool{}
	for _, alias := range input.TitleAliases {
		key := normalize(alias)
		if seenAliases[key] {
			return errors.New("title aliases must be unique")
		}
		seenAliases[key] = true
	}
	seenParticipation := map[string]bool{}
	for _, item := range input.Participations {
		if item.PersonID == uuid.Nil || !validRole(item.Role) || item.SortOrder < 0 {
			return errors.New("invalid participation")
		}
		key := item.PersonID.String() + ":" + item.Role
		if seenParticipation[key] {
			return errors.New("duplicate participation")
		}
		seenParticipation[key] = true
	}
	return nil
}
func validRole(value string) bool {
	return value == "student" || value == "advisor" || value == "co_advisor" || value == "committee_member"
}
func classifyRevision(ctx context.Context, tx pgx.Tx, projectID uuid.UUID) error {
	var exists bool
	if err := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM projects WHERE id=$1)", identity.UUID(projectID)).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return ErrNotFound
	}
	return ErrRevisionConflict
}
func matchesConfirmation(project Project, confirmation string) bool {
	candidate := strings.TrimSpace(confirmation)
	if project.ReferenceCode != nil && candidate == *project.ReferenceCode {
		return true
	}
	return project.Title != nil && candidate == *project.Title
}
func metadataOrEmpty(value map[string]any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	return value
}
func normalize(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(value)), " "))
}
func optionalText(value *string) pgtype.Text {
	if value == nil || strings.TrimSpace(*value) == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: strings.TrimSpace(*value), Valid: true}
}
func optionalInt(value *int) pgtype.Int4 {
	if value == nil {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: int32(*value), Valid: true}
}
func optionalUUID(value *uuid.UUID) pgtype.UUID {
	if value == nil {
		return pgtype.UUID{}
	}
	return identity.UUID(*value)
}
func nullableString(value *string) pgtype.Text {
	if value == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *value, Valid: true}
}
func fromRecord(record generated.Project) Project {
	result := Project{ID: identity.UUIDValue(record.ID), Status: record.Status, Revision: record.Revision, ExtensionMetadata: map[string]any{}}
	if record.ReferenceCode.Valid {
		v := record.ReferenceCode.String
		result.ReferenceCode = &v
	}
	if record.Title.Valid {
		v := record.Title.String
		result.Title = &v
	}
	if record.Abstract.Valid {
		v := record.Abstract.String
		result.Abstract = &v
	}
	if record.AcademicYear.Valid {
		v := int(record.AcademicYear.Int32)
		result.AcademicYear = &v
	}
	if record.Semester.Valid {
		v := record.Semester.String
		result.Semester = &v
	}
	if record.ProgramVersionID.Valid {
		v := identity.UUIDValue(record.ProgramVersionID)
		result.ProgramVersionID = &v
	}
	if record.MajorVersionID.Valid {
		v := identity.UUIDValue(record.MajorVersionID)
		result.MajorVersionID = &v
	}
	if record.CourseVersionID.Valid {
		v := identity.UUIDValue(record.CourseVersionID)
		result.CourseVersionID = &v
	}
	if record.PublishedAt.Valid {
		v := record.PublishedAt.Time
		result.PublishedAt = &v
	}
	if record.DeletedAt.Valid {
		v := record.DeletedAt.Time
		result.DeletedAt = &v
	}
	if record.CreatedAt.Valid {
		result.CreatedAt = record.CreatedAt.Time
	}
	if record.UpdatedAt.Valid {
		result.UpdatedAt = record.UpdatedAt.Time
	}
	_ = json.Unmarshal(record.ExtraMetadata, &result.ExtensionMetadata)
	return result
}
func loadChildren(ctx context.Context, pool *pgxpool.Pool, project *Project) error {
	rows, err := pool.Query(ctx, "SELECT value FROM project_title_aliases WHERE project_id=$1 ORDER BY position", identity.UUID(project.ID))
	if err != nil {
		return err
	}
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			rows.Close()
			return err
		}
		project.TitleAliases = append(project.TitleAliases, value)
	}
	rows.Close()
	rows, err = pool.Query(ctx, "SELECT person_id,role,position FROM project_participations WHERE project_id=$1 ORDER BY role,position", identity.UUID(project.ID))
	if err != nil {
		return err
	}
	for rows.Next() {
		var id pgtype.UUID
		var item Participation
		if err := rows.Scan(&id, &item.Role, &item.SortOrder); err != nil {
			rows.Close()
			return err
		}
		item.PersonID = identity.UUIDValue(id)
		project.Participations = append(project.Participations, item)
	}
	rows.Close()
	rows, err = pool.Query(ctx, "SELECT taxonomy_value_id,position FROM project_taxonomy_values WHERE project_id=$1 ORDER BY dimension,position", identity.UUID(project.ID))
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id pgtype.UUID
		var item TaxonomyValue
		if err := rows.Scan(&id, &item.SortOrder); err != nil {
			return err
		}
		item.ID = identity.UUIDValue(id)
		project.TaxonomyValues = append(project.TaxonomyValues, item)
	}
	return rows.Err()
}
