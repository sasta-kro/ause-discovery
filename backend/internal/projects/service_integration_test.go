package projects

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

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
