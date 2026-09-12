package artifacts

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// renamedBackend serves a LocalStorage root under a different backend name,
// so marker routing is exercised without network access.
type renamedBackend struct {
	LocalStorage
	name string
}

func (backend renamedBackend) Name() string { return backend.name }

// memoryBackend is an in-memory Backend used as a migration target in tests.
type memoryBackend struct {
	name     string
	maxBytes int64
	objects  map[string][]byte
}

func newMemoryBackend(name string, maxBytes int64) *memoryBackend {
	return &memoryBackend{name: name, maxBytes: maxBytes, objects: map[string][]byte{}}
}

func (backend *memoryBackend) Name() string { return backend.name }

func (backend *memoryBackend) Put(ctx context.Context, reader io.Reader, expectedSize int64) (StoredContent, error) {
	key := "v1/" + uuid.NewString()[:4] + "/" + uuid.NewString()
	return backend.PutAt(ctx, key, reader, expectedSize)
}

func (backend *memoryBackend) PutAt(ctx context.Context, storageKey string, reader io.Reader, expectedSize int64) (StoredContent, error) {
	content, err := io.ReadAll(io.LimitReader(reader, backend.maxBytes+1))
	if err != nil {
		return StoredContent{}, err
	}
	if len(content) == 0 {
		return StoredContent{}, ErrEmptyContent
	}
	if int64(len(content)) > backend.maxBytes {
		return StoredContent{}, ErrContentTooLarge
	}
	if expectedSize >= 0 && int64(len(content)) != expectedSize {
		return StoredContent{}, ErrSizeMismatch
	}
	backend.objects[storageKey] = content
	return StoredContent{StorageKey: storageKey, ByteCount: int64(len(content)), SHA256: sha256.Sum256(content)}, nil
}

func (backend *memoryBackend) Open(ctx context.Context, storageKey string) (io.ReadSeekCloser, int64, error) {
	content, ok := backend.objects[storageKey]
	if !ok {
		return nil, 0, os.ErrNotExist
	}
	return nopClosingReader{bytes.NewReader(content)}, int64(len(content)), nil
}

func (backend *memoryBackend) Exists(ctx context.Context, storageKey string) bool {
	_, ok := backend.objects[storageKey]
	return ok
}

func (backend *memoryBackend) RemoveNew(ctx context.Context, storageKey string) error {
	delete(backend.objects, storageKey)
	return nil
}

func (backend *memoryBackend) CheckReady(ctx context.Context) error { return nil }

type nopClosingReader struct{ reader *bytes.Reader }

func (reader nopClosingReader) Read(buffer []byte) (int, error) { return reader.reader.Read(buffer) }
func (reader nopClosingReader) Seek(offset int64, whence int) (int64, error) {
	return reader.reader.Seek(offset, whence)
}
func (reader nopClosingReader) Close() error { return nil }

func TestArtifactStorageMarkerRoutesServing(t *testing.T) {
	databaseURL := os.Getenv("AUSE_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("AUSE_TEST_DATABASE_URL is required for PostgreSQL integration tests")
	}

	ctx := context.Background()
	pool := createArtifactTestDatabase(t, ctx, databaseURL)
	actorID, projectID := seedArtifactOwner(t, ctx, pool)
	storage := LocalStorage{Root: t.TempDir(), MaxBytes: 1024}
	set := StorageSet{DefaultName: BackendLocal, Backends: map[string]Backend{
		BackendLocal: storage,
		BackendB2:    renamedBackend{LocalStorage: storage, name: BackendB2},
	}}
	service := Service{Pool: pool, Storage: set, MaxProjectBytes: 4096}
	content := []byte("%PDF-1.7\nartifact")

	created, err := service.Upload(ctx, actorID, projectID, 1, UploadInput{
		ArtifactType: "report", DisplayName: "Report", OriginalFilename: "report.pdf", ExpectedSize: int64(len(content)), Content: bytes.NewReader(content),
	})
	if err != nil {
		t.Fatalf("Upload returned an error: %v", err)
	}
	if _, err := pool.Exec(ctx, "UPDATE projects SET status = 'published', published_at = now() WHERE id = $1", projectID); err != nil {
		t.Fatalf("publish Project fixture: %v", err)
	}

	if _, err := service.OpenPublic(ctx, created.ID); err != nil {
		t.Fatalf("public Open through local marker returned an error: %v", err)
	}

	if _, err := pool.Exec(ctx, "UPDATE artifacts SET storage_backend = 'b2' WHERE id = $1", created.ID); err != nil {
		t.Fatalf("stamp marker: %v", err)
	}
	publicContent, err := service.OpenPublic(ctx, created.ID)
	if err != nil {
		t.Fatalf("public Open through b2 marker returned an error: %v", err)
	}
	opened, err := io.ReadAll(publicContent.File)
	_ = publicContent.File.Close()
	if err != nil || !bytes.Equal(opened, content) {
		t.Fatalf("public Open through b2 marker returned %q with error %v", opened, err)
	}

	// A b2-stamped record served by a deployment without any B2 backend
	// configured must degrade to the unavailable response, not error oddly.
	localOnlyService := Service{Pool: pool, Storage: newLocalStorageSet(t, storage), MaxProjectBytes: 4096}
	if _, err := localOnlyService.OpenPublic(ctx, created.ID); !errors.Is(err, ErrContentUnavailable) {
		t.Fatalf("public Open without the marked backend returned %v, expected content unavailable", err)
	}

	if _, err := pool.Exec(ctx, "UPDATE artifacts SET storage_backend = 's3' WHERE id = $1", created.ID); err == nil {
		t.Fatal("schema accepted a storage backend outside its allowlist")
	}
}

func TestMigrateStorageCopiesVerifiesAndStamps(t *testing.T) {
	databaseURL := os.Getenv("AUSE_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("AUSE_TEST_DATABASE_URL is required for PostgreSQL integration tests")
	}

	ctx := context.Background()
	pool := createArtifactTestDatabase(t, ctx, databaseURL)
	actorID, projectID := seedArtifactOwner(t, ctx, pool)
	storage := LocalStorage{Root: t.TempDir(), MaxBytes: 1024}
	localSet := newLocalStorageSet(t, storage)
	service := Service{Pool: pool, Storage: localSet, MaxProjectBytes: 4096}
	first := []byte("%PDF-1.7\nfirst report")
	second := []byte("%PDF-1.7\nsecond report")
	firstArtifact := uploadArtifactFixture(t, ctx, pool, service, actorID, projectID, "first.pdf", first)
	secondArtifact := uploadArtifactFixture(t, ctx, pool, service, actorID, projectID, "second.pdf", second)
	firstKey := artifactStorageKey(t, ctx, pool, firstArtifact)
	secondKey := artifactStorageKey(t, ctx, pool, secondArtifact)

	target := newMemoryBackend(BackendB2, 1024)
	migrationSet := StorageSet{DefaultName: BackendB2, Backends: map[string]Backend{BackendLocal: storage, BackendB2: target}}

	plan, err := MigrateStorage(ctx, pool, migrationSet, false)
	if err != nil {
		t.Fatalf("dry-run returned an error: %v", err)
	}
	if plan.Planned != 2 || plan.Copied != 0 || plan.OnSourceBackend != 2 {
		t.Fatalf("dry-run reported %#v, expected two planned rows and no copies", plan)
	}

	result, err := MigrateStorage(ctx, pool, migrationSet, true)
	if err != nil {
		t.Fatalf("apply returned an error: %v", err)
	}
	if result.Copied != 2 || result.Verified != 2 || len(result.Failed) != 0 || len(target.objects) != 2 {
		t.Fatalf("apply reported %#v with %d target objects", result, len(target.objects))
	}
	if !bytes.Equal(target.objects[firstKey], first) || !bytes.Equal(target.objects[secondKey], second) {
		t.Fatal("target backend holds bytes under unexpected keys or contents")
	}
	for _, artifactID := range []string{firstArtifact, secondArtifact} {
		var backend string
		if err := pool.QueryRow(ctx, "SELECT storage_backend FROM artifacts WHERE id = $1", artifactID).Scan(&backend); err != nil || backend != BackendB2 {
			t.Fatalf("artifact %s marker was %q with error %v", artifactID, backend, err)
		}
	}

	again, err := MigrateStorage(ctx, pool, migrationSet, true)
	if err != nil {
		t.Fatalf("second apply returned an error: %v", err)
	}
	if again.Planned != 0 || again.Copied != 0 || again.OnTargetBackend != 2 {
		t.Fatalf("second apply reported %#v, expected nothing left to migrate", again)
	}
}

func TestMigrateStorageAbortsOnDigestMismatchWithoutStamping(t *testing.T) {
	databaseURL := os.Getenv("AUSE_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("AUSE_TEST_DATABASE_URL is required for PostgreSQL integration tests")
	}

	ctx := context.Background()
	pool := createArtifactTestDatabase(t, ctx, databaseURL)
	actorID, projectID := seedArtifactOwner(t, ctx, pool)
	storage := LocalStorage{Root: t.TempDir(), MaxBytes: 1024}
	service := Service{Pool: pool, Storage: newLocalStorageSet(t, storage), MaxProjectBytes: 4096}
	content := []byte("%PDF-1.7\nreport")
	artifactID := uploadArtifactFixture(t, ctx, pool, service, actorID, projectID, "report.pdf", content)

	if _, err := pool.Exec(ctx, "UPDATE artifacts SET sha256 = $2 WHERE id = $1", artifactID, make([]byte, 32)); err != nil {
		t.Fatalf("corrupt stored digest: %v", err)
	}

	target := newMemoryBackend(BackendB2, 1024)
	migrationSet := StorageSet{DefaultName: BackendB2, Backends: map[string]Backend{BackendLocal: storage, BackendB2: target}}
	_, err := MigrateStorage(ctx, pool, migrationSet, true)
	if err == nil || !strings.Contains(err.Error(), "digest mismatch") {
		t.Fatalf("apply returned %v, expected a digest mismatch error", err)
	}
	var backend string
	if err := pool.QueryRow(ctx, "SELECT storage_backend FROM artifacts WHERE id = $1", artifactID).Scan(&backend); err != nil || backend != BackendLocal {
		t.Fatalf("artifact marker was %q with error %v, expected it to stay local", backend, err)
	}
}

func TestMigrateStorageRejectsLocalTarget(t *testing.T) {
	databaseURL := os.Getenv("AUSE_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("AUSE_TEST_DATABASE_URL is required for PostgreSQL integration tests")
	}

	ctx := context.Background()
	pool := createArtifactTestDatabase(t, ctx, databaseURL)
	storage := LocalStorage{Root: t.TempDir(), MaxBytes: 1024}
	set := newLocalStorageSet(t, storage)
	if _, err := MigrateStorage(ctx, pool, set, true); err == nil {
		t.Fatal("migration to a local target was accepted")
	}
}

func uploadArtifactFixture(t *testing.T, ctx context.Context, pool *pgxpool.Pool, service Service, actorID, projectID uuid.UUID, filename string, content []byte) string {
	t.Helper()
	var projectRevision int64
	if err := pool.QueryRow(ctx, "SELECT revision FROM projects WHERE id = $1", projectID).Scan(&projectRevision); err != nil {
		t.Fatalf("read project revision for fixture %s: %v", filename, err)
	}
	created, err := service.Upload(ctx, actorID, projectID, projectRevision, UploadInput{
		ArtifactType: "report", DisplayName: "Report " + filename, OriginalFilename: filename, ExpectedSize: int64(len(content)), Content: bytes.NewReader(content),
	})
	if err != nil {
		t.Fatalf("fixture Upload of %s returned an error: %v", filename, err)
	}
	return created.ID.String()
}

func artifactStorageKey(t *testing.T, ctx context.Context, pool *pgxpool.Pool, artifactID string) string {
	t.Helper()
	var storageKey string
	if err := pool.QueryRow(ctx, "SELECT storage_key FROM artifacts WHERE id = $1", artifactID).Scan(&storageKey); err != nil {
		t.Fatalf("read storage key for artifact %s: %v", artifactID, err)
	}
	return storageKey
}
