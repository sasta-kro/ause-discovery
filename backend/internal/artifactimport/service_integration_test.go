package artifactimport

import (
	"archive/zip"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ause-discovery.local/backend/internal/artifacts"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

func TestDemoAndManifestImportsAreValidatedAndRepeatable(t *testing.T) {
	databaseURL := os.Getenv("AUSE_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("AUSE_TEST_DATABASE_URL is required for PostgreSQL integration tests")
	}
	ctx := context.Background()
	pool := createArtifactImportTestDatabase(t, ctx, databaseURL)
	projectIDs := seedArtifactImportProjects(t, ctx, pool)
	sourceDirectory := createDemoFiles(t)
	storageSet, storageErr := artifacts.NewStorageSet(artifacts.StorageOptions{DefaultName: artifacts.BackendLocal, LocalRoot: t.TempDir(), MaxArtifactBytes: 1024 * 1024})
	if storageErr != nil {
		t.Fatalf("build storage set: %v", storageErr)
	}
	service := Service{
		Pool:             pool,
		Artifacts:        artifacts.Service{Pool: pool, Storage: storageSet, MaxProjectBytes: 10 * 1024 * 1024},
		MaxArtifactBytes: 1024 * 1024,
	}

	entries, err := service.DemoEntries(ctx, sourceDirectory, nil)
	if err != nil || len(entries) != 8 {
		t.Fatalf("DemoEntries returned %d entries and error %v", len(entries), err)
	}
	dryRun, err := service.Run(ctx, entries, Options{ActorUsername: "ARTIFACT-IMPORT-ADMIN"})
	if err != nil || dryRun.ProjectCount != 2 || dryRun.PlannedUploads != 8 || dryRun.Uploaded != 0 || dryRun.Skipped != 0 {
		t.Fatalf("unexpected dry-run result %#v and error %v", dryRun, err)
	}
	assertArtifactImportCount(t, ctx, pool, 0)

	applied, err := service.Run(ctx, entries, Options{ActorUsername: "artifact-import-admin", Apply: true})
	if err != nil || applied.ProjectCount != 2 || applied.PlannedUploads != 8 || applied.Uploaded != 8 || applied.Skipped != 0 {
		t.Fatalf("unexpected apply result %#v and error %v", applied, err)
	}
	assertArtifactImportCount(t, ctx, pool, 8)
	var realisticDownloadNames int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM artifacts WHERE original_filename IN ('final-report.pdf','presentation-slides.pdf','project-poster.png','source-code.zip')`).Scan(&realisticDownloadNames); err != nil {
		t.Fatalf("count realistic download filenames: %v", err)
	}
	if realisticDownloadNames != 8 {
		t.Fatalf("found %d realistic download filenames, expected 8", realisticDownloadNames)
	}

	repeated, err := service.Run(ctx, entries, Options{ActorUsername: "artifact-import-admin", Apply: true})
	if err != nil || repeated.PlannedUploads != 0 || repeated.Uploaded != 0 || repeated.Skipped != 8 {
		t.Fatalf("unexpected repeated result %#v and error %v", repeated, err)
	}
	assertArtifactImportCount(t, ctx, pool, 8)

	proposalPath := filepath.Join(sourceDirectory, "proposal.pdf")
	if err := os.WriteFile(proposalPath, []byte("%PDF-1.7\nproposal"), 0o600); err != nil {
		t.Fatalf("write proposal: %v", err)
	}
	manifestResult, err := service.Run(ctx, []Entry{{
		ProjectImportKey: "project-one",
		ArtifactType:     "proposal",
		DisplayName:      "Project proposal",
		SourcePath:       proposalPath,
	}}, Options{ActorUsername: "artifact-import-admin", Apply: true})
	if err != nil || manifestResult.ProjectCount != 1 || manifestResult.Uploaded != 1 {
		t.Fatalf("unexpected manifest result %#v and error %v", manifestResult, err)
	}
	assertArtifactImportCount(t, ctx, pool, 9)

	var importedProjectID uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT project_id FROM artifacts WHERE type='proposal'`).Scan(&importedProjectID); err != nil {
		t.Fatalf("read manifest Artifact: %v", err)
	}
	if importedProjectID != projectIDs[0] {
		t.Fatalf("manifest Artifact was attached to Project %s", importedProjectID)
	}
}

func createDemoFiles(t *testing.T) string {
	t.Helper()
	directory := t.TempDir()
	files := map[string][]byte{
		"mock-report.pdf": []byte("%PDF-1.7\nreport"),
		"mock-slides.pdf": []byte("%PDF-1.7\nslides"),
		"mock-poster.png": {0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0, 0, 0, 0},
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(directory, name), content, 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	archive, err := os.Create(filepath.Join(directory, "mock-source-code.zip"))
	if err != nil {
		t.Fatalf("create source archive: %v", err)
	}
	writer := zip.NewWriter(archive)
	entry, err := writer.Create("README.md")
	if err == nil {
		_, err = entry.Write([]byte("Project source code"))
	}
	if closeErr := writer.Close(); err == nil {
		err = closeErr
	}
	if closeErr := archive.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		t.Fatalf("write source archive: %v", err)
	}
	return directory
}

func assertArtifactImportCount(t *testing.T, ctx context.Context, pool *pgxpool.Pool, expected int) {
	t.Helper()
	var artifactCount, auditCount int
	if err := pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM artifacts), (SELECT count(*) FROM audit_events WHERE event_type='artifact.uploaded')`).Scan(&artifactCount, &auditCount); err != nil {
		t.Fatalf("count imported Artifacts: %v", err)
	}
	if artifactCount != expected || auditCount != expected {
		t.Fatalf("found %d Artifacts and %d audit events, expected %d", artifactCount, auditCount, expected)
	}
}

func createArtifactImportTestDatabase(t *testing.T, ctx context.Context, databaseURL string) *pgxpool.Pool {
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
	databaseName := "ause_artifact_import_" + strings.ReplaceAll(uuid.NewString(), "-", "")
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

func seedArtifactImportProjects(t *testing.T, ctx context.Context, pool *pgxpool.Pool) []uuid.UUID {
	t.Helper()
	statements := []string{
		"INSERT INTO application_users (id, username, status) VALUES ('018f0000-0000-7000-8000-000000000901', 'artifact-import-admin', 'active')",
		"INSERT INTO people (id, display_name, normalized_name, student_id) VALUES ('018f0000-0000-7000-8000-000000000902', 'Import Student', 'import student', '1234567')",
		"INSERT INTO people (id, display_name, normalized_name, staff_id) VALUES ('018f0000-0000-7000-8000-000000000903', 'Import Advisor', 'import advisor', 'import-advisor')",
		"INSERT INTO programs (id, key) VALUES ('018f0000-0000-7000-8000-000000000904', 'import_program')",
		"INSERT INTO program_versions (id, program_id, label, valid_from_year) VALUES ('018f0000-0000-7000-8000-000000000905', '018f0000-0000-7000-8000-000000000904', 'Import Program', 2020)",
		"INSERT INTO courses (id, key) VALUES ('018f0000-0000-7000-8000-000000000906', 'import_course')",
		"INSERT INTO course_versions (id, course_id, label, code, valid_from_year) VALUES ('018f0000-0000-7000-8000-000000000907', '018f0000-0000-7000-8000-000000000906', 'Import Course', 'IMP499', 2020)",
		"INSERT INTO course_program_versions (course_version_id, program_version_id) VALUES ('018f0000-0000-7000-8000-000000000907', '018f0000-0000-7000-8000-000000000905')",
		"INSERT INTO taxonomy_values (id, dimension, key, labels) VALUES ('018f0000-0000-7000-8000-000000000908', 'category', 'import_category', '{\"en\":\"Import Category\"}')",
		"INSERT INTO taxonomy_values (id, dimension, key, labels) VALUES ('018f0000-0000-7000-8000-000000000909', 'platform', 'import_platform', '{\"en\":\"Import Platform\"}')",
	}
	for _, statement := range statements {
		if _, err := pool.Exec(ctx, statement); err != nil {
			t.Fatalf("seed import fixture: %v", err)
		}
	}
	projectIDs := []uuid.UUID{
		uuid.MustParse("018f0000-0000-7000-8000-000000000910"),
		uuid.MustParse("018f0000-0000-7000-8000-000000000911"),
	}
	for index, projectID := range projectIDs {
		importKey := fmt.Sprintf("project-%s", []string{"one", "two"}[index])
		if _, err := pool.Exec(ctx, `INSERT INTO projects (id,title,abstract,academic_year,semester,program_version_id,course_version_id,extra_metadata) VALUES ($1,$2,'Complete Project.',2026,'first','018f0000-0000-7000-8000-000000000905','018f0000-0000-7000-8000-000000000907',jsonb_build_object('import_key',$3::text))`, projectID, fmt.Sprintf("Import Project %d", index+1), importKey); err != nil {
			t.Fatalf("seed Project: %v", err)
		}
		if _, err := pool.Exec(ctx, `INSERT INTO project_participations (project_id,person_id,role,position) VALUES ($1,'018f0000-0000-7000-8000-000000000902','student',0),($1,'018f0000-0000-7000-8000-000000000903','advisor',0)`, projectID); err != nil {
			t.Fatalf("seed Project participations: %v", err)
		}
		if _, err := pool.Exec(ctx, `INSERT INTO project_taxonomy_values (project_id,taxonomy_value_id,dimension,position) VALUES ($1,'018f0000-0000-7000-8000-000000000908','category',0),($1,'018f0000-0000-7000-8000-000000000909','platform',0)`, projectID); err != nil {
			t.Fatalf("seed Project taxonomy: %v", err)
		}
		if _, err := pool.Exec(ctx, `UPDATE projects SET status='published',published_at=now() WHERE id=$1`, projectID); err != nil {
			t.Fatalf("publish Project: %v", err)
		}
	}
	return projectIDs
}
