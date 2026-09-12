package projectcontentimport

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"ause-discovery.local/backend/internal/artifactimport"
	"ause-discovery.local/backend/internal/artifacts"
	"ause-discovery.local/backend/internal/projectlogos"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

// countingFailingStorage delegates to a real LocalStorage but fails file
// writes once its allowance is exhausted, for deterministic apply-time
// partial failures.
type countingFailingStorage struct {
	artifacts.LocalStorage
	allow int64
	calls int64
}

func (storage *countingFailingStorage) Name() string { return artifacts.BackendLocal }

func (storage *countingFailingStorage) Put(ctx context.Context, reader io.Reader, expectedSize int64) (artifacts.StoredContent, error) {
	if atomic.AddInt64(&storage.calls, 1) > storage.allow {
		return artifacts.StoredContent{}, artifacts.ErrSizeMismatch
	}
	return storage.LocalStorage.Put(ctx, reader, expectedSize)
}

func contentPNG(t *testing.T, shade uint8) []byte {
	t.Helper()
	picture := image.NewRGBA(image.Rect(0, 0, 6, 6))
	for x := 0; x < 6; x++ {
		for y := 0; y < 6; y++ {
			picture.Set(x, y, color.RGBA{R: shade, G: shade, B: shade, A: 255})
		}
	}
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, picture); err != nil {
		t.Fatalf("encode test PNG: %v", err)
	}
	return buffer.Bytes()
}

type contentFixture struct {
	pool      *pgxpool.Pool
	bundle    string
	firstID   uuid.UUID
	secondID  uuid.UUID
	logos     projectlogos.Service
	files     artifactimport.Service
	service   Service
	adminUser string
}

func newContentFixture(t *testing.T) *contentFixture {
	t.Helper()
	databaseURL := os.Getenv("AUSE_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("AUSE_TEST_DATABASE_URL is required for PostgreSQL integration tests")
	}
	ctx := context.Background()
	fixture := &contentFixture{adminUser: "content-import-admin", bundle: t.TempDir()}
	fixture.pool = createContentTestDatabase(t, ctx, databaseURL)
	fixture.seedProjects(t, ctx)
	storageRoot := t.TempDir()
	storageSet := artifacts.StorageSet{DefaultName: artifacts.BackendLocal, Backends: map[string]artifacts.Backend{
		artifacts.BackendLocal: artifacts.LocalStorage{Root: storageRoot, MaxBytes: 1024 * 1024},
	}}
	fixture.logos = projectlogos.Service{Pool: fixture.pool, Storage: storageSet}
	fixture.files = artifactimport.Service{
		Pool:             fixture.pool,
		Artifacts:        artifacts.Service{Pool: fixture.pool, Storage: storageSet, MaxProjectBytes: 10 * 1024 * 1024},
		MaxArtifactBytes: 1024 * 1024,
	}
	fixture.service = Service{Pool: fixture.pool, Logos: fixture.logos, Files: fixture.files}
	fixture.writeBundle(t, contentPNG(t, 40), contentPNG(t, 80), contentPNG(t, 200))
	return fixture
}

func (fixture *contentFixture) seedProjects(t *testing.T, ctx context.Context) {
	t.Helper()
	fixture.firstID = uuid.MustParse("018f0000-0000-7000-8000-000000000b01")
	fixture.secondID = uuid.MustParse("018f0000-0000-7000-8000-000000000b02")
	statements := []string{
		"INSERT INTO application_users (id, username, status) VALUES ('018f0000-0000-7000-8000-000000000b10', 'content-import-admin', 'active')",
		"INSERT INTO programs (id, key) VALUES ('018f0000-0000-7000-8000-000000000b11', 'content_program')",
		"INSERT INTO program_versions (id, program_id, label, valid_from_year) VALUES ('018f0000-0000-7000-8000-000000000b12', '018f0000-0000-7000-8000-000000000b11', 'Content Program', 2020)",
		"INSERT INTO courses (id, key) VALUES ('018f0000-0000-7000-8000-000000000b13', 'content_course')",
		"INSERT INTO course_versions (id, course_id, label, code, valid_from_year) VALUES ('018f0000-0000-7000-8000-000000000b14', '018f0000-0000-7000-8000-000000000b13', 'Content Course', 'CSC499', 2020)",
		"INSERT INTO course_program_versions (course_version_id, program_version_id) VALUES ('018f0000-0000-7000-8000-000000000b14', '018f0000-0000-7000-8000-000000000b12')",
		"INSERT INTO people (id, display_name, normalized_name, student_id) VALUES ('018f0000-0000-7000-8000-000000000b20', 'Content Student', 'content student', '7418529')",
		"INSERT INTO people (id, display_name, normalized_name, staff_id) VALUES ('018f0000-0000-7000-8000-000000000b21', 'Content Advisor', 'content advisor', 'content-advisor')",
		"INSERT INTO taxonomy_values (id, dimension, key, labels, sort_order) VALUES ('018f0000-0000-7000-8000-000000000b22', 'category', 'content_category', '{\"en\":\"Content Category\"}', 1)",
		"INSERT INTO taxonomy_values (id, dimension, key, labels, sort_order) VALUES ('018f0000-0000-7000-8000-000000000b23', 'platform', 'content_platform', '{\"en\":\"Content Platform\"}', 1)",
	}
	for _, statement := range statements {
		if _, err := fixture.pool.Exec(ctx, statement); err != nil {
			t.Fatalf("seed content fixture: %v", err)
		}
	}
	for index, projectID := range []uuid.UUID{fixture.firstID, fixture.secondID} {
		importKey := fmt.Sprintf("sp-%s", []string{"first", "second"}[index])
		if _, err := fixture.pool.Exec(ctx, `INSERT INTO projects (id,title,abstract,academic_year,semester,program_version_id,course_version_id,extra_metadata) VALUES ($1,$2,'Complete Project.',2026,'first','018f0000-0000-7000-8000-000000000b12','018f0000-0000-7000-8000-000000000b14',jsonb_build_object('import_key',$3::text))`, projectID, fmt.Sprintf("Content Project %d", index+1), importKey); err != nil {
			t.Fatalf("seed content Project: %v", err)
		}
		if _, err := fixture.pool.Exec(ctx, `INSERT INTO project_participations (project_id,person_id,role,position) VALUES ($1,'018f0000-0000-7000-8000-000000000b20','student',0),($1,'018f0000-0000-7000-8000-000000000b21','advisor',0)`, projectID); err != nil {
			t.Fatalf("seed content participations: %v", err)
		}
		if _, err := fixture.pool.Exec(ctx, `INSERT INTO project_taxonomy_values (project_id,taxonomy_value_id,dimension,position) VALUES ($1,'018f0000-0000-7000-8000-000000000b22','category',0),($1,'018f0000-0000-7000-8000-000000000b23','platform',0)`, projectID); err != nil {
			t.Fatalf("seed content taxonomy: %v", err)
		}
		if _, err := fixture.pool.Exec(ctx, `UPDATE projects SET status='published', published_at=now() WHERE id=$1`, projectID); err != nil {
			t.Fatalf("publish content Project: %v", err)
		}
	}
}

func (fixture *contentFixture) writeBundle(t *testing.T, firstLogo, secondLogo, replacementLogo []byte) {
	t.Helper()
	write := func(relative string, content []byte) {
		path := filepath.Join(fixture.bundle, relative)
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatalf("create bundle directory: %v", err)
		}
		if err := os.WriteFile(path, content, 0o600); err != nil {
			t.Fatalf("write bundle file: %v", err)
		}
	}
	write("logos/first.png", firstLogo)
	write("logos/second.png", secondLogo)
	write("logos/first-replacement.png", replacementLogo)
	write("files/first/final-report.pdf", []byte("%PDF-1.7\nfirst report"))
	write("files/first/presentation-slides.pdf", []byte("%PDF-1.7\nfirst slides"))
}

func (fixture *contentFixture) manifest(t *testing.T, projects string) Manifest {
	t.Helper()
	path := filepath.Join(fixture.bundle, "manifest.json")
	contents := `{"version": 1, "projects": [` + projects + `]}`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	manifest, err := LoadManifest(path)
	if err != nil {
		t.Fatalf("LoadManifest returned an error: %v", err)
	}
	return manifest
}

func (fixture *contentFixture) projectRevision(t *testing.T, ctx context.Context, projectID uuid.UUID) int64 {
	t.Helper()
	var revision int64
	if err := fixture.pool.QueryRow(ctx, "SELECT revision FROM projects WHERE id=$1", projectID).Scan(&revision); err != nil {
		t.Fatalf("read project revision: %v", err)
	}
	return revision
}

func TestProjectContentImportPlansAppliesAndRerunsIdempotently(t *testing.T) {
	fixture := newContentFixture(t)
	ctx := context.Background()
	manifest := fixture.manifest(t, `{"project_import_key": "sp-first", "logo": {"file_path": "logos/first.png"}, "files": [
		{"artifact_type": "report", "display_name": "Final report", "original_filename": "final-report.pdf", "file_path": "files/first/final-report.pdf"},
		{"artifact_type": "slides", "display_name": "Presentation slides", "file_path": "files/first/presentation-slides.pdf"}]}, {"project_id": "`+fixture.secondID.String()+`", "logo": {"file_path": "logos/second.png"}}`)

	dryRun, err := fixture.service.Run(ctx, manifest, fixture.bundle, Options{ActorUsername: fixture.adminUser})
	if err != nil || dryRun.ProjectCount != 2 || dryRun.LogoUploads != 2 || dryRun.FileUploads != 2 || dryRun.UnchangedSkips != 0 {
		t.Fatalf("unexpected dry-run result %#v and error %v", dryRun, err)
	}
	assertContentCounts(t, ctx, fixture.pool, 0, 0)

	starts := []ApplyStart{}
	progress := []ProjectProgress{}
	applied, err := fixture.service.Run(ctx, manifest, fixture.bundle, Options{
		ActorUsername: fixture.adminUser, Apply: true, Workers: 2,
		OnApplyStart:  func(start ApplyStart) { starts = append(starts, start) },
		OnProjectDone: func(done ProjectProgress) { progress = append(progress, done) },
	})
	if err != nil || applied.LogoUploads != 2 || applied.FileUploads != 2 || applied.UnchangedSkips != 0 {
		t.Fatalf("unexpected apply result %#v and error %v", applied, err)
	}
	if len(starts) != 1 || starts[0].ProjectCount != 2 || starts[0].Workers != 2 {
		t.Fatalf("unexpected start summaries %#v", starts)
	}
	if len(progress) != 2 {
		t.Fatalf("apply produced %d progress events, expected 2", len(progress))
	}
	for index, event := range progress {
		if event.Completed != index+1 || event.Total != 2 || len(event.Items) == 0 {
			t.Fatalf("progress event %d was %+v", index, event)
		}
		for _, item := range event.Items {
			if item.State != StateUploaded {
				t.Fatalf("apply item %s was %s", item.Kind, item.State)
			}
		}
	}
	assertContentCounts(t, ctx, fixture.pool, 2, 2)
	firstRevision := fixture.projectRevision(t, ctx, fixture.firstID)
	secondRevision := fixture.projectRevision(t, ctx, fixture.secondID)

	rerunProgress := []ProjectProgress{}
	rerun, err := fixture.service.Run(ctx, manifest, fixture.bundle, Options{
		ActorUsername: fixture.adminUser, Apply: true, Workers: 2,
		OnProjectDone: func(done ProjectProgress) { rerunProgress = append(rerunProgress, done) },
	})
	if err != nil || rerun.LogoUploads != 0 || rerun.LogoReplacements != 0 || rerun.FileUploads != 0 || rerun.UnchangedSkips != 4 {
		t.Fatalf("unexpected rerun result %#v and error %v", rerun, err)
	}
	if len(rerunProgress) != 2 {
		t.Fatalf("rerun produced %d progress events, expected 2", len(rerunProgress))
	}
	for _, event := range rerunProgress {
		for _, item := range event.Items {
			if item.State != StateSkipped || item.SkipReason == "" {
				t.Fatalf("rerun item %s was %s with reason %q", item.Kind, item.State, item.SkipReason)
			}
		}
	}
	if fixture.projectRevision(t, ctx, fixture.firstID) != firstRevision || fixture.projectRevision(t, ctx, fixture.secondID) != secondRevision {
		t.Fatal("unchanged rerun advanced Project revisions")
	}
	assertContentCounts(t, ctx, fixture.pool, 2, 2)

	// A changed logo replaces only that Project's active logo.
	replacement := fixture.manifest(t, `{"project_import_key": "sp-first", "logo": {"file_path": "logos/first-replacement.png"}}, {"project_id": "`+fixture.secondID.String()+`", "logo": {"file_path": "logos/second.png"}}`)
	replaced, err := fixture.service.Run(ctx, replacement, fixture.bundle, Options{ActorUsername: fixture.adminUser, Apply: true})
	if err != nil || replaced.LogoUploads != 1 || replaced.LogoReplacements != 1 || replaced.UnchangedSkips != 1 {
		t.Fatalf("unexpected replacement result %#v and error %v", replaced, err)
	}
	var firstActiveLogos, secondActiveLogos, firstTotalLogos int
	if err := fixture.pool.QueryRow(ctx, "SELECT count(*) FROM project_logos WHERE project_id=$1 AND status='active'", fixture.firstID).Scan(&firstActiveLogos); err != nil {
		t.Fatalf("count first active logos: %v", err)
	}
	if err := fixture.pool.QueryRow(ctx, "SELECT count(*) FROM project_logos WHERE project_id=$1 AND status='active'", fixture.secondID).Scan(&secondActiveLogos); err != nil {
		t.Fatalf("count second active logos: %v", err)
	}
	if err := fixture.pool.QueryRow(ctx, "SELECT count(*) FROM project_logos WHERE project_id=$1", fixture.firstID).Scan(&firstTotalLogos); err != nil {
		t.Fatalf("count first logos: %v", err)
	}
	if firstActiveLogos != 1 || secondActiveLogos != 1 || firstTotalLogos != 2 {
		t.Fatalf("logo rows were active %d/%d total %d", firstActiveLogos, secondActiveLogos, firstTotalLogos)
	}
}

func TestProjectContentPlanningRejectsInvalidBundleBeforeWrites(t *testing.T) {
	fixture := newContentFixture(t)
	ctx := context.Background()
	path := filepath.Join(fixture.bundle, "manifest.json")
	if err := os.WriteFile(path, []byte(`{"version": 1, "projects": [{"project_import_key": "sp-first", "logo": {"file_path": "logos/not-a-logo.png"}}]}`), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(fixture.bundle, "logos/not-a-logo.png"), []byte("this is not a png"), 0o600); err != nil {
		t.Fatalf("write fake logo: %v", err)
	}
	manifest, loadErr := LoadManifest(path)
	if loadErr != nil {
		t.Fatalf("LoadManifest returned an error: %v", loadErr)
	}
	if _, err := fixture.service.Run(ctx, manifest, fixture.bundle, Options{ActorUsername: fixture.adminUser, Apply: true}); err == nil {
		t.Fatal("planning accepted an invalid logo")
	}
	assertContentCounts(t, ctx, fixture.pool, 0, 0)

	missing := fixture.manifest(t, `{"project_import_key": "sp-missing", "logo": {"file_path": "logos/first.png"}}`)
	if _, err := fixture.service.Run(ctx, missing, fixture.bundle, Options{ActorUsername: fixture.adminUser}); err == nil || !strings.Contains(err.Error(), "sp-missing") {
		t.Fatalf("missing Project returned %v", err)
	}
}

func TestProjectContentPartialFailureLeavesWorkAndRerunCompletes(t *testing.T) {
	fixture := newContentFixture(t)
	ctx := context.Background()
	manifest := fixture.manifest(t, `{"project_import_key": "sp-first", "logo": {"file_path": "logos/first.png"}, "files": [
		{"artifact_type": "report", "display_name": "Final report", "file_path": "files/first/final-report.pdf"},
		{"artifact_type": "slides", "display_name": "Presentation slides", "file_path": "files/first/presentation-slides.pdf"}]}, {"project_id": "`+fixture.secondID.String()+`", "logo": {"file_path": "logos/second.png"}}`)

	healthyRoot := artifacts.LocalStorage{Root: t.TempDir(), MaxBytes: 1024 * 1024}
	failingFiles := artifactimport.Service{
		Pool: fixture.pool,
		Artifacts: artifacts.Service{Pool: fixture.pool, Storage: artifacts.StorageSet{DefaultName: artifacts.BackendLocal, Backends: map[string]artifacts.Backend{
			artifacts.BackendLocal: &countingFailingStorage{LocalStorage: healthyRoot, allow: 1},
		}}, MaxProjectBytes: 10 * 1024 * 1024},
		MaxArtifactBytes: 1024 * 1024,
	}
	failingService := Service{Pool: fixture.pool, Logos: projectlogos.Service{Pool: fixture.pool, Storage: artifacts.StorageSet{DefaultName: artifacts.BackendLocal, Backends: map[string]artifacts.Backend{artifacts.BackendLocal: healthyRoot}}}, Files: failingFiles}

	// With one worker the interleaving is deterministic: the first Project
	// uploads its logo and first file, fails its second file, and stops;
	// the second Project still runs because the failure is not fatal.
	progress := []ProjectProgress{}
	failed, err := failingService.Run(ctx, manifest, fixture.bundle, Options{
		ActorUsername: fixture.adminUser, Apply: true, Workers: 1,
		OnProjectDone: func(done ProjectProgress) { progress = append(progress, done) },
	})
	if err == nil || !strings.Contains(err.Error(), "1 of 2 Projects failed") {
		t.Fatalf("partial failure returned %v", err)
	}
	if failed.FailedProjects != 1 || failed.FailedItems != 1 || failed.LogoUploads != 2 || failed.FileUploads != 1 {
		t.Fatalf("unexpected failed result %#v", failed)
	}
	if len(progress) != 2 {
		t.Fatalf("partial failure produced %d progress events, expected 2", len(progress))
	}
	assertContentCounts(t, ctx, fixture.pool, 2, 1)

	recovered, err := fixture.service.Run(ctx, manifest, fixture.bundle, Options{ActorUsername: fixture.adminUser, Apply: true})
	if err != nil || recovered.FileUploads != 1 || recovered.UnchangedSkips != 3 {
		t.Fatalf("unexpected recovered result %#v and error %v", recovered, err)
	}
	assertContentCounts(t, ctx, fixture.pool, 2, 2)
}

func assertContentCounts(t *testing.T, ctx context.Context, pool *pgxpool.Pool, expectedLogos, expectedArtifacts int) {
	t.Helper()
	var logoCount, artifactCount int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM project_logos WHERE status='active'").Scan(&logoCount); err != nil {
		t.Fatalf("count logos: %v", err)
	}
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM artifacts").Scan(&artifactCount); err != nil {
		t.Fatalf("count artifacts: %v", err)
	}
	if logoCount != expectedLogos || artifactCount != expectedArtifacts {
		t.Fatalf("content counts were %d logos and %d artifacts, expected %d and %d", logoCount, artifactCount, expectedLogos, expectedArtifacts)
	}
}

func createContentTestDatabase(t *testing.T, ctx context.Context, databaseURL string) *pgxpool.Pool {
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
	databaseName := "ause_content_import_" + strings.ReplaceAll(uuid.NewString(), "-", "")
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

func TestProjectContentImportRejectsDuplicateResolvedProjects(t *testing.T) {
	fixture := newContentFixture(t)
	ctx := context.Background()

	// The same database Project reached once by import key and once by UUID:
	// the loader cannot see the collision, so planning must reject it after
	// resolution and before any write.
	manifest := fixture.manifest(t, `{"project_import_key": "sp-first", "logo": {"file_path": "logos/first.png"}}, {"project_id": "`+fixture.firstID.String()+`", "logo": {"file_path": "logos/second.png"}}`)
	if _, err := fixture.service.Run(ctx, manifest, fixture.bundle, Options{ActorUsername: fixture.adminUser, Apply: true}); err == nil || !strings.Contains(err.Error(), "already resolved") {
		t.Fatalf("duplicate resolution returned %v, expected a duplicate rejection", err)
	}
	assertContentCounts(t, ctx, fixture.pool, 0, 0)
}

// contentDOCX builds a minimal DOCX-compatible package inside the bundle.
func contentDOCX(t *testing.T) []byte {
	t.Helper()
	var buffer bytes.Buffer
	archive := zip.NewWriter(&buffer)
	for name, body := range map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"/>`,
		"word/document.xml":   `<?xml version="1.0"?><w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"/>`,
	} {
		entry, err := archive.Create(name)
		if err != nil {
			t.Fatalf("create fixture entry: %v", err)
		}
		if _, err := entry.Write([]byte(body)); err != nil {
			t.Fatalf("write fixture entry: %v", err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatalf("close fixture archive: %v", err)
	}
	return buffer.Bytes()
}

func TestProjectContentImportPlansDOCXReports(t *testing.T) {
	fixture := newContentFixture(t)
	ctx := context.Background()
	docx := contentDOCX(t)
	if err := os.MkdirAll(filepath.Join(fixture.bundle, "files", "first"), 0o750); err != nil {
		t.Fatalf("create bundle directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(fixture.bundle, "files", "first", "final-report.docx"), docx, 0o600); err != nil {
		t.Fatalf("write DOCX source: %v", err)
	}
	manifest := fixture.manifest(t, `{"project_import_key": "sp-first", "files": [
		{"artifact_type": "report", "display_name": "Final report", "original_filename": "final-report.docx", "file_path": "files/first/final-report.docx"}]}`)
	dryRun, err := fixture.service.Run(ctx, manifest, fixture.bundle, Options{ActorUsername: fixture.adminUser})
	if err != nil || dryRun.FileUploads != 1 || dryRun.LogoUploads != 0 {
		t.Fatalf("DOCX report planning returned %#v and error %v, expected one file upload", dryRun, err)
	}

	applied, err := fixture.service.Run(ctx, manifest, fixture.bundle, Options{ActorUsername: fixture.adminUser, Apply: true})
	if err != nil || applied.FileUploads != 1 {
		t.Fatalf("DOCX report apply returned %#v and error %v", applied, err)
	}
	var extension string
	if err := fixture.pool.QueryRow(ctx, "SELECT extension FROM artifacts WHERE type='report' LIMIT 1").Scan(&extension); err != nil || extension != "docx" {
		t.Fatalf("stored report extension was %q with error %v, expected docx", extension, err)
	}
}
