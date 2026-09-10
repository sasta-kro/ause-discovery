package artifacts

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

func TestArtifactLifecycleMaintainsProjectSearchAuditAndBytes(t *testing.T) {
	databaseURL := os.Getenv("AUSE_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("AUSE_TEST_DATABASE_URL is required for PostgreSQL integration tests")
	}

	ctx := context.Background()
	pool := createArtifactTestDatabase(t, ctx, databaseURL)
	actorID, projectID := seedArtifactOwner(t, ctx, pool)
	storage := LocalStorage{Root: t.TempDir(), MaxBytes: 1024}
	storageSet := newLocalStorageSet(t, storage)
	service := Service{Pool: pool, Storage: storageSet, MaxProjectBytes: 4096}
	content := []byte("%PDF-1.7\nartifact")

	created, err := service.Upload(ctx, actorID, projectID, 1, UploadInput{
		ArtifactType: "report", DisplayName: "Final report", OriginalFilename: "report.pdf", ExpectedSize: int64(len(content)), Content: bytes.NewReader(content),
	})
	if err != nil {
		t.Fatalf("Upload returned an error: %v", err)
	}
	if created.Status != "active" || created.Revision != 1 || created.StorageBackend != BackendLocal || !storage.Exists(ctx, created.StorageKey) {
		t.Fatalf("Upload returned invalid Artifact state: %#v", created)
	}
	assertProjectArtifactState(t, ctx, pool, projectID, 2, "remove", 1)

	updated, err := service.Update(ctx, actorID, created.ID, created.Revision, "other", "Public report")
	if err != nil {
		t.Fatalf("Update returned an error: %v", err)
	}
	if updated.ArtifactType != "other" || updated.DisplayName != "Public report" || updated.Revision != 2 {
		t.Fatalf("Update returned invalid Artifact state: %#v", updated)
	}

	deleted, err := service.Delete(ctx, actorID, created.ID, updated.Revision)
	if err != nil {
		t.Fatalf("Delete returned an error: %v", err)
	}
	if deleted.Status != "deleted" || !storage.Exists(ctx, deleted.StorageKey) {
		t.Fatalf("Delete removed content or returned invalid state: %#v", deleted)
	}

	restored, err := service.Restore(ctx, actorID, created.ID, deleted.Revision)
	if err != nil {
		t.Fatalf("Restore returned an error: %v", err)
	}
	if restored.Status != "active" || restored.Revision != 4 {
		t.Fatalf("Restore returned invalid Artifact state: %#v", restored)
	}

	replacementContent := []byte("%PDF-1.7\nreplacement")
	replacement, err := service.Replace(ctx, actorID, created.ID, restored.Revision, ReplaceInput{
		OriginalFilename: "replacement.pdf", ExpectedSize: int64(len(replacementContent)), Content: bytes.NewReader(replacementContent),
	})
	if err != nil {
		t.Fatalf("Replace returned an error: %v", err)
	}
	if replacement.ID == created.ID || replacement.StorageKey == created.StorageKey || !storage.Exists(ctx, replacement.StorageKey) || !storage.Exists(ctx, created.StorageKey) {
		t.Fatalf("Replace did not preserve immutable content identities: %#v", replacement)
	}
	var oldStatus string
	if err := pool.QueryRow(ctx, "SELECT status FROM artifacts WHERE id = $1", created.ID).Scan(&oldStatus); err != nil {
		t.Fatalf("read replaced Artifact: %v", err)
	}
	if oldStatus != "deleted" {
		t.Fatalf("replaced Artifact status was %q", oldStatus)
	}
	assertProjectArtifactState(t, ctx, pool, projectID, 6, "remove", 1)

	var auditCount int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM audit_events WHERE target_type = 'artifact' AND metadata ? 'project_id'").Scan(&auditCount); err != nil {
		t.Fatalf("count Artifact audit events: %v", err)
	}
	if auditCount != 5 {
		t.Fatalf("found %d Artifact audit events, expected 5", auditCount)
	}
}

func TestArtifactUploadEnforcesRevisionQuotaAndPublicVisibility(t *testing.T) {
	databaseURL := os.Getenv("AUSE_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("AUSE_TEST_DATABASE_URL is required for PostgreSQL integration tests")
	}

	ctx := context.Background()
	pool := createArtifactTestDatabase(t, ctx, databaseURL)
	actorID, projectID := seedArtifactOwner(t, ctx, pool)
	storage := LocalStorage{Root: t.TempDir(), MaxBytes: 1024}
	content := []byte("%PDF-1.7\nartifact")
	service := Service{Pool: pool, Storage: newLocalStorageSet(t, storage), MaxProjectBytes: int64(len(content))}

	created, err := service.Upload(ctx, actorID, projectID, 1, UploadInput{ArtifactType: "report", DisplayName: "Report", OriginalFilename: "report.pdf", ExpectedSize: int64(len(content)), Content: bytes.NewReader(content)})
	if err != nil {
		t.Fatalf("first Upload returned an error: %v", err)
	}
	if _, err := service.Upload(ctx, actorID, projectID, 1, UploadInput{ArtifactType: "report", DisplayName: "Stale", OriginalFilename: "stale.pdf", ExpectedSize: int64(len(content)), Content: bytes.NewReader(content)}); !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("stale Upload returned %v, expected revision conflict", err)
	}
	if _, err := service.Upload(ctx, actorID, projectID, 2, UploadInput{ArtifactType: "report", DisplayName: "Over quota", OriginalFilename: "quota.pdf", ExpectedSize: int64(len(content)), Content: bytes.NewReader(content)}); !errors.Is(err, ErrProjectQuotaExceeded) {
		t.Fatalf("over-quota Upload returned %v, expected quota error", err)
	}
	if countStoredFiles(t, storage.Root) != 1 {
		t.Fatalf("failed uploads left finalized content")
	}

	if _, err := service.OpenPublic(ctx, created.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("draft public Open returned %v, expected not found", err)
	}
	if _, err := pool.Exec(ctx, "UPDATE projects SET status = 'published', published_at = now() WHERE id = $1", projectID); err != nil {
		t.Fatalf("publish Project fixture: %v", err)
	}
	publicContent, err := service.OpenPublic(ctx, created.ID)
	if err != nil {
		t.Fatalf("published public Open returned an error: %v", err)
	}
	defer publicContent.File.Close()
	opened, err := io.ReadAll(publicContent.File)
	if err != nil || !bytes.Equal(opened, content) {
		t.Fatalf("public Open returned content %q and error %v", opened, err)
	}
}

func newLocalStorageSet(t *testing.T, storage LocalStorage) StorageSet {
	t.Helper()
	set, err := NewStorageSet(StorageOptions{DefaultName: BackendLocal, LocalRoot: storage.Root, MaxArtifactBytes: storage.MaxBytes})
	if err != nil {
		t.Fatalf("build local storage set: %v", err)
	}
	return set
}

func assertProjectArtifactState(t *testing.T, ctx context.Context, pool *pgxpool.Pool, projectID uuid.UUID, expectedRevision int64, expectedAction string, expectedActiveCount int) {
	t.Helper()
	var revision int64
	var desiredRevision int64
	var action string
	var activeCount int
	err := pool.QueryRow(ctx, `SELECT projects.revision, project_search_sync.desired_revision, project_search_sync.desired_action, (SELECT count(*) FROM artifacts WHERE project_id = projects.id AND status = 'active') FROM projects JOIN project_search_sync ON project_search_sync.project_id = projects.id WHERE projects.id = $1`, projectID).Scan(&revision, &desiredRevision, &action, &activeCount)
	if err != nil {
		t.Fatalf("read Project Artifact state: %v", err)
	}
	if revision != expectedRevision || desiredRevision != expectedRevision || action != expectedAction || activeCount != expectedActiveCount {
		t.Fatalf("Project Artifact state was revision %d desired %d action %q active %d", revision, desiredRevision, action, activeCount)
	}
}

func countStoredFiles(t *testing.T, root string) int {
	t.Helper()
	count := 0
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && filepath.Ext(path) != ".part" {
			count++
		}
		return nil
	})
	if err != nil {
		t.Fatalf("count stored files: %v", err)
	}
	return count
}

func createArtifactTestDatabase(t *testing.T, ctx context.Context, databaseURL string) *pgxpool.Pool {
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
	databaseName := fmt.Sprintf("ause_artifacts_%d", time.Now().UnixNano())
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

func seedArtifactOwner(t *testing.T, ctx context.Context, pool *pgxpool.Pool) (uuid.UUID, uuid.UUID) {
	t.Helper()
	actorID := uuid.MustParse("018f0000-0000-7000-8000-000000000401")
	projectID := uuid.MustParse("018f0000-0000-7000-8000-000000000402")
	statements := []string{
		"INSERT INTO application_users (id, username, status) VALUES ('018f0000-0000-7000-8000-000000000401', 'artifact-test-admin', 'active')",
		"INSERT INTO people (id, display_name, normalized_name, student_id) VALUES ('018f0000-0000-7000-8000-000000000403', 'Artifact Student', 'artifact student', '1234567')",
		"INSERT INTO people (id, display_name, normalized_name, staff_id) VALUES ('018f0000-0000-7000-8000-000000000404', 'Artifact Advisor', 'artifact advisor', 'artifact-advisor')",
		"INSERT INTO programs (id, key) VALUES ('018f0000-0000-7000-8000-000000000405', 'artifact_program')",
		"INSERT INTO program_versions (id, program_id, label, valid_from_year) VALUES ('018f0000-0000-7000-8000-000000000406', '018f0000-0000-7000-8000-000000000405', 'Artifact Program', 2020)",
		"INSERT INTO courses (id, key) VALUES ('018f0000-0000-7000-8000-000000000407', 'artifact_course')",
		"INSERT INTO course_versions (id, course_id, label, code, valid_from_year) VALUES ('018f0000-0000-7000-8000-000000000408', '018f0000-0000-7000-8000-000000000407', 'Artifact Course', 'ART499', 2020)",
		"INSERT INTO course_program_versions (course_version_id, program_version_id) VALUES ('018f0000-0000-7000-8000-000000000408', '018f0000-0000-7000-8000-000000000406')",
		"INSERT INTO taxonomy_values (id, dimension, key, labels, sort_order) VALUES ('018f0000-0000-7000-8000-000000000409', 'category', 'artifact_category', '{\"en\":\"Artifact Category\"}', 1)",
		"INSERT INTO taxonomy_values (id, dimension, key, labels, sort_order) VALUES ('018f0000-0000-7000-8000-000000000410', 'platform', 'artifact_platform', '{\"en\":\"Artifact Platform\"}', 1)",
		"INSERT INTO projects (id, title, abstract, academic_year, semester, program_version_id, course_version_id) VALUES ('018f0000-0000-7000-8000-000000000402', 'Artifact Project', 'Complete Artifact Project.', 2026, 'first', '018f0000-0000-7000-8000-000000000406', '018f0000-0000-7000-8000-000000000408')",
		"INSERT INTO project_participations (project_id, person_id, role, position) VALUES ('018f0000-0000-7000-8000-000000000402', '018f0000-0000-7000-8000-000000000403', 'student', 0), ('018f0000-0000-7000-8000-000000000402', '018f0000-0000-7000-8000-000000000404', 'advisor', 0)",
		"INSERT INTO project_taxonomy_values (project_id, taxonomy_value_id, dimension, position) VALUES ('018f0000-0000-7000-8000-000000000402', '018f0000-0000-7000-8000-000000000409', 'category', 0), ('018f0000-0000-7000-8000-000000000402', '018f0000-0000-7000-8000-000000000410', 'platform', 0)",
	}
	for _, statement := range statements {
		if _, err := pool.Exec(ctx, statement); err != nil {
			t.Fatalf("seed Artifact fixture: %v", err)
		}
	}
	return actorID, projectID
}
