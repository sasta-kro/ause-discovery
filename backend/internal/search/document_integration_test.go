package search

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

func TestBuildProjectDocumentUsesOnlyPublishedSearchProjection(t *testing.T) {
	databaseURL := os.Getenv("AUSE_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("AUSE_TEST_DATABASE_URL is required for PostgreSQL integration tests")
	}
	ctx := context.Background()
	pool := createSearchTestDatabase(t, ctx, databaseURL)
	projectID := seedSearchProject(t, ctx, pool)

	document, err := BuildProjectDocument(ctx, pool, projectID)
	if err != nil {
		t.Fatalf("BuildProjectDocument returned an error: %v", err)
	}
	if document.Title != "Searchable Project" || document.Revision != 7 || document.Program.Key != "search_program" || document.Course.Key != "search_course" {
		t.Fatalf("document omitted core Project values: %#v", document)
	}
	if len(document.StudentIDs) != 1 || document.StudentIDs[0] != "7654321" || len(document.AdvisorPersonIDs) != 1 {
		t.Fatalf("document omitted Person facets: %#v", document)
	}
	if len(document.CategoryKeys) != 1 || document.CategoryKeys[0] != "search_category" || len(document.PlatformKeys) != 1 {
		t.Fatalf("document omitted taxonomy facets: %#v", document)
	}
	if document.ArtifactCount != 1 || !document.HasReport || document.HasDataset || len(document.ArtifactTypes) != 1 {
		t.Fatalf("document included invalid Artifact availability: %#v", document)
	}
	encoded, err := json.Marshal(document)
	if err != nil {
		t.Fatalf("marshal document: %v", err)
	}
	for _, forbidden := range []string{"storage_key", "extra_metadata", "search-secret-storage-key", "deleted.csv"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("document exposed forbidden value %q: %s", forbidden, encoded)
		}
	}

	if _, err := pool.Exec(ctx, "UPDATE projects SET status='draft', published_at=NULL, revision=revision+1 WHERE id=$1", projectID); err != nil {
		t.Fatalf("unpublish Project fixture: %v", err)
	}
	if _, err := BuildProjectDocument(ctx, pool, projectID); err != ErrProjectNotPublished {
		t.Fatalf("draft BuildProjectDocument returned %v, expected not published", err)
	}
}

func createSearchTestDatabase(t *testing.T, ctx context.Context, databaseURL string) *pgxpool.Pool {
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
	databaseName := fmt.Sprintf("ause_search_%d", time.Now().UnixNano())
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
		connection, connectErr := pgx.ConnectConfig(ctx, adminConfig)
		if connectErr != nil {
			return
		}
		defer connection.Close(ctx)
		_, _ = connection.Exec(ctx, "DROP DATABASE IF EXISTS "+databaseName+" WITH (FORCE)")
	})
	return pool
}

func seedSearchProject(t *testing.T, ctx context.Context, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	projectID := uuid.MustParse("018f0000-0000-7000-8000-000000000602")
	statements := []string{
		"INSERT INTO application_users (id, username, status) VALUES ('018f0000-0000-7000-8000-000000000601', 'search-test-admin', 'active')",
		"INSERT INTO people (id, display_name, normalized_name, student_id) VALUES ('018f0000-0000-7000-8000-000000000603', 'Search Student', 'search student', '7654321')",
		"INSERT INTO people (id, display_name, normalized_name, staff_id) VALUES ('018f0000-0000-7000-8000-000000000604', 'Search Advisor', 'search advisor', 'search-advisor')",
		"INSERT INTO programs (id, key) VALUES ('018f0000-0000-7000-8000-000000000605', 'search_program')",
		"INSERT INTO program_versions (id, program_id, label, valid_from_year) VALUES ('018f0000-0000-7000-8000-000000000606', '018f0000-0000-7000-8000-000000000605', 'Search Program', 2020)",
		"INSERT INTO courses (id, key) VALUES ('018f0000-0000-7000-8000-000000000607', 'search_course')",
		"INSERT INTO course_versions (id, course_id, label, code, valid_from_year) VALUES ('018f0000-0000-7000-8000-000000000608', '018f0000-0000-7000-8000-000000000607', 'Search Course', 'SEA499', 2020)",
		"INSERT INTO course_program_versions (course_version_id, program_version_id) VALUES ('018f0000-0000-7000-8000-000000000608', '018f0000-0000-7000-8000-000000000606')",
		"INSERT INTO taxonomy_values (id, dimension, key, labels, sort_order) VALUES ('018f0000-0000-7000-8000-000000000609', 'category', 'search_category', '{\"en\":\"Search Category\"}', 1)",
		"INSERT INTO taxonomy_values (id, dimension, key, labels, sort_order) VALUES ('018f0000-0000-7000-8000-000000000610', 'platform', 'search_platform', '{\"en\":\"Search Platform\"}', 1)",
		"INSERT INTO projects (id, reference_code, title, abstract, academic_year, semester, program_version_id, course_version_id, status, revision, extra_metadata) VALUES ('018f0000-0000-7000-8000-000000000602', 'SEARCH-1', 'Searchable Project', 'A bounded searchable abstract.', 2026, 'second', '018f0000-0000-7000-8000-000000000606', '018f0000-0000-7000-8000-000000000608', 'draft', 1, '{\"private\":\"value\"}')",
		"INSERT INTO project_title_aliases (project_id, position, value, normalized_value) VALUES ('018f0000-0000-7000-8000-000000000602', 0, 'Discovery Alias', 'discovery alias')",
		"INSERT INTO project_participations (project_id, person_id, role, position) VALUES ('018f0000-0000-7000-8000-000000000602', '018f0000-0000-7000-8000-000000000603', 'student', 0), ('018f0000-0000-7000-8000-000000000602', '018f0000-0000-7000-8000-000000000604', 'advisor', 0)",
		"INSERT INTO project_taxonomy_values (project_id, taxonomy_value_id, dimension, position) VALUES ('018f0000-0000-7000-8000-000000000602', '018f0000-0000-7000-8000-000000000609', 'category', 0), ('018f0000-0000-7000-8000-000000000602', '018f0000-0000-7000-8000-000000000610', 'platform', 0)",
		"INSERT INTO artifacts (id, project_id, type, display_name, original_filename, storage_key, mime_type, extension, byte_count, sha256, status) VALUES ('018f0000-0000-7000-8000-000000000611', '018f0000-0000-7000-8000-000000000602', 'report', 'Report', 'report.pdf', 'v1/search/report', 'application/pdf', 'pdf', 10, decode(repeat('00',32),'hex'), 'active')",
		"INSERT INTO artifacts (id, project_id, type, display_name, original_filename, storage_key, mime_type, extension, byte_count, sha256, status, deleted_at) VALUES ('018f0000-0000-7000-8000-000000000612', '018f0000-0000-7000-8000-000000000602', 'dataset', 'Deleted', 'deleted.csv', 'search-secret-storage-key', 'text/plain; charset=utf-8', 'csv', 10, decode(repeat('11',32),'hex'), 'deleted', now())",
		"UPDATE projects SET status='published', revision=7, published_at=now() WHERE id='018f0000-0000-7000-8000-000000000602'",
	}
	for _, statement := range statements {
		if _, err := pool.Exec(ctx, statement); err != nil {
			t.Fatalf("seed search fixture: %v", err)
		}
	}
	return projectID
}

func TestBuildProjectDocumentProjectsActiveLogoRevision(t *testing.T) {
	databaseURL := os.Getenv("AUSE_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("AUSE_TEST_DATABASE_URL is required for PostgreSQL integration tests")
	}
	ctx := context.Background()
	pool := createSearchTestDatabase(t, ctx, databaseURL)
	projectID := seedSearchProject(t, ctx, pool)

	withoutLogo, err := BuildProjectDocument(ctx, pool, projectID)
	if err != nil {
		t.Fatalf("BuildProjectDocument without a logo returned an error: %v", err)
	}
	if withoutLogo.LogoRevision != nil {
		t.Fatalf("document carried logo revision %d without any logo row", *withoutLogo.LogoRevision)
	}

	if _, err := pool.Exec(ctx, "INSERT INTO project_logos (id, project_id, storage_key, storage_backend, mime_type, extension, byte_count, sha256, status, revision) VALUES ('018f0000-0000-7000-8000-000000000620', $1, 'search-logo/v1', 'local', 'image/png', 'png', 10, decode(repeat('22',32),'hex'), 'active', 4)", projectID); err != nil {
		t.Fatalf("seed active logo: %v", err)
	}
	withLogo, err := BuildProjectDocument(ctx, pool, projectID)
	if err != nil {
		t.Fatalf("BuildProjectDocument with a logo returned an error: %v", err)
	}
	if withLogo.LogoRevision == nil || *withLogo.LogoRevision != 4 {
		t.Fatalf("document carried logo revision %v, expected 4", withLogo.LogoRevision)
	}

	if _, err := pool.Exec(ctx, "UPDATE project_logos SET status='deleted', revision=5, deleted_at=now()"); err != nil {
		t.Fatalf("soft-delete logo: %v", err)
	}
	afterRemoval, err := BuildProjectDocument(ctx, pool, projectID)
	if err != nil {
		t.Fatalf("BuildProjectDocument after logo removal returned an error: %v", err)
	}
	if afterRemoval.LogoRevision != nil {
		t.Fatalf("document carried logo revision %d after removal", *afterRemoval.LogoRevision)
	}
}
