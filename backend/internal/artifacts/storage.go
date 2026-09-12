package artifacts

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

const (
	BackendLocal = "local"
	BackendB2    = "b2"
)

var (
	ErrEmptyContent       = errors.New("artifact content is empty")
	ErrContentTooLarge    = errors.New("artifact content exceeds the configured limit")
	ErrSizeMismatch       = errors.New("artifact content size does not match the declared size")
	ErrUnsafeStoragePath  = errors.New("artifact storage path is unsafe")
	ErrStorageUnavailable = errors.New("artifact storage backend is unavailable")
	ErrUnknownStorageName = errors.New("artifact storage backend name is not configured")
)

// Backend stores Artifact bytes under opaque storage keys. Implementations
// must be safe for concurrent use. Put generates a fresh key; PutAt writes
// under an explicit key, which migration uses to keep a record's key stable
// across backends. CheckReady reports whether the provider is sufficiently
// reachable for the readiness boundary: it must honor cancellation and
// deadlines, return an error matching ErrStorageUnavailable on any failure,
// and never leave persisted content behind.
type Backend interface {
	Name() string
	Put(ctx context.Context, reader io.Reader, expectedSize int64) (StoredContent, error)
	PutAt(ctx context.Context, storageKey string, reader io.Reader, expectedSize int64) (StoredContent, error)
	Open(ctx context.Context, storageKey string) (io.ReadSeekCloser, int64, error)
	Exists(ctx context.Context, storageKey string) bool
	RemoveNew(ctx context.Context, storageKey string) error
	CheckReady(ctx context.Context) error
}

// StorageOptions carries every configured backend's construction values.
// LocalStorage is always registered; B2 is registered whenever its values
// are present so records stamped for it remain servable regardless of the
// configured default.
type StorageOptions struct {
	DefaultName      string
	LocalRoot        string
	MaxArtifactBytes int64
	B2Endpoint       string
	B2Bucket         string
	B2KeyID          string
	B2ApplicationKey string
}

// StorageSet routes Artifact byte operations by backend name. Writes always
// target the configured default backend; reads consult the per-Artifact
// storage_backend marker so migration and rollback need no rewrites.
type StorageSet struct {
	DefaultName string
	Backends    map[string]Backend
}

func NewStorageSet(options StorageOptions) (StorageSet, error) {
	defaultName := options.DefaultName
	if defaultName == "" {
		defaultName = BackendLocal
	}
	if defaultName != BackendLocal && defaultName != BackendB2 {
		return StorageSet{}, fmt.Errorf("unknown artifact storage backend %q", defaultName)
	}
	set := StorageSet{DefaultName: defaultName, Backends: map[string]Backend{}}
	set.Backends[BackendLocal] = LocalStorage{Root: options.LocalRoot, MaxBytes: options.MaxArtifactBytes}
	b2Configured := options.B2Endpoint != "" || options.B2Bucket != "" || options.B2KeyID != "" || options.B2ApplicationKey != ""
	if defaultName == BackendB2 || b2Configured {
		b2, err := NewB2Storage(options.B2Endpoint, options.B2Bucket, options.B2KeyID, options.B2ApplicationKey, options.MaxArtifactBytes)
		if err != nil {
			return StorageSet{}, err
		}
		set.Backends[BackendB2] = b2
	}
	if _, ok := set.Backends[defaultName]; !ok {
		return StorageSet{}, fmt.Errorf("artifact storage backend %q has no configuration", defaultName)
	}
	return set, nil
}

func (set StorageSet) Default() Backend {
	return set.Backends[set.DefaultName]
}

// CheckReady probes only the configured default provider. A secondary
// provider registered for migration or historical reads must not make the
// application unready while it is not the active write provider.
func (set StorageSet) CheckReady(ctx context.Context) error {
	provider, err := set.Lookup(set.DefaultName)
	if err != nil {
		return fmt.Errorf("%w: readiness default provider: %v", ErrStorageUnavailable, err)
	}
	return provider.CheckReady(ctx)
}

func (set StorageSet) Lookup(name string) (Backend, error) {
	backend, ok := set.Backends[name]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnknownStorageName, name)
	}
	return backend, nil
}

type StoredContent struct {
	StorageKey   string
	ByteCount    int64
	SHA256       [sha256.Size]byte
	DetectedMIME string
	Prefix       []byte
}

// LocalStorage stores Artifact bytes on the local filesystem under an
// absolute root. It is the canonical LocalStorage backend of AD-008.
type LocalStorage struct {
	Root     string
	MaxBytes int64
}

func (storage LocalStorage) Name() string { return BackendLocal }

func (storage LocalStorage) Put(ctx context.Context, reader io.Reader, expectedSize int64) (StoredContent, error) {
	contentID := uuid.NewString()
	storageKey := filepath.ToSlash(filepath.Join("v1", contentID[0:2], contentID[2:4], contentID))
	return storage.PutAt(ctx, storageKey, reader, expectedSize)
}

func (storage LocalStorage) PutAt(ctx context.Context, storageKey string, reader io.Reader, expectedSize int64) (StoredContent, error) {
	if expectedSize == 0 {
		return StoredContent{}, ErrEmptyContent
	}
	if storage.MaxBytes <= 0 {
		return StoredContent{}, errors.New("artifact storage limit must be positive")
	}
	if expectedSize > storage.MaxBytes {
		return StoredContent{}, ErrContentTooLarge
	}
	if err := storage.ensureRoot(); err != nil {
		return StoredContent{}, err
	}

	finalPath, err := storage.resolve(storageKey, false)
	if err != nil {
		return StoredContent{}, err
	}
	if err := os.MkdirAll(filepath.Dir(finalPath), 0o750); err != nil {
		return StoredContent{}, fmt.Errorf("create artifact directory: %w", err)
	}
	if err := storage.rejectSymlinks(filepath.Dir(finalPath)); err != nil {
		return StoredContent{}, err
	}

	temporaryDirectory := filepath.Join(storage.Root, ".tmp")
	temporaryFile, err := os.CreateTemp(temporaryDirectory, "artifact-*.part")
	if err != nil {
		return StoredContent{}, fmt.Errorf("create artifact temporary file: %w", err)
	}
	temporaryPath := temporaryFile.Name()
	removeTemporary := true
	defer func() {
		_ = temporaryFile.Close()
		if removeTemporary {
			_ = os.Remove(temporaryPath)
		}
	}()

	hasher := sha256.New()
	prefix := &prefixWriter{limit: 512}
	limitedReader := io.LimitReader(&contextReader{ctx: ctx, reader: reader}, storage.MaxBytes+1)
	byteCount, err := io.Copy(io.MultiWriter(temporaryFile, hasher, prefix), limitedReader)
	if err != nil {
		return StoredContent{}, fmt.Errorf("store artifact content: %w", err)
	}
	if byteCount == 0 {
		return StoredContent{}, ErrEmptyContent
	}
	if byteCount > storage.MaxBytes {
		return StoredContent{}, ErrContentTooLarge
	}
	if expectedSize >= 0 && byteCount != expectedSize {
		return StoredContent{}, ErrSizeMismatch
	}
	if err := temporaryFile.Sync(); err != nil {
		return StoredContent{}, fmt.Errorf("sync artifact content: %w", err)
	}
	if err := temporaryFile.Close(); err != nil {
		return StoredContent{}, fmt.Errorf("close artifact content: %w", err)
	}
	if err := os.Rename(temporaryPath, finalPath); err != nil {
		return StoredContent{}, fmt.Errorf("finalize artifact content: %w", err)
	}
	removeTemporary = false

	var digest [sha256.Size]byte
	copy(digest[:], hasher.Sum(nil))
	return StoredContent{
		StorageKey:   storageKey,
		ByteCount:    byteCount,
		SHA256:       digest,
		DetectedMIME: http.DetectContentType(prefix.content),
		Prefix:       append([]byte(nil), prefix.content...),
	}, nil
}

func (storage LocalStorage) Open(ctx context.Context, storageKey string) (io.ReadSeekCloser, int64, error) {
	_ = ctx
	path, err := storage.resolve(storageKey, true)
	if err != nil {
		return nil, 0, err
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	information, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, 0, err
	}
	if !information.Mode().IsRegular() {
		file.Close()
		return nil, 0, ErrUnsafeStoragePath
	}
	return file, information.Size(), nil
}

func (storage LocalStorage) Exists(ctx context.Context, storageKey string) bool {
	file, _, err := storage.Open(ctx, storageKey)
	if err != nil {
		return false
	}
	file.Close()
	return true
}

func (storage LocalStorage) RemoveNew(ctx context.Context, storageKey string) error {
	_ = ctx
	path, err := storage.resolve(storageKey, true)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// CheckReady proves the local Artifact root exists as a usable directory by
// creating, closing, and removing one bounded empty probe file directly
// inside it. A root that disappeared after startup fails readiness; the
// check never recreates the root and never requires the write path's
// on-demand temporary directory.
func (storage LocalStorage) CheckReady(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("%w: local readiness canceled before probing: %v", ErrStorageUnavailable, err)
	}
	if !filepath.IsAbs(storage.Root) {
		return fmt.Errorf("%w: local readiness requires an absolute root", ErrStorageUnavailable)
	}
	// The same symlink validation as the write path: os.Stat would follow a
	// symlinked root, so readiness would probe a location uploads reject.
	if err := storage.rejectSymlinks(storage.Root); err != nil {
		return fmt.Errorf("%w: local readiness root: %v", ErrStorageUnavailable, err)
	}
	root, err := os.Stat(storage.Root)
	if err != nil {
		return fmt.Errorf("%w: local readiness root: %v", ErrStorageUnavailable, err)
	}
	if !root.IsDir() {
		return fmt.Errorf("%w: local readiness root is not a directory", ErrStorageUnavailable)
	}
	probe, err := os.CreateTemp(storage.Root, ".ready-probe-")
	if err != nil {
		return fmt.Errorf("%w: local readiness probe create: %v", ErrStorageUnavailable, err)
	}
	probeName := probe.Name()
	if err := probe.Close(); err != nil {
		_ = os.Remove(probeName)
		return fmt.Errorf("%w: local readiness probe close: %v", ErrStorageUnavailable, err)
	}
	if err := os.Remove(probeName); err != nil {
		return fmt.Errorf("%w: local readiness probe remove: %v", ErrStorageUnavailable, err)
	}
	return nil
}

func (storage LocalStorage) ensureRoot() error {
	if !filepath.IsAbs(storage.Root) {
		return ErrUnsafeStoragePath
	}
	if err := os.MkdirAll(storage.Root, 0o750); err != nil {
		return fmt.Errorf("create artifact root: %w", err)
	}
	if err := storage.rejectSymlinks(storage.Root); err != nil {
		return err
	}
	temporaryDirectory := filepath.Join(storage.Root, ".tmp")
	if err := os.MkdirAll(temporaryDirectory, 0o750); err != nil {
		return fmt.Errorf("create artifact temporary directory: %w", err)
	}
	return storage.rejectSymlinks(temporaryDirectory)
}

func (storage LocalStorage) resolve(storageKey string, requireExisting bool) (string, error) {
	if storageKey == "" || filepath.IsAbs(storageKey) || filepath.Clean(storageKey) != storageKey || strings.HasPrefix(storageKey, ".."+string(filepath.Separator)) {
		return "", ErrUnsafeStoragePath
	}
	root, err := filepath.Abs(storage.Root)
	if err != nil {
		return "", ErrUnsafeStoragePath
	}
	path := filepath.Join(root, storageKey)
	relative, err := filepath.Rel(root, path)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", ErrUnsafeStoragePath
	}
	checkPath := path
	if !requireExisting {
		checkPath = filepath.Dir(path)
	}
	if err := storage.rejectSymlinks(checkPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	return path, nil
}

func (storage LocalStorage) rejectSymlinks(path string) error {
	root, err := filepath.Abs(storage.Root)
	if err != nil {
		return ErrUnsafeStoragePath
	}
	relative, err := filepath.Rel(root, path)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return ErrUnsafeStoragePath
	}
	current := root
	paths := []string{root}
	if relative != "." {
		for _, component := range strings.Split(relative, string(filepath.Separator)) {
			current = filepath.Join(current, component)
			paths = append(paths, current)
		}
	}
	for _, candidate := range paths {
		information, statErr := os.Lstat(candidate)
		if statErr != nil {
			return statErr
		}
		if information.Mode()&os.ModeSymlink != 0 {
			return ErrUnsafeStoragePath
		}
	}
	return nil
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (reader *contextReader) Read(buffer []byte) (int, error) {
	select {
	case <-reader.ctx.Done():
		return 0, reader.ctx.Err()
	default:
		return reader.reader.Read(buffer)
	}
}

type prefixWriter struct {
	content []byte
	limit   int
}

func (writer *prefixWriter) Write(content []byte) (int, error) {
	remaining := writer.limit - len(writer.content)
	if remaining > 0 {
		if len(content) < remaining {
			remaining = len(content)
		}
		writer.content = append(writer.content, content[:remaining]...)
	}
	return len(content), nil
}
