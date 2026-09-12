package projectlogos

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"strings"
	"testing"

	"ause-discovery.local/backend/internal/artifacts"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

func pngBytes(t *testing.T, width, height int) []byte {
	t.Helper()
	picture := image.NewRGBA(image.Rect(0, 0, width, height))
	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			picture.Set(x, y, color.RGBA{R: 90, G: 40, B: 160, A: 255})
		}
	}
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, picture); err != nil {
		t.Fatalf("encode test PNG: %v", err)
	}
	return buffer.Bytes()
}

// corruptByte flips one byte of a PNG copy at the given offset so the file
// keeps its valid header while the image data or terminal chunk breaks.
func corruptByte(source []byte, offset int) []byte {
	corrupted := append([]byte{}, source...)
	corrupted[offset] ^= 0xff
	return corrupted
}

// renamedLocalStorage serves a LocalStorage root under a different backend
// name, standing in for a healthy alternate provider in tests.
type renamedLocalStorage struct {
	artifacts.LocalStorage
	name string
}

func (storage renamedLocalStorage) Name() string { return storage.name }

// failingStorage rejects every write with storage unavailability.
type failingStorage struct {
	artifacts.LocalStorage
}

func (storage *failingStorage) Name() string { return artifacts.BackendB2 }
func (storage *failingStorage) Put(ctx context.Context, reader io.Reader, expectedSize int64) (artifacts.StoredContent, error) {
	return artifacts.StoredContent{}, artifacts.ErrStorageUnavailable
}
func (storage *failingStorage) PutAt(ctx context.Context, storageKey string, reader io.Reader, expectedSize int64) (artifacts.StoredContent, error) {
	return artifacts.StoredContent{}, artifacts.ErrStorageUnavailable
}

func TestValidateAcceptsAndRejectsImages(t *testing.T) {
	valid := pngBytes(t, 8, 8)
	validated, err := Validate(bytes.NewReader(valid), int64(len(valid)))
	if err != nil || validated.Width != 8 || validated.Height != 8 || validated.ByteCount != int64(len(valid)) {
		t.Fatalf("valid PNG returned %+v and %v", validated, err)
	}
	if validated.SHA256 != sha256.Sum256(valid) {
		t.Fatal("digest mismatch")
	}

	if _, err := Validate(bytes.NewReader(nil), 0); !errors.Is(err, ErrEmptyContent) {
		t.Fatalf("empty content returned %v", err)
	}
	if _, err := Validate(bytes.NewReader(bytes.Repeat([]byte("x"), int(MaxBytes)+1)), -1); !errors.Is(err, ErrContentTooLarge) {
		t.Fatalf("oversize content returned %v", err)
	}
	if _, err := Validate(bytes.NewReader([]byte("not a png at all")), -1); !errors.Is(err, ErrInvalidContentType) {
		t.Fatalf("wrong MIME returned %v", err)
	}
	// A decodable header proves nothing about the image data: fully formed
	// headers with truncated, corrupt, or missing image data are rejected.
	corruptions := map[string][]byte{
		"truncated after the header":  append([]byte{}, valid[:33]...),
		"truncated inside image data": append([]byte{}, valid[:len(valid)/2]...),
		"missing terminal chunk":      append([]byte{}, valid[:len(valid)-12]...),
		"missing all image data":      append([]byte{}, valid[:8+25+13]...),
		"corrupt image checksum":      corruptByte(valid, len(valid)-40),
		"corrupt terminal chunk data": corruptByte(valid, len(valid)-8),
	}
	for name, corrupted := range corruptions {
		if _, err := Validate(bytes.NewReader(corrupted), -1); !errors.Is(err, ErrCorruptContent) {
			t.Fatalf("%s was accepted or misclassified: %v", name, err)
		}
	}
	truncated := append([]byte{}, valid[:16]...)
	if _, err := Validate(bytes.NewReader(truncated), -1); err == nil {
		t.Fatal("truncated PNG was accepted")
	}
	over := pngBytes(t, MaxDimension+1, 1)
	if _, err := Validate(bytes.NewReader(over), -1); !errors.Is(err, ErrInvalidDimensions) {
		t.Fatalf("over-dimension image returned %v", err)
	}
	if _, err := Validate(bytes.NewReader(valid), int64(len(valid))+1); !errors.Is(err, ErrSizeMismatch) {
		t.Fatalf("declared size mismatch returned %v", err)
	}
}

func TestProjectLogoLifecycleMaintainsActiveRowAndAudit(t *testing.T) {
	databaseURL := os.Getenv("AUSE_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("AUSE_TEST_DATABASE_URL is required for PostgreSQL integration tests")
	}
	ctx := context.Background()
	pool := createLogoTestDatabase(t, ctx, databaseURL)
	publishedID, draftID := seedLogoProjects(t, ctx, pool)
	root := t.TempDir()
	storage := artifacts.LocalStorage{Root: root, MaxBytes: 1 << 20}
	set := artifacts.StorageSet{DefaultName: artifacts.BackendLocal, Backends: map[string]artifacts.Backend{
		artifacts.BackendLocal: storage,
		artifacts.BackendB2:    renamedLocalStorage{LocalStorage: storage, name: artifacts.BackendB2},
	}}
	actorID := uuid.MustParse("018f0000-0000-7000-8000-000000000a01")
	service := Service{Pool: pool, Storage: set}

	first := pngBytes(t, 8, 8)
	logo, err := service.Upload(ctx, actorID, publishedID, 1, UploadInput{ExpectedSize: int64(len(first)), Content: bytes.NewReader(first)})
	if err != nil {
		t.Fatalf("Upload returned an error: %v", err)
	}
	if logo.Status != "active" || logo.StorageBackend != artifacts.BackendLocal || logo.MIMEType != MIMEType || logo.Extension != Extension {
		t.Fatalf("Upload returned invalid logo state: %#v", logo)
	}
	assertActiveLogoCount(t, ctx, pool, publishedID, 1)
	assertProjectState(t, ctx, pool, publishedID, 2, "upsert")
	assertAuditEvent(t, ctx, pool, "project_logo.uploaded")

	content, err := service.OpenActive(ctx, publishedID, logo.Revision)
	if err != nil {
		t.Fatalf("OpenActive returned an error: %v", err)
	}
	opened, _ := io.ReadAll(content.File)
	_ = content.File.Close()
	if !bytes.Equal(opened, first) {
		t.Fatal("opened logo bytes differ from the upload")
	}
	if _, err := service.OpenActive(ctx, draftID, 1); !errors.Is(err, ErrNotFound) {
		t.Fatalf("draft Project logo open returned %v", err)
	}

	if _, hasActive, err := service.ActiveDigest(ctx, draftID); err != nil || hasActive {
		t.Fatalf("draft Project ActiveDigest returned %v with active %t", err, hasActive)
	}

	second := pngBytes(t, 16, 16)
	replacement, err := service.Upload(ctx, actorID, publishedID, 2, UploadInput{ExpectedSize: int64(len(second)), Content: bytes.NewReader(second)})
	if err != nil {
		t.Fatalf("replacement Upload returned an error: %v", err)
	}
	if replacement.ID == logo.ID || replacement.StorageKey == logo.StorageKey {
		t.Fatal("replacement reused the prior logo identity")
	}
	// The soft-deleted row consumed the next revision, so the replacement
	// sits two steps past the replaced row and strictly above every earlier
	// public version.
	if replacement.Revision != logo.Revision+2 {
		t.Fatalf("replacement revision was %d, expected %d so logo_url versions strictly increase", replacement.Revision, logo.Revision+2)
	}
	assertActiveLogoCount(t, ctx, pool, publishedID, 1)
	assertAuditEvent(t, ctx, pool, "project_logo.replaced")

	if _, _, err := service.ActiveRevision(ctx, publishedID); err != nil {
		t.Fatalf("ActiveRevision returned %v", err)
	}

	if err := service.Remove(ctx, actorID, publishedID, 3); err != nil {
		t.Fatalf("Remove returned an error: %v", err)
	}
	assertActiveLogoCount(t, ctx, pool, publishedID, 0)
	assertProjectState(t, ctx, pool, publishedID, 4, "upsert")
	assertAuditEvent(t, ctx, pool, "project_logo.removed")
	var retainedRows int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM project_logos WHERE project_id=$1", publishedID).Scan(&retainedRows); err != nil || retainedRows != 2 {
		t.Fatalf("soft-deleted logo rows were discarded: %d rows, error %v", retainedRows, err)
	}

	if _, err := service.Upload(ctx, actorID, publishedID, 99, UploadInput{ExpectedSize: int64(len(first)), Content: bytes.NewReader(first)}); !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("stale revision upload returned %v", err)
	}
}

func TestProjectLogoUploadFailsCleanlyOnStorageAndTransaction(t *testing.T) {
	databaseURL := os.Getenv("AUSE_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("AUSE_TEST_DATABASE_URL is required for PostgreSQL integration tests")
	}
	ctx := context.Background()
	pool := createLogoTestDatabase(t, ctx, databaseURL)
	publishedID, _ := seedLogoProjects(t, ctx, pool)
	storage := artifacts.LocalStorage{Root: t.TempDir(), MaxBytes: 1 << 20}
	actorID := uuid.MustParse("018f0000-0000-7000-8000-000000000a01")

	failing := Service{Pool: pool, Storage: artifacts.StorageSet{DefaultName: artifacts.BackendB2, Backends: map[string]artifacts.Backend{
		artifacts.BackendLocal: storage,
		artifacts.BackendB2:    &failingStorage{LocalStorage: storage},
	}}}
	image := pngBytes(t, 4, 4)
	if _, err := failing.Upload(ctx, actorID, publishedID, 1, UploadInput{ExpectedSize: int64(len(image)), Content: bytes.NewReader(image)}); !errors.Is(err, ErrStorageUnavailable) {
		t.Fatalf("storage failure returned %v", err)
	}

	healthy := Service{Pool: pool, Storage: artifacts.StorageSet{DefaultName: artifacts.BackendLocal, Backends: map[string]artifacts.Backend{artifacts.BackendLocal: storage}}}
	if _, err := healthy.Upload(ctx, actorID, publishedID, 99, UploadInput{ExpectedSize: int64(len(image)), Content: bytes.NewReader(image)}); !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("stale revision upload returned %v", err)
	}
	entries, readErr := os.ReadDir(storage.Root)
	if readErr != nil {
		t.Fatalf("read storage root: %v", readErr)
	}
	files := 0
	for _, entry := range entries {
		if !entry.IsDir() && !strings.HasSuffix(entry.Name(), ".part") {
			files++
		}
	}
	if files != 0 {
		t.Fatalf("failed transaction left %d stored logo objects", files)
	}
}

func assertActiveLogoCount(t *testing.T, ctx context.Context, pool *pgxpool.Pool, projectID uuid.UUID, expected int) {
	t.Helper()
	var count int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM project_logos WHERE project_id=$1 AND status='active'", projectID).Scan(&count); err != nil || count != expected {
		t.Fatalf("active logo count was %d, expected %d, error %v", count, expected, err)
	}
}

func assertProjectState(t *testing.T, ctx context.Context, pool *pgxpool.Pool, projectID uuid.UUID, expectedRevision int64, expectedAction string) {
	t.Helper()
	var revision int64
	var desiredRevision int64
	var action string
	err := pool.QueryRow(ctx, `SELECT projects.revision, project_search_sync.desired_revision, project_search_sync.desired_action FROM projects JOIN project_search_sync ON project_search_sync.project_id=projects.id WHERE projects.id=$1`, projectID).Scan(&revision, &desiredRevision, &action)
	if err != nil || revision != expectedRevision || desiredRevision != expectedRevision || action != expectedAction {
		t.Fatalf("Project state was revision %d desired %d action %q, error %v", revision, desiredRevision, action, err)
	}
}

func assertAuditEvent(t *testing.T, ctx context.Context, pool *pgxpool.Pool, eventType string) {
	t.Helper()
	var count int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM audit_events WHERE event_type=$1 AND target_type='project_logo'", eventType).Scan(&count); err != nil || count != 1 {
		t.Fatalf("audit event %s appeared %d times, error %v", eventType, count, err)
	}
}

func createLogoTestDatabase(t *testing.T, ctx context.Context, databaseURL string) *pgxpool.Pool {
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
	databaseName := "ause_project_logos_" + strings.ReplaceAll(uuid.NewString(), "-", "")
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

func seedLogoProjects(t *testing.T, ctx context.Context, pool *pgxpool.Pool) (uuid.UUID, uuid.UUID) {
	t.Helper()
	statements := []string{
		"INSERT INTO application_users (id, username, status) VALUES ('018f0000-0000-7000-8000-000000000a01', 'logo-test-admin', 'active')",
		"INSERT INTO programs (id, key) VALUES ('018f0000-0000-7000-8000-000000000a10', 'logo_program')",
		"INSERT INTO program_versions (id, program_id, label, valid_from_year) VALUES ('018f0000-0000-7000-8000-000000000a11', '018f0000-0000-7000-8000-000000000a10', 'Logo Program', 2020)",
		"INSERT INTO courses (id, key) VALUES ('018f0000-0000-7000-8000-000000000a12', 'logo_course')",
		"INSERT INTO course_versions (id, course_id, label, code, valid_from_year) VALUES ('018f0000-0000-7000-8000-000000000a13', '018f0000-0000-7000-8000-000000000a12', 'Logo Course', 'LOG499', 2020)",
		"INSERT INTO course_program_versions (course_version_id, program_version_id) VALUES ('018f0000-0000-7000-8000-000000000a13', '018f0000-0000-7000-8000-000000000a11')",
		"INSERT INTO people (id, display_name, normalized_name, student_id) VALUES ('018f0000-0000-7000-8000-000000000a30', 'Logo Student', 'logo student', '7654321')",
		"INSERT INTO people (id, display_name, normalized_name, staff_id) VALUES ('018f0000-0000-7000-8000-000000000a31', 'Logo Advisor', 'logo advisor', 'logo-advisor')",
		"INSERT INTO taxonomy_values (id, dimension, key, labels, sort_order) VALUES ('018f0000-0000-7000-8000-000000000a32', 'category', 'logo_category', '{\"en\":\"Logo Category\"}', 1)",
		"INSERT INTO taxonomy_values (id, dimension, key, labels, sort_order) VALUES ('018f0000-0000-7000-8000-000000000a33', 'platform', 'logo_platform', '{\"en\":\"Logo Platform\"}', 1)",
		"INSERT INTO projects (id, title, abstract, academic_year, semester, program_version_id, course_version_id) VALUES ('018f0000-0000-7000-8000-000000000a20', 'Published Logo Project', 'Complete.', 2026, 'first', '018f0000-0000-7000-8000-000000000a11', '018f0000-0000-7000-8000-000000000a13')",
		"INSERT INTO projects (id, title, abstract, academic_year, semester, program_version_id, course_version_id) VALUES ('018f0000-0000-7000-8000-000000000a21', 'Draft Logo Project', 'Draft.', 2026, 'first', '018f0000-0000-7000-8000-000000000a11', '018f0000-0000-7000-8000-000000000a13')",
		"INSERT INTO project_participations (project_id, person_id, role, position) VALUES ('018f0000-0000-7000-8000-000000000a20', '018f0000-0000-7000-8000-000000000a30', 'student', 0), ('018f0000-0000-7000-8000-000000000a20', '018f0000-0000-7000-8000-000000000a31', 'advisor', 0)",
		"INSERT INTO project_taxonomy_values (project_id, taxonomy_value_id, dimension, position) VALUES ('018f0000-0000-7000-8000-000000000a20', '018f0000-0000-7000-8000-000000000a32', 'category', 0), ('018f0000-0000-7000-8000-000000000a20', '018f0000-0000-7000-8000-000000000a33', 'platform', 0)",
		"UPDATE projects SET status='published', published_at=now() WHERE id='018f0000-0000-7000-8000-000000000a20'",
	}
	for _, statement := range statements {
		if _, err := pool.Exec(ctx, statement); err != nil {
			t.Fatalf("seed logo fixture: %v", err)
		}
	}
	return uuid.MustParse("018f0000-0000-7000-8000-000000000a20"), uuid.MustParse("018f0000-0000-7000-8000-000000000a21")
}

// countingOpenStorage records how often content is opened so tests can prove
// a stale version request never reaches the storage provider.
type countingOpenStorage struct {
	artifacts.LocalStorage
	opens int
}

func (storage *countingOpenStorage) Open(ctx context.Context, storageKey string) (io.ReadSeekCloser, int64, error) {
	storage.opens++
	return storage.LocalStorage.Open(ctx, storageKey)
}

func TestProjectLogoRevisionsNeverReuseVersions(t *testing.T) {
	databaseURL := os.Getenv("AUSE_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("AUSE_TEST_DATABASE_URL is required for PostgreSQL integration tests")
	}
	ctx := context.Background()
	pool := createLogoTestDatabase(t, ctx, databaseURL)
	publishedID, draftID := seedLogoProjects(t, ctx, pool)
	counting := &countingOpenStorage{LocalStorage: artifacts.LocalStorage{Root: t.TempDir(), MaxBytes: 1 << 20}}
	service := Service{Pool: pool, Storage: artifacts.StorageSet{DefaultName: artifacts.BackendLocal, Backends: map[string]artifacts.Backend{
		artifacts.BackendLocal: counting,
	}}}
	actorID := uuid.MustParse("018f0000-0000-7000-8000-000000000a01")

	firstBytes, secondBytes, thirdBytes := pngBytes(t, 8, 8), pngBytes(t, 12, 12), pngBytes(t, 16, 16)
	first, err := service.Upload(ctx, actorID, publishedID, 1, UploadInput{ExpectedSize: int64(len(firstBytes)), Content: bytes.NewReader(firstBytes)})
	if err != nil {
		t.Fatalf("first upload returned an error: %v", err)
	}
	second, err := service.Upload(ctx, actorID, publishedID, 2, UploadInput{ExpectedSize: int64(len(secondBytes)), Content: bytes.NewReader(secondBytes)})
	if err != nil {
		t.Fatalf("replacement upload returned an error: %v", err)
	}
	if err := service.Remove(ctx, actorID, publishedID, 3); err != nil {
		t.Fatalf("remove returned an error: %v", err)
	}
	readded, err := service.Upload(ctx, actorID, publishedID, 4, UploadInput{ExpectedSize: int64(len(thirdBytes)), Content: bytes.NewReader(thirdBytes)})
	if err != nil {
		t.Fatalf("re-upload returned an error: %v", err)
	}

	// The re-added logo must sit strictly above every earlier version.
	if readded.Revision <= second.Revision || readded.Revision <= first.Revision {
		t.Fatalf("re-added revision %d did not exceed earlier versions %d and %d", readded.Revision, first.Revision, second.Revision)
	}
	var highestHistorical int64
	if err := pool.QueryRow(ctx, "SELECT coalesce(max(revision), 0) FROM project_logos WHERE project_id=$1 AND status <> 'active'", publishedID).Scan(&highestHistorical); err != nil {
		t.Fatalf("read historical revisions: %v", err)
	}
	if readded.Revision <= highestHistorical {
		t.Fatalf("re-added revision %d did not exceed historical maximum %d", readded.Revision, highestHistorical)
	}

	// Every earlier public version stays stale, and none of those stale
	// lookups may open stored content.
	opensBefore := counting.opens
	for version := int64(1); version <= highestHistorical; version++ {
		if _, err := service.OpenActive(ctx, publishedID, version); !errors.Is(err, ErrNotFound) {
			t.Fatalf("stale version %d returned %v, expected not found", version, err)
		}
	}
	if counting.opens != opensBefore {
		t.Fatalf("stale version lookups opened storage %d times", counting.opens-opensBefore)
	}

	// The current version serves, and the administrative preview serves the
	// same active revision.
	current, err := service.OpenActive(ctx, publishedID, readded.Revision)
	if err != nil {
		t.Fatalf("current version open returned %v", err)
	}
	_ = current.File.Close()
	administrative, err := service.OpenActiveForAdministration(ctx, publishedID)
	if err != nil {
		t.Fatalf("administrative open returned %v", err)
	}
	_ = administrative.File.Close()
	if _, err := service.OpenActiveForAdministration(ctx, draftID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("draft Project without a logo returned %v, expected not found", err)
	}

	// A persisted draft logo previews through the administrative path while
	// the public path keeps rejecting the draft Project.
	draftBytes := pngBytes(t, 6, 6)
	draftLogo, err := service.Upload(ctx, actorID, draftID, 1, UploadInput{ExpectedSize: int64(len(draftBytes)), Content: bytes.NewReader(draftBytes)})
	if err != nil {
		t.Fatalf("draft logo upload returned an error: %v", err)
	}
	if _, err := service.OpenActive(ctx, draftID, draftLogo.Revision); !errors.Is(err, ErrNotFound) {
		t.Fatalf("public open of a draft logo returned %v", err)
	}
	draftPreview, err := service.OpenActiveForAdministration(ctx, draftID)
	if err != nil {
		t.Fatalf("administrative draft preview returned %v", err)
	}
	previewBytes, _ := io.ReadAll(draftPreview.File)
	_ = draftPreview.File.Close()
	if !bytes.Equal(previewBytes, draftBytes) {
		t.Fatal("administrative draft preview served different bytes")
	}
}
