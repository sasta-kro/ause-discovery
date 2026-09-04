package projects

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"ause-discovery.local/backend/internal/platform/pagecursor"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

func TestProjectLifecycleMaintainsSearchAndAuditState(t *testing.T) {
	databaseURL := os.Getenv("AUSE_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("AUSE_TEST_DATABASE_URL is required for PostgreSQL integration tests")
	}

	ctx := context.Background()
	pool := createProjectTestDatabase(t, ctx, databaseURL)
	seedProjectReferences(t, ctx, pool)

	service := Service{Pool: pool}
	actorID := uuid.MustParse("018f0000-0000-7000-8000-000000000201")
	studentID := uuid.MustParse("018f0000-0000-7000-8000-000000000202")
	advisorID := uuid.MustParse("018f0000-0000-7000-8000-000000000203")
	referenceCode := "REF-001"
	title := "Verified Project"
	abstract := "A complete Project used to verify lifecycle behavior."
	academicYear := 2026
	semester := "first"
	programVersionID := uuid.MustParse("018f0000-0000-7000-8000-000000000011")
	courseVersionID := uuid.MustParse("018f0000-0000-7000-8000-000000000031")
	categoryID := uuid.MustParse("018f0000-0000-7000-8000-000000000101")
	platformID := uuid.MustParse("018f0000-0000-7000-8000-000000000111")

	project, err := service.Create(ctx, actorID, Input{
		ReferenceCode:    &referenceCode,
		Title:            &title,
		Abstract:         &abstract,
		AcademicYear:     &academicYear,
		Semester:         &semester,
		ProgramVersionID: &programVersionID,
		CourseVersionID:  &courseVersionID,
		Participations: []Participation{
			{PersonID: studentID, Role: "student", SortOrder: 0},
			{PersonID: advisorID, Role: "advisor", SortOrder: 0},
		},
		TaxonomyValues: []TaxonomyValue{
			{ID: categoryID, SortOrder: 0},
			{ID: platformID, SortOrder: 0},
		},
	})
	if err != nil {
		t.Fatalf("Create returned an error: %v", err)
	}
	assertSearchState(t, ctx, pool, project.ID, project.Revision, "remove")

	published, err := service.Publish(ctx, actorID, project.ID, project.Revision)
	if err != nil {
		t.Fatalf("Publish returned an error: %v", err)
	}
	if published.Status != "published" {
		t.Fatalf("Publish returned status %q", published.Status)
	}
	assertSearchState(t, ctx, pool, project.ID, published.Revision, "upsert")

	if _, err := service.Replace(ctx, actorID, project.ID, project.Revision, Input{}); !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("Replace returned %v, expected revision conflict", err)
	}
	if _, err := service.Delete(ctx, actorID, project.ID, published.Revision, "incorrect"); !errors.Is(err, ErrInvalidConfirmation) {
		t.Fatalf("Delete returned %v, expected confirmation error", err)
	}

	deleted, err := service.Delete(ctx, actorID, project.ID, published.Revision, referenceCode)
	if err != nil {
		t.Fatalf("Delete returned an error: %v", err)
	}
	assertSearchState(t, ctx, pool, project.ID, deleted.Revision, "remove")
	if _, err := service.Get(ctx, project.ID, true); !errors.Is(err, ErrNotFound) {
		t.Fatalf("public Get returned %v, expected not found", err)
	}

	restored, err := service.Restore(ctx, actorID, project.ID, deleted.Revision)
	if err != nil {
		t.Fatalf("Restore returned an error: %v", err)
	}
	if restored.Status != "published" {
		t.Fatalf("Restore returned status %q", restored.Status)
	}
	assertSearchState(t, ctx, pool, project.ID, restored.Revision, "upsert")

	var auditCount int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM audit_events WHERE target_id = $1", project.ID).Scan(&auditCount); err != nil {
		t.Fatalf("count audit events: %v", err)
	}
	if auditCount != 4 {
		t.Fatalf("found %d audit events, expected 4", auditCount)
	}
}

func assertSearchState(t *testing.T, ctx context.Context, pool *pgxpool.Pool, projectID uuid.UUID, revision int64, action string) {
	t.Helper()
	var storedRevision int64
	var storedAction string
	if err := pool.QueryRow(ctx, "SELECT desired_revision, desired_action FROM project_search_sync WHERE project_id = $1", projectID).Scan(&storedRevision, &storedAction); err != nil {
		t.Fatalf("read search state: %v", err)
	}
	if storedRevision != revision || storedAction != action {
		t.Fatalf("search state was revision %d action %q, expected revision %d action %q", storedRevision, storedAction, revision, action)
	}
}

func createProjectTestDatabase(t *testing.T, ctx context.Context, databaseURL string) *pgxpool.Pool {
	t.Helper()
	adminConfig, err := pgx.ParseConfig(databaseURL)
	if err != nil {
		t.Fatalf("parse database URL: %v", err)
	}
	adminConfig.Database = "postgres"
	adminConnection, err := pgx.ConnectConfig(ctx, adminConfig)
	if err != nil {
		t.Fatalf("connect to PostgreSQL: %v", err)
	}
	databaseName := fmt.Sprintf("ause_projects_%d", time.Now().UnixNano())
	if _, err := adminConnection.Exec(ctx, "CREATE DATABASE "+databaseName); err != nil {
		adminConnection.Close(ctx)
		t.Fatalf("create temporary database: %v", err)
	}
	adminConnection.Close(ctx)

	poolConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatalf("parse pool configuration: %v", err)
	}
	poolConfig.ConnConfig.Database = databaseName
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		t.Fatalf("open temporary database: %v", err)
	}
	if err := goose.SetDialect("postgres"); err != nil {
		pool.Close()
		t.Fatalf("set migration dialect: %v", err)
	}
	sqlDatabase := stdlib.OpenDBFromPool(pool)
	if err := goose.UpContext(ctx, sqlDatabase, "../../migrations"); err != nil {
		sqlDatabase.Close()
		pool.Close()
		t.Fatalf("apply migrations: %v", err)
	}
	sqlDatabase.Close()

	t.Cleanup(func() {
		pool.Close()
		adminConfig.Database = "postgres"
		connection, connectErr := pgx.ConnectConfig(ctx, adminConfig)
		if connectErr != nil {
			return
		}
		defer connection.Close(ctx)
		_, _ = connection.Exec(ctx, "DROP DATABASE IF EXISTS "+databaseName+" WITH (FORCE)")
	})
	return pool
}

func seedProjectReferences(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	statements := []string{
		"INSERT INTO application_users (id, username, status) VALUES ('018f0000-0000-7000-8000-000000000201', 'project-test-admin', 'active')",
		"INSERT INTO people (id, display_name, normalized_name, student_id) VALUES ('018f0000-0000-7000-8000-000000000202', 'Student Example', 'student example', '0123456')",
		"INSERT INTO people (id, display_name, normalized_name, staff_id) VALUES ('018f0000-0000-7000-8000-000000000203', 'Advisor Example', 'advisor example', 'advisor-1')",
		"INSERT INTO programs (id, key) VALUES ('018f0000-0000-7000-8000-000000000001', 'computer_science')",
		"INSERT INTO program_versions (id, program_id, label, valid_from_year) VALUES ('018f0000-0000-7000-8000-000000000011', '018f0000-0000-7000-8000-000000000001', 'Computer Science', 2020)",
		"INSERT INTO courses (id, key) VALUES ('018f0000-0000-7000-8000-000000000021', 'senior_project')",
		"INSERT INTO course_versions (id, course_id, label, code, valid_from_year) VALUES ('018f0000-0000-7000-8000-000000000031', '018f0000-0000-7000-8000-000000000021', 'Senior Project', 'CS499', 2020)",
		"INSERT INTO course_program_versions (course_version_id, program_version_id) VALUES ('018f0000-0000-7000-8000-000000000031', '018f0000-0000-7000-8000-000000000011')",
		"INSERT INTO taxonomy_values (id, dimension, key, labels, sort_order) VALUES ('018f0000-0000-7000-8000-000000000101', 'category', 'software_application', '{\"en\":\"Software / Application\"}', 1)",
		"INSERT INTO taxonomy_values (id, dimension, key, labels, sort_order) VALUES ('018f0000-0000-7000-8000-000000000111', 'platform', 'web', '{\"en\":\"Web\"}', 1)",
	}
	for _, statement := range statements {
		if _, err := pool.Exec(ctx, statement); err != nil {
			t.Fatalf("seed Project reference data: %v", err)
		}
	}
}

func TestProjectListFiltersAndPagesWithCursor(t *testing.T) {
	databaseURL := os.Getenv("AUSE_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("AUSE_TEST_DATABASE_URL is required for PostgreSQL integration tests")
	}

	ctx := context.Background()
	pool := createProjectTestDatabase(t, ctx, databaseURL)
	seedProjectReferences(t, ctx, pool)
	service := Service{Pool: pool}
	sharedTime := time.Date(2026, 9, 4, 10, 0, 0, 0, time.UTC)
	insertProject(t, ctx, pool, "018f0000-0000-7000-8000-000000000301", "Alpha Discovery Engine", "REF-ALPHA", "draft", sharedTime)
	insertProject(t, ctx, pool, "018f0000-0000-7000-8000-000000000302", "Beta Search Interface", "", "published", sharedTime)
	insertProject(t, ctx, pool, "018f0000-0000-7000-8000-000000000303", "Other Work", "REF-GAMMA", "published", sharedTime.Add(-time.Hour))
	insertProject(t, ctx, pool, "018f0000-0000-7000-8000-000000000304", "", "", "deleted", sharedTime.Add(-2*time.Hour))

	first, err := service.List(ctx, nil, "", 2, "")
	if err != nil {
		t.Fatalf("first page returned an error: %v", err)
	}
	if len(first.Items) != 2 || first.NextCursor == nil {
		t.Fatalf("first page held %d items with cursor %v", len(first.Items), first.NextCursor)
	}
	assertListOrder(t, first.Items, "018f0000-0000-7000-8000-000000000302", "018f0000-0000-7000-8000-000000000301")
	second, err := service.List(ctx, nil, "", 2, *first.NextCursor)
	if err != nil {
		t.Fatalf("second page returned an error: %v", err)
	}
	if second.NextCursor != nil {
		t.Fatalf("final page exposed cursor %v", *second.NextCursor)
	}
	assertListOrder(t, second.Items, "018f0000-0000-7000-8000-000000000303", "018f0000-0000-7000-8000-000000000304")
	seen := map[uuid.UUID]bool{}
	for _, item := range append(append([]Project{}, first.Items...), second.Items...) {
		if seen[item.ID] {
			t.Fatalf("Project %s appeared on both pages", item.ID)
		}
		seen[item.ID] = true
	}

	trimmedCaseQuery, err := service.List(ctx, nil, "  discovery  ", 20, "")
	if err != nil {
		t.Fatalf("trimmed case-insensitive query returned an error: %v", err)
	}
	assertListOrder(t, trimmedCaseQuery.Items, "018f0000-0000-7000-8000-000000000301")
	referenceQuery, err := service.List(ctx, nil, "gamma", 20, "")
	if err != nil {
		t.Fatalf("reference-code query returned an error: %v", err)
	}
	assertListOrder(t, referenceQuery.Items, "018f0000-0000-7000-8000-000000000303")
	absentQuery, err := service.List(ctx, nil, "no-such-project", 20, "")
	if err != nil || len(absentQuery.Items) != 0 || absentQuery.NextCursor != nil {
		t.Fatalf("absent query returned %d items, cursor %v, error %v", len(absentQuery.Items), absentQuery.NextCursor, err)
	}

	published := "published"
	statusPage, err := service.List(ctx, &published, "", 20, "")
	if err != nil {
		t.Fatalf("status filter returned an error: %v", err)
	}
	assertListOrder(t, statusPage.Items, "018f0000-0000-7000-8000-000000000302", "018f0000-0000-7000-8000-000000000303")

	if _, err := service.List(ctx, nil, "", 2, "not-a-cursor"); !errors.Is(err, pagecursor.ErrInvalid) {
		t.Fatalf("malformed cursor returned error %v", err)
	}
	nonTimeCursor, err := pagecursor.Encode("not-a-timestamp", uuid.MustParse("018f0000-0000-7000-8000-000000000301"))
	if err != nil {
		t.Fatalf("encode non-time cursor: %v", err)
	}
	if _, err := service.List(ctx, nil, "", 2, nonTimeCursor); !errors.Is(err, pagecursor.ErrInvalid) {
		t.Fatalf("non-timestamp cursor returned error %v", err)
	}
}

func insertProject(t *testing.T, ctx context.Context, pool *pgxpool.Pool, id, title, referenceCode, status string, updatedAt time.Time) {
	t.Helper()
	var titleArg *string
	if title != "" {
		titleArg = &title
	}
	var referenceArg *string
	if referenceCode != "" {
		referenceArg = &referenceCode
	}
	columns := []string{"id", "reference_code", "title", "status", "updated_at"}
	placeholders := []string{"$1", "$2", "$3", "$4", "$5"}
	args := []any{id, referenceArg, titleArg, status, updatedAt}
	switch status {
	case "published":
		columns = append(columns, "abstract", "academic_year", "semester", "program_version_id", "course_version_id", "published_at")
		placeholders = append(placeholders, "$6", "$7", "$8", "$9", "$10", "$11")
		args = append(args, "A complete abstract used by list fixtures.", 2026, "first", "018f0000-0000-7000-8000-000000000011", "018f0000-0000-7000-8000-000000000031", updatedAt)
	case "deleted":
		columns = append(columns, "pre_delete_status", "deleted_at")
		placeholders = append(placeholders, "$6", "$7")
		args = append(args, "draft", updatedAt)
	}
	transaction, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin fixture transaction: %v", err)
	}
	defer transaction.Rollback(ctx)
	statement := `INSERT INTO projects (` + strings.Join(columns, ", ") + `) VALUES (` + strings.Join(placeholders, ", ") + `)`
	if _, err := transaction.Exec(ctx, statement, args...); err != nil {
		t.Fatalf("insert Project: %v", err)
	}
	if status == "published" {
		if _, err := transaction.Exec(ctx, `INSERT INTO project_participations (project_id, person_id, role, position) VALUES ($1, $2, 'student', 0), ($1, $3, 'advisor', 0)`, id, "018f0000-0000-7000-8000-000000000202", "018f0000-0000-7000-8000-000000000203"); err != nil {
			t.Fatalf("insert published Project participation: %v", err)
		}
		if _, err := transaction.Exec(ctx, `INSERT INTO project_taxonomy_values (project_id, taxonomy_value_id, dimension, position) VALUES ($1, $2, 'category', 0), ($1, $3, 'platform', 0)`, id, "018f0000-0000-7000-8000-000000000101", "018f0000-0000-7000-8000-000000000111"); err != nil {
			t.Fatalf("insert published Project taxonomy: %v", err)
		}
	}
	if err := transaction.Commit(ctx); err != nil {
		t.Fatalf("commit fixture transaction: %v", err)
	}
}

func assertListOrder(t *testing.T, items []Project, expected ...string) {
	t.Helper()
	if len(items) != len(expected) {
		t.Fatalf("list held %d items, expected %d", len(items), len(expected))
	}
	for index, id := range expected {
		if items[index].ID.String() != id {
			t.Fatalf("item %d was %s, expected %s", index, items[index].ID, id)
		}
	}
}
